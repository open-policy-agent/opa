// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package authorizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/server/identifier"
	"github.com/open-policy-agent/opa/v1/server/types"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
	"github.com/open-policy-agent/opa/v1/topdown/print"
	"github.com/open-policy-agent/opa/v1/util"
)

type mockHandler struct {
}

type appendingPrintHook struct {
	printed *[]string
}

func (a appendingPrintHook) Print(_ print.Context, s string) error {
	*a.printed = append(*a.printed, s)
	return nil
}

func (*mockHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(200)
}

func TestBasic(t *testing.T) {
	// Policy for testing access to policies.
	compiler := func() *ast.Compiler {
		module := `
        package system.authz

        import data.system.tokens

        allow = resp if {
            not undefined_case
        }

        resp["allowed"] = allowed if {
            not undefined_case
            not wrong_object
        }

        resp["reason"] = "custom reason" if {
            input.path = ["reason"]
        }

        resp["reason"] = 0 if {
            input.path = ["reason", "wrong_type"]
        }

        resp["foo"] = "bar" if {
            wrong_object
        }

        default allowed = false

        allowed = allow_inner if {
            not undefined_case            # undefined
            not wrong_object              # object response, wrong key
            not input.path[0] = "reason"  # custom reason
            not conflict_error            # eval errors
			print("ok")
        }

        undefined_case if {
            input.path[0] = "undefined"
        }

        wrong_object if {
            input.path = ["reason", "wrong_object"]
        }

        conflict_error if {
            input.path[0] = "conflict_error"
            {k: v | k = ["a", "a"][_]; [1, 2][v]}
        }

        default allow_inner = false

        allow_inner if {
            valid_method
            valid_path
        }

        valid_method if {
            rights[_].access[_] = access_map[input.method]
        }

        valid_path if {
            rights[_].path = "*"
        }

        valid_path if {
            rights[_].path = input.path
        }

        rights contains right if {
            role = tokens[input.identity].roles[_]
            right = all_rights[role][_]
        }

        all_rights = {
            "admin": [{
                "path": "*",
                "access": ["read", "write"],
            }],
            "service_read_only_path": [
                {
                    "path": ["data", "some", "specific", "document"],
                    "access": ["read"],
                },
            ],
            "service_read_write_path": [
                {
                    "path": ["data", "some", "other", "document"],
                    "access": ["read", "write"],
                },
            ],
        }

        access_map = {
            "GET": "read",
            "HEAD": "read",
            "PATCH": "write",
            "POST": "write",
            "PUT": "write",
            "DELETE": "write",
        }
        `
		c := ast.NewCompiler().WithEnablePrintStatements(true)
		c.Compile(map[string]*ast.Module{
			"test.rego": ast.MustParseModuleWithOpts(module, ast.ParserOptions{AllFutureKeywords: true}),
		})
		if c.Failed() {
			t.Fatalf("Unexpected error compiling test module: %v", c.Errors)
		}
		return c
	}

	// Data used for testing authorizer access to storage.
	data := util.MustUnmarshalJSON([]byte(`
    {
        "system": {
            "tokens": {
				"token0": {
					"roles": ["admin"]
				},
				"token1": {
					"roles": ["service_read_only_path"]
				},
				"token2": {
					"roles": ["service_read_write_path"]
				}
            }
        }
    }
    `))

	store := inmem.NewFromObject(data.(map[string]any))

	tests := []struct {
		note           string
		identity       string
		method         string
		path           string
		expectedStatus int
		expectedCode   string
		expectedMsg    string
		expectedPrint  []string
	}{
		{"root (ok)", "token0", http.MethodGet, "", http.StatusOK, "", "", []string{"ok"}},
		{"index.html (ok)", "token0", http.MethodGet, "/index.html", http.StatusOK, "", "", []string{"ok"}},
		{"undefined", "token0", http.MethodGet, "/undefined", http.StatusInternalServerError, types.CodeInternal, types.MsgUnauthorizedUndefinedError, []string{}},
		{"evaluation error", "token0", http.MethodGet, "/conflict_error", http.StatusInternalServerError, types.CodeInternal, types.MsgEvaluationError, []string{}},
		{"ok", "token1", http.MethodGet, "/data/some/specific/document", http.StatusOK, "", "", []string{"ok"}},
		{"ok (w/ query params)", "token1", http.MethodGet, "/data/some/specific/document?pretty=true", http.StatusOK, "", "", []string{"ok"}},
		{"unauthorized method", "token1", http.MethodPut, "/data/some/specific/document", http.StatusUnauthorized, types.CodeUnauthorized, types.MsgUnauthorizedError, []string{"ok"}},
		{"unauthorized path", "token2", http.MethodGet, "/data/some/doc/not/allowed", http.StatusUnauthorized, types.CodeUnauthorized, types.MsgUnauthorizedError, []string{"ok"}},
		{"unauthorized path (w/ query params)", "token2", http.MethodGet, "/data/some/doc/not/allowed?pretty=true", http.StatusUnauthorized, types.CodeUnauthorized, types.MsgUnauthorizedError, []string{"ok"}},
		{"custom reason", "token2", http.MethodGet, "/reason", http.StatusUnauthorized, types.CodeUnauthorized, "custom reason", []string{}},
		{"custom reason, wrong type", "token2", http.MethodGet, "/reason/wrong_type", http.StatusUnauthorized, types.CodeUnauthorized, types.MsgUnauthorizedError, []string{}},
		{"non-bool/obj response", "token2", http.MethodGet, "/reason/wrong_object", http.StatusInternalServerError, types.CodeInternal, types.MsgUndefinedError, []string{}},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			recorder := httptest.NewRecorder()
			req, err := http.NewRequest(tc.method, "http://localhost:8181"+tc.path, nil)
			if err != nil {
				t.Fatalf("Unexpected error creating request for %v: %v", tc, err)
			}

			if len(tc.identity) > 0 {
				req = identifier.SetIdentity(req, tc.identity)
			}

			var output []string
			NewBasic(
				&mockHandler{},
				compiler,
				store,
				EnablePrintStatements(true),
				PrintHook(appendingPrintHook{printed: &output}),
				Decision(func() ast.Ref {
					return ast.MustParseRef("data.system.authz.allow")
				}),
			).ServeHTTP(recorder, req)

			if recorder.Code != tc.expectedStatus {
				t.Fatalf("Expected status code %v but got: %v", tc.expectedStatus, recorder)
			}

			if !slices.Equal(tc.expectedPrint, output) {
				t.Errorf("Expected output %v, got %v", tc.expectedPrint, output)
			}

			// Check code/message if response should be error.
			if tc.expectedStatus != http.StatusOK {
				var x any
				if err := util.NewJSONDecoder(recorder.Body).Decode(&x); err != nil {
					t.Fatalf("Expected JSON response but got: %v", recorder)
				}
				response := ast.MustInterfaceToValue(x)
				code, err := response.Find(ast.RefTerm(ast.StringTerm("code")).Value.(ast.Ref))
				if err != nil {
					t.Fatalf("Missing code in response: %v", recorder)
				} else if code.Compare(ast.String(tc.expectedCode)) != 0 {
					t.Fatalf("Expected code %v but got: %v", tc.expectedCode, recorder)
				}

				msg, err := response.Find(ast.RefTerm(ast.StringTerm("message")).Value.(ast.Ref))
				if err != nil {
					t.Fatalf("Missing message in response: %v", recorder)
				} else if !strings.Contains(msg.String(), tc.expectedMsg) {
					t.Fatalf("Expected msg to contain %v but got: %v", tc.expectedMsg, response)
				}
			}
		})
	}
}

