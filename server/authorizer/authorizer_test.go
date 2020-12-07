// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package authorizer

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"net/http/httptest"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/server/identifier"
	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

type mockHandler struct {
}

func (h *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

func TestBasic(t *testing.T) {

	// Policy for testing access to policies.
	compiler := func() *ast.Compiler {
		module := `
        package system.authz

        import data.system.tokens

        allow = allow_inner {
            not input.path[0] = "undefined" # testing undefined
            not conflict_error              # testing eval errors
        }

        conflict_error {
			input.path[0] = "conflict_error"
			{k: v | k = ["a", "a"][_]; [1, 2][v]}
        }

        default allow_inner = false

        allow_inner {
            valid_method
            valid_path
        }

        valid_method {
            rights[_].access[_] = access_map[input.method]
        }

        valid_path {
            rights[_].path = "*"
        }

        valid_path {
            rights[_].path = input.path
        }

        rights[right] {
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
		c := ast.NewCompiler()
		c.Compile(map[string]*ast.Module{
			"test.rego": ast.MustParseModule(module),
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

	store := inmem.NewFromObject(data.(map[string]interface{}))

	tests := []struct {
		note           string
		identity       string
		method         string
		path           string
		expectedStatus int
		expectedCode   string
		expectedMsg    string
	}{
		{"root (ok)", "token0", http.MethodGet, "", http.StatusOK, "", ""},
		{"index.html (ok)", "token0", http.MethodGet, "/index.html", http.StatusOK, "", ""},
		{"undefined", "token0", http.MethodGet, "/undefined", http.StatusInternalServerError, types.CodeInternal, types.MsgUnauthorizedUndefinedError},
		{"evaluation error", "token0", http.MethodGet, "/conflict_error", http.StatusInternalServerError, types.CodeInternal, types.MsgEvaluationError},
		{"ok", "token1", http.MethodGet, "/data/some/specific/document", http.StatusOK, "", ""},
		{"ok (w/ query params)", "token1", http.MethodGet, "/data/some/specific/document?pretty=true", http.StatusOK, "", ""},
		{"unauthorized method", "token1", http.MethodPut, "/data/some/specific/document", http.StatusUnauthorized, types.CodeUnauthorized, types.MsgUnauthorizedError},
		{"unauthorized path", "token2", http.MethodGet, "/data/some/doc/not/allowed", http.StatusUnauthorized, types.CodeUnauthorized, types.MsgUnauthorizedError},
		{"unauthorized path (w/ query params)", "token2", http.MethodGet, "/data/some/doc/not/allowed?pretty=true", http.StatusUnauthorized, types.CodeUnauthorized, types.MsgUnauthorizedError},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {

			recorder := httptest.NewRecorder()
			req, err := http.NewRequest(tc.method, "http://localhost:8181"+tc.path, nil)
			if err != nil {
				t.Fatalf("Unexpected error creating request for %v: %v", tc, err)
			}

			if len(tc.identity) > 0 {
				req = identifier.SetIdentity(req, tc.identity)
			}

			NewBasic(&mockHandler{}, compiler, store, Decision(func() ast.Ref {
				return ast.MustParseRef("data.system.authz.allow")
			})).ServeHTTP(recorder, req)

			if recorder.Code != tc.expectedStatus {
				t.Fatalf("Expected status code %v but got: %v", tc.expectedStatus, recorder)
			}

			// Check code/message if response should be error.
			if tc.expectedStatus != http.StatusOK {
				var x interface{}
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
		panic(err)
	}

	req.URL.Path = `/invalid/path/foo%LALALA`

	compiler := func() *ast.Compiler {
		return ast.NewCompiler()
	}

	store := inmem.New()

	NewBasic(&mockHandler{}, compiler, store).ServeHTTP(recorder, req)

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
		panic(err)
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

	req, result, err := makeInput(req)
	if err != nil {
		panic(err)
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

	if !reflect.DeepEqual(util.MustMarshalJSON(expectedResult), util.MustMarshalJSON(result)) {
		t.Fatalf("Expected %+v but got %+v", expectedResult, result)
	}

}

func TestMakeInputWithBody(t *testing.T) {

	reqs := []struct {
		method                 string
		path                   string
		headers                map[string]string
		body                   string
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

			req, input, err := makeInput(req)
			if err != nil {
				t.Fatal(err)
			}

			if tc.assertBodyExists {

				var want interface{}

				if tc.useYAML {
					if err := util.Unmarshal([]byte(tc.body), &want); err != nil {
						t.Fatal(err)
					}
				} else {
					want = util.MustUnmarshalJSON([]byte(tc.body))
				}

				body := input.(map[string]interface{})["body"]

				if !reflect.DeepEqual(body, want) {
					t.Fatalf("expected parsed bodies to be equal but got %v and want %v", body, want)
				}

				body, ok := GetBodyOnContext(req.Context())
				if !ok || !reflect.DeepEqual(body, want) {
					t.Fatalf("expected parsed body to be cached on context but got %v and want %v", body, want)
				}
			}

			if tc.assertBodyDoesNotExist {
				_, ok := input.(map[string]interface{})["body"]
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