func TestBasicEscapeError(t *testing.T) {

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8181", nil)
	if err != nil {
		t.Fatal(err)
	}

	req.URL.Path = `/invalid/path/foo%LALALA`

	store := inmem.New()

	NewBasic(&mockHandler{}, ast.NewCompiler, store).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("Expected bad request but got: %v", recorder)
	}

	var response types.ErrorV1

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Expected error response but got: %v", recorder)
	}

	if response.Code != types.CodeInvalidParameter ||
		!strings.Contains(response.Message, "invalid URL") {
		t.Fatalf("Expected invalid parameter and URL parse error but got: %v", recorder)
	}
}

func TestMakeInput(t *testing.T) {
	path := "/foo/bar?pretty=true&explain=\"full\""
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8181"+path, nil)
	if err != nil {
		t.Fatal(err)
	}

	req.Header.Add("x-custom", "foo")
	req.Header.Add("X-custom", "bar")
	req.Header.Add("x-custom-2", "baz")
	req.Header.Add("custom-header-3?", "wat")

	query := req.URL.Query()

	// set query parameters
	query.Set("pretty", "true")
	query.Set("explain", "full")
	req.URL.RawQuery = query.Encode()

	req = identifier.SetIdentity(req, "bob")

	_, result, err := makeInput(req, nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedResult := util.MustUnmarshalJSON([]byte(`
		{
		  "path": ["foo","bar"],
		  "method": "GET",
		  "identity": "bob",
		  "headers": {
			"X-Custom": ["foo", "bar"],
			"X-Custom-2": ["baz"],
			"custom-header-3?": ["wat"]
		  },
		  "params": {"explain": ["full"], "pretty": ["true"]}
		}
	`))

	if !bytes.Equal(util.MustMarshalJSON(expectedResult), util.MustMarshalJSON(result)) {
		t.Fatalf("Expected %+v but got %+v", expectedResult, result)
	}
}

func TestMakeInputWithBody(t *testing.T) {
	reqs := []struct {
		method                 string
		path                   string
		headers                map[string]string
		body                   string
		extraPaths             []func(string, []any) bool
		useYAML                bool
		assertBodyExists       bool
		assertBodyDoesNotExist bool
	}{
		{
			method:           "POST",
			path:             "/",
			body:             `{"foo": "bar"}`,
			assertBodyExists: true,
		},
		{
			method:           "POST",
			path:             "/",
			body:             `foo: bar`,
			useYAML:          true,
			assertBodyExists: true,
		},
		{
			method:           "POST",
			path:             "/v0/data",
			body:             `{"foo": "bar"}`,
			assertBodyExists: true,
		},
		{
			method:           "POST",
			path:             "/v1/data",
			body:             `{"foo": "bar"}`,
			assertBodyExists: true,
		},
		{
			method:                 "PUT",
			path:                   "/v1/data",
			body:                   `{"foo": "bar"}`,
			assertBodyDoesNotExist: true,
		},
		{
			method:                 "PATCH",
			path:                   "/v1/data",
			body:                   `{"foo": "bar"}`,
			assertBodyDoesNotExist: true,
		},
		{
			method:                 "GET",
			path:                   "/v1/data",
			assertBodyDoesNotExist: true,
		},
		{
			method:                 "PUT",
			path:                   "/v1/policies/test",
			body:                   "package test\np = 7",
			assertBodyDoesNotExist: true,
		},
		{
			method: "PUT",
			path:   "/v1/example-plugin",
			body:   `{"example": "body must still be yaml or json"}`,
			extraPaths: []func(string, []any) bool{func(method string, path []any) bool {
				s1 := path[0].(string)
				s2 := path[1].(string)
				return dataAPIVersions[s1] && s2 == "example-plugin"
			}},
		},
	}

	for _, tc := range reqs {

		t.Run(tc.method+"_"+tc.path, func(t *testing.T) {

			req, err := http.NewRequest(tc.method, "http://localhost:8181"+tc.path, bytes.NewBufferString(tc.body))
			if err != nil {
				t.Fatal(err)
			}

			if tc.useYAML {
				req.Header.Set("Content-Type", "application/x-yaml")
			}

			req, input, err := makeInput(req, tc.extraPaths)
			if err != nil {
				t.Fatal(err)
			}

			if tc.assertBodyExists {

				var want any

				if tc.useYAML {
					if err := util.Unmarshal([]byte(tc.body), &want); err != nil {
						t.Fatal(err)
					}
				} else {
					want = util.MustUnmarshalJSON([]byte(tc.body))
				}

				body := input.(map[string]any)["body"]

				if !reflect.DeepEqual(body, want) {
					t.Fatalf("expected parsed bodies to be equal but got %v and want %v", body, want)
				}

				body, ok := GetBodyOnContext(req.Context())
				if !ok || !reflect.DeepEqual(body, want) {
					t.Fatalf("expected parsed body to be cached on context but got %v and want %v", body, want)
				}
			}

			if tc.assertBodyDoesNotExist {
				_, ok := input.(map[string]any)["body"]
				if ok {
					t.Fatal("expected no parsed body in input")
				}
				_, ok = GetBodyOnContext(req.Context())
				if ok {
					t.Fatal("expected no parsed body to be cached on context")
				}
			}

		})

	}

}

func TestInterQueryCache(t *testing.T) {

	count := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		count++
	}))

	t.Cleanup(func() {
		ts.Close()
	})

	compiler := func() *ast.Compiler {
		module := fmt.Sprintf(`
        package system.authz
        import rego.v1

        allow if {
            http.send({
                "method": "GET",
                "url": "%v",
                "force_cache": true,
                "force_cache_duration_seconds": 60
            }).status_code == 200
        }
        `, ts.URL)
		c := ast.NewCompiler()
		c.Compile(map[string]*ast.Module{
			"test.rego": ast.MustParseModule(module),
		})
		if c.Failed() {
			t.Fatalf("Unexpected error compiling test module: %v", c.Errors)
		}
		return c
	}

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8181/v1/data", nil)
	if err != nil {
		t.Fatal(err)
	}

	config, _ := cache.ParseCachingConfig(nil)
	interQueryCache := cache.NewInterQueryCache(config)

	basic := NewBasic(&mockHandler{}, compiler, inmem.New(), InterQueryCache(interQueryCache), Decision(func() ast.Ref {
		return ast.MustParseRef("data.system.authz.allow")
	}))

	// Execute the policy twice
	basic.ServeHTTP(recorder, req)
	basic.ServeHTTP(recorder, req)

	// And make sure the test server was only hit once
	if count != 1 {
		t.Error("Expected http.send response to be cached")
	}
}

func TestInterQueryValueCache(t *testing.T) {
	compiler := func() *ast.Compiler {
		module := `
        package system.authz
        import rego.v1

		allow if {
			regex.match("foo.*", "foobar")
		}`
		c := ast.NewCompiler()
		c.Compile(map[string]*ast.Module{
			"test.rego": ast.MustParseModule(module),
		})
		if c.Failed() {
			t.Fatalf("Unexpected error compiling test module: %v", c.Errors)
		}
		return c
	}

	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "http://localhost:8181/v1/data", nil)
	if err != nil {
		t.Fatal(err)
	}

	config, _ := cache.ParseCachingConfig(nil)
	interQueryValueCache := cache.NewInterQueryValueCache(t.Context(), config)

	basic := NewBasic(&mockHandler{}, compiler, inmem.New(), InterQueryValueCache(interQueryValueCache), Decision(func() ast.Ref {
		return ast.MustParseRef("data.system.authz.allow")
	}))

	// Execute the policy
	basic.ServeHTTP(recorder, req)
}
