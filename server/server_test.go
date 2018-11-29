// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/server/identifier"
	"github.com/open-policy-agent/opa/server/types"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
	"github.com/pkg/errors"
)

type tr struct {
	method string
	path   string
	body   string
	code   int
	resp   string
}

type trw struct {
	tr   tr
	wait chan struct{}
}

func TestDataV0(t *testing.T) {
	testMod1 := `package test

	p = "hello"

	q = {
		"foo": [1,2,3,4]
	} {
		input.flag = true
	}
	`

	f := newFixture(t)

	if err := f.v1(http.MethodPut, "/policies/test", testMod1, 200, ""); err != nil {
		t.Fatalf("Unexpected error while creating policy: %v", err)
	}

	if err := f.v0(http.MethodPost, "/data/test/p", "", 200, `"hello"`); err != nil {
		t.Fatalf("Expected response hello but got: %v", err)
	}

	if err := f.v0(http.MethodPost, "/data/test/q/foo", `{"flag": true}`, 200, `[1,2,3,4]`); err != nil {
		t.Fatalf("Expected response [1,2,3,4] but got: %v", err)
	}

	req := newReqV0(http.MethodPost, "/data/test/q", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 404 {
		t.Fatalf("Expected HTTP 404 but got: %v", f.recorder)
	}

	var resp types.ErrorV1
	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&resp); err != nil {
		t.Fatalf("Unexpected error while deserializing response: %v", err)
	}

	if resp.Code != types.CodeUndefinedDocument {
		t.Fatalf("Expected undefiend code but got: %v", resp)
	}
}

// Tests that the responses for (theoretically) valid resources but with forbidden methods return the proper status code
func Test405StatusCodev1(t *testing.T) {
	tests := []struct {
		note string
		reqs []tr
	}{
		{"v1 data one level 405", []tr{
			{http.MethodHead, "/data/lvl1", "", 405, ""},
			{http.MethodConnect, "/data/lvl1", "", 405, ""},
			{http.MethodOptions, "/data/lvl1", "", 405, ""},
			{http.MethodTrace, "/data/lvl1", "", 405, ""},
		}},
		{"v1 data 405", []tr{
			{http.MethodHead, "/data", "", 405, ""},
			{http.MethodConnect, "/data", "", 405, ""},
			{http.MethodOptions, "/data", "", 405, ""},
			{http.MethodTrace, "/data", "", 405, ""},
			{http.MethodDelete, "/data", "", 405, ""},
		}},
		{"v1 policies 405", []tr{
			{http.MethodHead, "/policies", "", 405, ""},
			{http.MethodConnect, "/policies", "", 405, ""},
			{http.MethodDelete, "/policies", "", 405, ""},
			{http.MethodOptions, "/policies", "", 405, ""},
			{http.MethodTrace, "/policies", "", 405, ""},
			{http.MethodPost, "/policies", "", 405, ""},
			{http.MethodPut, "/policies", "", 405, ""},
			{http.MethodPatch, "/policies", "", 405, ""},
		}},
		{"v1 policies one level 405", []tr{
			{http.MethodHead, "/policies/lvl1", "", 405, ""},
			{http.MethodConnect, "/policies/lvl1", "", 405, ""},
			{http.MethodOptions, "/policies/lvl1", "", 405, ""},
			{http.MethodTrace, "/policies/lvl1", "", 405, ""},
			{http.MethodPost, "/policies/lvl1", "", 405, ""},
		}},
		{"v1 query one level 405", []tr{
			{http.MethodHead, "/query/lvl1", "", 405, ""},
			{http.MethodConnect, "/query/lvl1", "", 405, ""},
			{http.MethodDelete, "/query/lvl1", "", 405, ""},
			{http.MethodOptions, "/query/lvl1", "", 405, ""},
			{http.MethodTrace, "/query/lvl1", "", 405, ""},
			{http.MethodPost, "/query/lvl1", "", 405, ""},
			{http.MethodPut, "/query/lvl1", "", 405, ""},
			{http.MethodPatch, "/query/lvl1", "", 405, ""},
		}},
		{"v1 query 405", []tr{
			{http.MethodHead, "/query", "", 405, ""},
			{http.MethodConnect, "/query", "", 405, ""},
			{http.MethodDelete, "/query", "", 405, ""},
			{http.MethodOptions, "/query", "", 405, ""},
			{http.MethodTrace, "/query", "", 405, ""},
			{http.MethodPut, "/query", "", 405, ""},
			{http.MethodPatch, "/query", "", 405, ""},
		}},
	}
	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			executeRequests(t, tc.reqs)
		})
	}
}

// Tests that the responses for (theoretically) valid resources but with forbidden methods return the proper status code
func Test405StatusCodev0(t *testing.T) {
	tests := []struct {
		note string
		reqs []tr
	}{
		{"v0 data one levels 405", []tr{
			{http.MethodHead, "/data/lvl2", "", 405, ""},
			{http.MethodConnect, "/data/lvl2", "", 405, ""},
			{http.MethodDelete, "/data/lvl2", "", 405, ""},
			{http.MethodOptions, "/data/lvl2", "", 405, ""},
			{http.MethodTrace, "/data/lvl2", "", 405, ""},
			{http.MethodGet, "/data/lvl2", "", 405, ""},
			{http.MethodPatch, "/data/lvl2", "", 405, ""},
			{http.MethodPut, "/data/lvl2", "", 405, ""},
		}},
		{"v0 data 405", []tr{
			{http.MethodHead, "/data", "", 405, ""},
			{http.MethodConnect, "/data", "", 405, ""},
			{http.MethodDelete, "/data", "", 405, ""},
			{http.MethodOptions, "/data", "", 405, ""},
			{http.MethodTrace, "/data", "", 405, ""},
			{http.MethodGet, "/data", "", 405, ""},
			{http.MethodPatch, "/data", "", 405, ""},
			{http.MethodPut, "/data", "", 405, ""},
		}},
	}
	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			executeRequestsv0(t, tc.reqs)
		})
	}
}

func TestCompileV1(t *testing.T) {

	mod := `package test

	p {
		input.x = 1
	}

	q {
		data.a[i] = input.x
	}

	default r = true
	`

	expQuery := func(s string) string {
		return fmt.Sprintf(`{"result": {"queries": [%v]}}`, string(util.MustMarshalJSON(ast.MustParseBody(s))))
	}

	expQueryAndSupport := func(q string, m string) string {
		return fmt.Sprintf(`{"result": {"queries": [%v], "support": [%v]}}`, string(util.MustMarshalJSON(ast.MustParseBody(q))), string(util.MustMarshalJSON(ast.MustParseModule(m))))
	}

	tests := []struct {
		note string
		trs  []tr
	}{
		{
			note: "basic",
			trs: []tr{
				{http.MethodPut, "/policies/test", mod, 200, ""},
				{http.MethodPost, "/compile", `{
					"unknowns": ["input"],
					"query": "data.test.p = true"
				}`, 200, expQuery("input.x = 1")},
			},
		},
		{
			note: "subtree",
			trs: []tr{
				{http.MethodPost, "/compile", `{
					"unknowns": ["input.x"],
					"input": {"y": 1},
					"query": "input.x > input.y"
				}`, 200, expQuery("input.x > 1")},
			},
		},
		{
			note: "data",
			trs: []tr{
				{http.MethodPut, "/policies/test", mod, 200, ""},
				{http.MethodPost, "/compile", `{
					"unknowns": ["data.a"],
					"input": {
						"x": 1
					},
					"query": "data.test.q = true"
				}`, 200, expQuery("1 = data.a[i1]")},
			},
		},
		{
			note: "escaped string",
			trs: []tr{
				{http.MethodPost, "/compile", `{
					"query": "input[\"x\"] = 1"
				}`, 200, expQuery("input.x = 1")},
			},
		},
		{
			note: "support",
			trs: []tr{
				{http.MethodPut, "/policies/test", mod, 200, ""},
				{http.MethodPost, "/compile", `{
					"query": "data.test.r = true"
				}`, 200, expQueryAndSupport(
					`data.partial.test.r = true`,
					`package partial.test

					default r = true
					`)},
			},
		},
		{
			note: "empty unknowns",
			trs: []tr{
				{http.MethodPost, "/compile", `{"query": "input.x > 1", "unknowns": []}`, 200, `{"result": {}}`},
			},
		},
		{
			note: "never defined",
			trs: []tr{
				{http.MethodPost, "/compile", `{"query": "1 = 2"}`, 200, `{"result": {}}`},
			},
		},
		{
			note: "always defined",
			trs: []tr{
				{http.MethodPost, "/compile", `{"query": "1 = 1"}`, 200, `{"result": {"queries": [[]]}}`},
			},
		},
		{
			note: "error: bad request",
			trs:  []tr{{http.MethodPost, "/compile", `{"input": [{]}`, 400, ``}},
		},
		{
			note: "error: empty query",
			trs:  []tr{{http.MethodPost, "/compile", `{}`, 400, ""}},
		},
		{
			note: "error: bad query",
			trs:  []tr{{http.MethodPost, "/compile", `{"query": "x %!> 9"}`, 400, ""}},
		},
		{
			note: "error: bad unknown",
			trs:  []tr{{http.MethodPost, "/compile", `{"unknowns": ["input."], "query": "true"}`, 400, ""}},
		},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			executeRequests(t, tc.trs)
		})
	}
}

func TestCompileV1Observability(t *testing.T) {

	f := newFixture(t)

	f.v1(http.MethodPut, "/policies/test", `package test

	p { input.x = 1 }`, 200, "")

	compileReq := newReqV1(http.MethodPost, "/compile?metrics&explain=full", `{
		"query": "data.test.p = true"
	}`)

	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, compileReq)

	var response types.CompileResponseV1
	if err := json.NewDecoder(f.recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}

	if len(response.Explanation) == 0 {
		t.Fatal("Expected non-empty explanation")
	}

	if _, ok := response.Metrics["timer_rego_partial_eval_ns"]; !ok {
		t.Fatal("Expected partial evaluation latency")
	}
}

func TestCompileV1UnsafeBuiltin(t *testing.T) {
	f := newFixture(t)
	get := newReqV1(http.MethodPost, `/compile`, `{"query": "http.send({\"method\": \"get\", \"url\": \"foo.com\"}, x)"}`)
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 400 {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}

	expected := `{
  "code": "invalid_parameter",
  "message": "unsafe built-in function calls in query: http.send"
}`

	if f.recorder.Body.String() != expected {
		t.Fatalf(`Expected %v but got: %v`, expected, f.recorder.Body.String())
	}
}

func TestDataV1Redirection(t *testing.T) {
	f := newFixture(t)
	// Testing redirect at the root level
	if err := f.v1(http.MethodPut, "/data/", `{"foo": [1,2,3]}`, 301, ""); err != nil {
		t.Fatalf("Unexpected error from PUT: %v", err)
	}
	locHdr := f.recorder.Header().Get("Location")
	if strings.Compare(locHdr, "/v1/data") != 0 {
		t.Fatalf("Unexpected error Location header value: %v", locHdr)
	}
	RedirectedPath := strings.SplitAfter(locHdr, "/v1")[1]
	if err := f.v1(http.MethodPut, RedirectedPath, `{"foo": [1,2,3]}`, 204, ""); err != nil {
		t.Fatalf("Unexpected error from PUT: %v", err)
	}
	if err := f.v1(http.MethodGet, RedirectedPath, "", 200, `{"result": {"foo": [1,2,3]}}`); err != nil {
		t.Fatalf("Unexpected error from GET: %v", err)
	}
	// Now we test redirection a few levels down
	if err := f.v1(http.MethodPut, "/data/a/b/c/", `{"foo": [1,2,3]}`, 301, ""); err != nil {
		t.Fatalf("Unexpected error from PUT: %v", err)
	}
	locHdrLv := f.recorder.Header().Get("Location")
	if strings.Compare(locHdrLv, "/v1/data/a/b/c") != 0 {
		t.Fatalf("Unexpected error Location header value: %v", locHdrLv)
	}
	RedirectedPathLvl := strings.SplitAfter(locHdrLv, "/v1")[1]
	if err := f.v1(http.MethodPut, RedirectedPathLvl, `{"foo": [1,2,3]}`, 204, ""); err != nil {
		t.Fatalf("Unexpected error from PUT: %v", err)
	}
	if err := f.v1(http.MethodGet, RedirectedPathLvl, "", 200, `{"result": {"foo": [1,2,3]}}`); err != nil {
		t.Fatalf("Unexpected error from GET: %v", err)
	}
}

func TestDataV1(t *testing.T) {
	testMod1 := `package testmod

import input.req1
import input.req2 as reqx
import input.req3.attr1

p[x] { q[x]; not r[x] }
q[x] { data.x.y[i] = x }
r[x] { data.x.z[i] = x }
g = true { req1.a[0] = 1; reqx.b[i] = 1 }
h = true { attr1[i] > 1 }
gt1 = true { req1 > 1 }
arr = [1, 2, 3, 4] { true }
undef = true { false }`

	testMod2 := `package testmod

p = [1, 2, 3, 4] { true }
q = {"a": 1, "b": 2} { true }`

	testMod4 := `package testmod

p = true { true }
p = false { true }`

	testMod5 := `package testmod.empty.mod`
	testMod6 := `package testmod.all.undefined

p = true { false }`

	testMod7 := `package testmod

	default p = false

	p { q[x]; not r[x] }

	q[1] { input.x = 1 }
	q[2] { input.y = 2 }
	r[1] { input.z = 3 }`

	testMod7Modified := `package testmod

	default p = false

	p { q[x]; not r[x] }

	q[1] { input.x = 1 }
	q[2] { input.y = 2 }
	r[1] { input.z = 3 }
	r[2] { input.z = 3 }`

	testMod8 := `package testmod

	p {
		data.x = 1
	}`

	tests := []struct {
		note string
		reqs []tr
	}{
		{"add root", []tr{
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": {"a": 1}}]`, 204, ""},
			{http.MethodGet, "/data/x/a", "", 200, `{"result": 1}`},
		}},
		{"append array", []tr{
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": []}]`, 204, ""},
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "-", "value": {"a": 1}}]`, 204, ""},
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "-", "value": {"a": 2}}]`, 204, ""},
			{http.MethodGet, "/data/x/0/a", "", 200, `{"result": 1}`},
			{http.MethodGet, "/data/x/1/a", "", 200, `{"result": 2}`},
		}},
		{"append array one-shot", []tr{
			{http.MethodPatch, "/data/x", `[
                {"op": "add", "path": "/", "value": []},
                {"op": "add", "path": "-", "value": {"a": 1}},
                {"op": "add", "path": "-", "value": {"a": 2}}
            ]`, 204, ""},
			{http.MethodGet, "/data/x/1/a", "", 200, `{"result": 2}`},
		}},
		{"insert array", []tr{
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": {
                "y": [
                    {"z": [1,2,3]},
                    {"z": [4,5,6]}
                ]
            }}]`, 204, ""},
			{http.MethodGet, "/data/x/y/1/z/2", "", 200, `{"result": 6}`},
			{http.MethodPatch, "/data/x/y/1", `[{"op": "add", "path": "/z/1", "value": 100}]`, 204, ""},
			{http.MethodGet, "/data/x/y/1/z", "", 200, `{"result": [4, 100, 5, 6]}`},
		}},
		{"patch root", []tr{
			{http.MethodPatch, "/data", `[
				{"op": "add",
				 "path": "/",
				 "value": {"a": 1, "b": 2}
				}
			]`, 204, ""},
			{http.MethodGet, "/data", "", 200, `{"result": {"a": 1, "b": 2}}`},
		}},
		{"patch invalid", []tr{
			{http.MethodPatch, "/data", `[
				{
					"op": "remove",
					"path": "/"
				}
			]`, 400, ""},
		}},
		{"patch abort", []tr{
			{http.MethodPatch, "/data", `[
				{"op": "add", "path": "/foo", "value": "hello"},
				{"op": "add", "path": "/bar", "value": "world"},
				{"op": "add", "path": "/foo/bad", "value": "deadbeef"}
			]`, 404, ""},
			{http.MethodGet, "/data", "", 200, `{"result": {}}`},
		}},
		{"put root", []tr{
			{http.MethodPut, "/data", `{"foo": [1,2,3]}`, 204, ""},
			{http.MethodGet, "/data", "", 200, `{"result": {"foo": [1,2,3]}}`},
		}},
		{"put deep makedir", []tr{
			{http.MethodPut, "/data/a/b/c/d", `1`, 204, ""},
			{http.MethodGet, "/data/a/b/c", "", 200, `{"result": {"d": 1}}`},
		}},
		{"put deep makedir partial", []tr{
			{http.MethodPut, "/data/a/b", `{}`, 204, ""},
			{http.MethodPut, "/data/a/b/c/d", `0`, 204, ""},
			{http.MethodGet, "/data/a/b/c", "", 200, `{"result": {"d": 0}}`},
		}},
		{"put exists overwrite", []tr{
			{http.MethodPut, "/data/a/b/c", `"hello"`, 204, ""},
			{http.MethodPut, "/data/a/b", `"goodbye"`, 204, ""},
			{http.MethodGet, "/data/a", "", 200, `{"result": {"b": "goodbye"}}`},
		}},
		{"put base write conflict", []tr{
			{http.MethodPut, "/data/a/b", `[1,2,3,4]`, 204, ""},
			{http.MethodPut, "/data/a/b/c/d", "0", 404, `{
				"code": "resource_conflict",
				"message": "storage_write_conflict_error: /a/b"
			}`},
		}},
		{"get virtual", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": {"y": [1,2,3,4], "z": [3,4,5,6]}}]`, 204, ""},
			{http.MethodGet, "/data/testmod/p", "", 200, `{"result": [1,2]}`},
		}},
		{"get with input", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodGet, "/data/testmod/g?input=%7B%22req1%22%3A%7B%22a%22%3A%5B1%5D%7D%2C+%22req2%22%3A%7B%22b%22%3A%5B0%2C1%5D%7D%7D", "", 200, `{"result": true}`},
		}},
		{"get with input (missing input value)", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodGet, "/data/testmod/g?input=%7B%22req1%22%3A%7B%22a%22%3A%5B1%5D%7D%7D", "", 200, "{}"}, // req2 not specified
		}},
		{"get with input (namespaced)", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodGet, "/data/testmod/h?input=%7B%22req3%22%3A%7B%22attr1%22%3A%5B4%2C3%2C2%2C1%5D%7D%7D", "", 200, `{"result": true}`},
		}},
		{"get with input (root)", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodGet, `/data/testmod/gt1?input={"req1":2}`, "", 200, `{"result": true}`},
		}},
		{"get with input (bad format)", []tr{
			{http.MethodGet, "/data/deadbeef?input", "", 400, `{
				"code": "invalid_parameter",
				"message": "parameter contains malformed input document: EOF"
			}`},
			{http.MethodGet, "/data/deadbeef?input=", "", 400, `{
				"code": "invalid_parameter",
				"message": "parameter contains malformed input document: EOF"
			}`},
			{http.MethodGet, `/data/deadbeef?input="foo`, "", 400, `{
				"code": "invalid_parameter",
				"message": "parameter contains malformed input document: unexpected EOF"
			}`},
		}},
		{"get with input (path error)", []tr{
			{http.MethodGet, `/data/deadbeef?input={"foo:1}`, "", 400, `{
				"code": "invalid_parameter",
				"message": "parameter contains malformed input document: unexpected EOF"
			}`},
		}},
		{"get empty and undefined", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodPut, "/policies/test2", testMod5, 200, ""},
			{http.MethodPut, "/policies/test3", testMod6, 200, ""},
			{http.MethodGet, "/data/testmod/undef", "", 200, "{}"},
			{http.MethodGet, "/data/doesnot/exist", "", 200, "{}"},
			{http.MethodGet, "/data/testmod/empty/mod", "", 200, `{
				"result": {}
			}`},
			{http.MethodGet, "/data/testmod/all/undefined", "", 200, `{
				"result": {}
			}`},
		}},
		{"get root", []tr{
			{http.MethodPut, "/policies/test", testMod2, 200, ""},
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": [1,2,3,4]}]`, 204, ""},
			{http.MethodGet, "/data", "", 200, `{"result": {"testmod": {"p": [1,2,3,4], "q": {"a":1, "b": 2}}, "x": [1,2,3,4]}}`},
		}},
		{"post root", []tr{
			{http.MethodPost, "/data", "", 200, `{"result": {}}`},
			{http.MethodPut, "/policies/test", testMod2, 200, ""},
			{http.MethodPost, "/data", "", 200, `{"result": {"testmod": {"p": [1,2,3,4], "q": {"b": 2, "a": 1}}}}`},
		}},
		{"post input", []tr{
			{http.MethodPut, "/policies/test", testMod1, 200, ""},
			{http.MethodPost, "/data/testmod/gt1", `{"input": {"req1": 2}}`, 200, `{"result": true}`},
		}},
		{"post malformed input", []tr{
			{http.MethodPost, "/data/deadbeef", `{"input": @}`, 400, `{
				"code": "invalid_parameter",
				"message": "body contains malformed input document: invalid character '@' looking for beginning of value"
			}`},
		}},
		{"post empty object", []tr{
			{http.MethodPost, "/data", `{}`, 200, `{"result": {}}`},
		}},
		{"post partial", []tr{
			{http.MethodPut, "/policies/test", testMod7, 200, ""},
			{http.MethodPost, "/data/testmod/p?partial", `{"input": {"x": 1, "y": 2, "z": 9999}}`, 200, `{"result": true}`},
			{http.MethodPost, "/data/testmod/p?partial", `{"input": {"x": 1, "z": 3}}`, 200, `{"result": false}`},
			{http.MethodPost, "/data/testmod/p", `{"input": {"x": 1, "y": 2, "z": 9999}}`, 200, `{"result": true}`},
			{http.MethodPost, "/data/testmod/p", `{"input": {"x": 1, "z": 3}}`, 200, `{"result": false}`},
		}},
		{"partial invalidate policy", []tr{
			{http.MethodPut, "/policies/test", testMod7, 200, ""},
			{http.MethodPost, "/data/testmod/p?partial", `{"input": {"x": 1, "y": 2, "z": 3}}`, 200, `{"result": true}`},
			{http.MethodPut, "/policies/test", testMod7Modified, 200, ""},
			{http.MethodPost, "/data/testmod/p?partial", `{"input": {"x": 1, "y": 2, "z": 3}}`, 200, `{"result": false}`},
		}},
		{"partial invalidate data", []tr{
			{http.MethodPut, "/policies/test", testMod8, 200, ""},
			{http.MethodPost, "/data/testmod/p?partial", "", 200, `{}`},
			{http.MethodPut, "/data/x", `1`, 204, ""},
			{http.MethodPost, "/data/testmod/p?partial", "", 200, `{"result": true}`},
		}},
		{"evaluation conflict", []tr{
			{http.MethodPut, "/policies/test", testMod4, 200, ""},
			{http.MethodPost, "/data/testmod/p", "", 500, `{
    		  "code": "internal_error",
    		  "errors": [
    		    {
    		      "code": "eval_conflict_error",
    		      "location": {
    		        "col": 1,
    		        "file": "test",
    		        "row": 4
    		      },
    		      "message": "complete rules must not produce multiple outputs"
    		    }
    		  ],
    		  "message": "error(s) occurred while evaluating query"
    		}`},
		}},
		{"query wildcards omitted", []tr{
			{http.MethodPatch, "/data/x", `[{"op": "add", "path": "/", "value": [1,2,3,4]}]`, 204, ""},
			{http.MethodGet, "/query?q=data.x[_]%20=%20x", "", 200, `{"result": [{"x": 1}, {"x": 2}, {"x": 3}, {"x": 4}]}`},
		}},
		{"query undefined", []tr{
			{http.MethodGet, "/query?q=a=1%3Bb=2%3Ba=b", "", 200, `{}`},
		}},
		{"query compiler error", []tr{
			{http.MethodGet, "/query?q=x", "", 400, ""},
			// Subsequent query should not fail.
			{http.MethodGet, "/query?q=x=1", "", 200, `{"result": [{"x": 1}]}`},
		}},
		{"delete and check", []tr{
			{http.MethodDelete, "/data/a/b", "", 404, ""},
			{http.MethodPut, "/data/a/b/c/d", `1`, 204, ""},
			{http.MethodGet, "/data/a/b/c", "", 200, `{"result": {"d": 1}}`},
			{http.MethodDelete, "/data/a/b", "", 204, ""},
			{http.MethodGet, "/data/a/b/c/d", "", 200, `{}`},
			{http.MethodGet, "/data/a", "", 200, `{"result": {}}`},
			{http.MethodGet, "/data/a/b/c", "", 200, `{}`},
		}},
		{"escaped paths", []tr{
			{http.MethodPut, "/data/a%2Fb", `{"c/d": 1}`, 204, ""},
			{http.MethodGet, "/data", "", 200, `{"result": {"a/b": {"c/d": 1}}}`},
			{http.MethodGet, "/data/a%2Fb/c%2Fd", "", 200, `{"result": 1}`},
			{http.MethodGet, "/data/a/b", "", 200, `{}`},
			{http.MethodPost, "/data/a%2Fb/c%2Fd", "", 200, `{"result": 1}`},
			{http.MethodPost, "/data/a/b", "", 200, `{}`},
			{http.MethodPatch, "/data/a%2Fb", `[{"op": "add", "path": "/e%2Ff", "value": 2}]`, 204, ""},
			{http.MethodPost, "/data", "", 200, `{"result": {"a/b": {"c/d": 1, "e/f": 2}}}`},
		}},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			executeRequests(t, tc.reqs)
		})
	}
}

func TestDataYAML(t *testing.T) {

	testMod1 := `package testmod
import input.req1
gt1 = true { req1 > 1 }`

	inputYaml1 := `
---
input:
  req1: 2`

	inputYaml2 := `
---
req1: 2`

	f := newFixture(t)

	if err := f.v1(http.MethodPut, "/policies/test", testMod1, 200, ""); err != nil {
		t.Fatalf("Unexpected error from PUT /policies/test: %v", err)
	}

	// First JSON and then later yaml to make sure both work
	if err := f.v1(http.MethodPost, "/data/testmod/gt1", `{"input": {"req1": 2}}`, 200, `{"result": true}`); err != nil {
		t.Fatalf("Unexpected error from PUT /policies/test: %v", err)
	}

	req := newReqV1(http.MethodPost, "/data/testmod/gt1", inputYaml1)
	req.Header.Set("Content-Type", "application/x-yaml")
	if err := f.executeRequest(req, 200, `{"result": true}`); err != nil {
		t.Fatalf("Unexpected error from POST with yaml: %v", err)
	}

	req = newReqV0(http.MethodPost, "/data/testmod/gt1", inputYaml2)
	req.Header.Set("Content-Type", "application/x-yaml")
	if err := f.executeRequest(req, 200, `true`); err != nil {
		t.Fatalf("Unexpected error from POST with yaml: %v", err)
	}

	if err := f.v1(http.MethodPut, "/policies/test2", `package system
main = data.testmod.gt1`, 200, ""); err != nil {
		t.Fatalf("Unexpected error from PUT /policies/test: %v", err)
	}

	req = newReqUnversioned(http.MethodPost, "/", inputYaml2)
	req.Header.Set("Content-Type", "application/x-yaml")
	if err := f.executeRequest(req, 200, `true`); err != nil {
		t.Fatalf("Unexpected error from POST with yaml: %v", err)
	}

}

func TestDataPutV1IfNoneMatch(t *testing.T) {
	f := newFixture(t)
	if err := f.v1(http.MethodPut, "/data/a/b/c", "0", 204, ""); err != nil {
		t.Fatalf("Unexpected error from PUT /data/a/b/c: %v", err)
	}
	req := newReqV1(http.MethodPut, "/data/a/b/c", "1")
	req.Header.Set("If-None-Match", "*")
	if err := f.executeRequest(req, 304, ""); err != nil {
		t.Fatalf("Unexpected error from PUT with If-None-Match=*: %v", err)
	}
}

func TestDataWatch(t *testing.T) {
	f := newFixture(t)

	// Test watching /data.
	exp := strings.Join([]string{
		"HTTP/1.1 200 OK\nContent-Type: application/json\nTransfer-Encoding: chunked\n\ne",
		`{"result":{}}
`,
		`1f`,
		`{"result":{"x":{"a":1,"b":2}}}
`,
		`17`,
		`{"result":{"x":"foo"}}
`,
		`13`,
		`{"result":{"x":7}}
`,
		``,
	}, "\r\n")
	r1 := newMockConn()
	r2 := newMockConn()

	get := newReqV1(http.MethodGet, `/data?watch`, "")
	go f.server.Handler.ServeHTTP(r1, get)
	<-r1.hijacked
	<-r1.write

	get = newReqV1(http.MethodPost, `/data?watch`, "")
	go f.server.Handler.ServeHTTP(r2, get)
	<-r2.hijacked
	<-r2.write

	tests := []tr{
		{http.MethodPut, "/data/x", `{"a":1,"b":2}`, 204, ""},
		{http.MethodPut, "/data/x", `"foo"`, 204, ""},
		{http.MethodPut, "/data/x", `7`, 204, ""},
	}

	for _, tr := range tests {
		if err := f.v1(tr.method, tr.path, tr.body, tr.code, tr.resp); err != nil {
			t.Fatal(err)
		}
		<-r1.write
		<-r2.write
	}
	r1.Close()
	r2.Close()

	if result := r1.buf.String(); result != exp {
		t.Fatalf("Expected stream to equal %s, got %s", exp, result)
	}
	if result := r2.buf.String(); result != exp {
		t.Fatalf("Expected stream to equal %s, got %s", exp, result)
	}
}

const servers = `[
  {
    "id": "s1",
    "name": "app",
    "protocols": [
      "https",
      "ssh"
    ],
    "ports": [
      "p1",
      "p2",
      "p3"
    ]
  },
  {
    "id": "s2",
    "name": "db",
    "protocols": [
      "mysql"
    ],
    "ports": [
      "p3"
    ]
  },
  {
    "id": "s3",
    "name": "cache",
    "protocols": [
      "memcache",
      "http"
    ],
    "ports": [
      "p3"
    ]
  },
  {
    "id": "s4",
    "name": "dev",
    "protocols": [
      "http"
    ],
    "ports": [
      "p1",
      "p2"
    ]
  }
]`

func TestDataWatchDocsExample(t *testing.T) {
	f := newFixture(t)
	if err := f.v1(http.MethodPut, "/data/servers", servers, 204, ""); err != nil {
		t.Fatal(err)
	}

	exp := strings.Join([]string{
		"HTTP/1.1 200 OK\nContent-Type: application/json\nTransfer-Encoding: chunked\n\n281",
		`{
  "result": [
    {
      "id": "s1",
      "name": "app",
      "ports": [
        "p1",
        "p2",
        "p3"
      ],
      "protocols": [
        "https",
        "ssh"
      ]
    },
    {
      "id": "s2",
      "name": "db",
      "ports": [
        "p3"
      ],
      "protocols": [
        "mysql"
      ]
    },
    {
      "id": "s3",
      "name": "cache",
      "ports": [
        "p3"
      ],
      "protocols": [
        "memcache",
        "http"
      ]
    },
    {
      "id": "s4",
      "name": "dev",
      "ports": [
        "p1",
        "p2"
      ],
      "protocols": [
        "http"
      ]
    }
  ]
}
`,

		`308`,
		`{
  "result": [
    {
      "id": "s1",
      "name": "app",
      "ports": [
        "p1",
        "p2",
        "p3"
      ],
      "protocols": [
        "https",
        "ssh"
      ]
    },
    {
      "id": "s2",
      "name": "db",
      "ports": [
        "p3"
      ],
      "protocols": [
        "mysql"
      ]
    },
    {
      "id": "s3",
      "name": "cache",
      "ports": [
        "p3"
      ],
      "protocols": [
        "memcache",
        "http"
      ]
    },
    {
      "id": "s4",
      "name": "dev",
      "ports": [
        "p1",
        "p2"
      ],
      "protocols": [
        "http"
      ]
    },
    {
      "id": "s5",
      "name": "job",
      "ports": [
        "p3"
      ],
      "protocols": [
        "amqp"
      ]
    }
  ]
}
`,
		`281`,
		`{
  "result": [
    {
      "id": "s1",
      "name": "app",
      "ports": [
        "p1",
        "p2",
        "p3"
      ],
      "protocols": [
        "https",
        "ssh"
      ]
    },
    {
      "id": "s3",
      "name": "cache",
      "ports": [
        "p3"
      ],
      "protocols": [
        "memcache",
        "http"
      ]
    },
    {
      "id": "s4",
      "name": "dev",
      "ports": [
        "p1",
        "p2"
      ],
      "protocols": [
        "http"
      ]
    },
    {
      "id": "s5",
      "name": "job",
      "ports": [
        "p3"
      ],
      "protocols": [
        "amqp"
      ]
    }
  ]
}
`,
		``,
	}, "\r\n")

	tests := []tr{
		{http.MethodPatch, "/data/servers", `[{"op": "add", "path": "-", "value": {"id": "s5", "name": "job", "protocols": ["amqp"], "ports": ["p3"]}}]`, 204, ""},
		{http.MethodPatch, "/data/servers", `[{"op": "remove", "path": "1"}]`, 204, ""},
	}

	recorder := newMockConn()
	get := newReqV1(http.MethodGet, `/data/servers?watch&pretty=true`, "")
	go f.server.Handler.ServeHTTP(recorder, get)
	<-recorder.hijacked
	<-recorder.write

	for _, tr := range tests {
		if err := f.v1(tr.method, tr.path, tr.body, tr.code, tr.resp); err != nil {
			t.Fatal(err)
		}
		<-recorder.write
	}
	recorder.Close()

	if result := recorder.buf.String(); result != exp {
		t.Fatalf("Expected stream to equal %s, got %s", exp, result)
	}
}

func TestDataGetExplainFull(t *testing.T) {
	f := newFixture(t)

	f.v1(http.MethodPut, "/data/x", `{"a":1,"b":2}`, 204, "")

	req := newReqV1(http.MethodGet, "/data/x?explain=full", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	explain := mustUnmarshalTrace(result.Explanation)
	nexpect := 5
	if len(explain) != nexpect {
		t.Fatalf("Expected exactly %d events but got %d", nexpect, len(explain))
	}

	_, ok := explain[2].Node.(ast.Body)
	if !ok {
		t.Fatalf("Expected body for node but got: %v", explain[2].Node)
	}

	if len(explain[2].Locals) != 1 {
		t.Fatalf("Expected one binding but got: %v", explain[2].Locals)
	}

	req = newReqV1(http.MethodGet, "/data/deadbeef?explain=full", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	result = types.DataResponseV1{}

	if f.recorder.Code != 200 {
		t.Fatalf("Expected status code to be 200 but got: %v", f.recorder.Code)
	}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	explain = mustUnmarshalTrace(result.Explanation)
	if len(explain) != 3 {
		t.Fatalf("Expected exactly 3 events but got %d", len(explain))
	}

	if explain[2].Op != "fail" {
		t.Fatalf("Expected last event to be 'fail' but got: %v", explain[2])
	}

	req = newReqV1(http.MethodGet, "/data/x?explain=full&pretty=true", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	result = types.DataResponseV1{}
	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	exp := []interface{}{`Enter data.x = _`, `| Eval data.x = _`, `| Exit data.x = _`, `Redo data.x = _`, `| Redo data.x = _`}

	actual := util.MustUnmarshalJSON(result.Explanation).([]interface{})
	if !reflect.DeepEqual(actual, exp) {
		t.Fatalf(`Expected pretty explanation to be %v, got %v`, exp, actual)
	}
}

func TestDataPostExplain(t *testing.T) {
	f := newFixture(t)

	f.v1(http.MethodPut, "/policies/test", `package test

p = [1, 2, 3, 4] { true }`, 200, "")

	req := newReqV1(http.MethodPost, "/data/test/p?explain=full", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	explain := mustUnmarshalTrace(result.Explanation)
	nexpect := 11

	if len(explain) != nexpect {
		t.Fatalf("Expected exactly %d events but got %d", nexpect, len(explain))
	}

	var expected interface{}

	if err := util.UnmarshalJSON([]byte(`[1,2,3,4]`), &expected); err != nil {
		panic(err)
	}

	if result.Result == nil || !reflect.DeepEqual(*result.Result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result.Result)
	}

}

func TestDataMetrics(t *testing.T) {

	f := newFixture(t)

	req := newReqV1(http.MethodPost, "/data?metrics", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result types.DataResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	// Test some basic well-known metrics.
	expected := []string{
		"timer_rego_query_parse_ns",
		"timer_rego_query_compile_ns",
		"timer_rego_query_eval_ns",
	}

	for _, key := range expected {
		if result.Metrics[key] == nil {
			t.Fatalf("Expected non-zero metric for %v but got: %v", key, result)
		}
	}

	req = newReqV1(http.MethodPost, "/data?metrics&partial", "")

	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	result = types.DataResponseV1{}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	expected = []string{
		"timer_rego_query_parse_ns",
		"timer_rego_query_compile_ns",
		"timer_rego_query_eval_ns",
		"timer_rego_partial_eval_ns",
	}

	for _, key := range expected {
		if result.Metrics[key] == nil {
			t.Fatalf("Expected non-zero metric for %v but got: %v", key, result)
		}
	}

}

func TestV1Pretty(t *testing.T) {

	f := newFixture(t)
	f.v1(http.MethodPatch, "/data/x", `[{"op": "add", "path":"/", "value": [1,2,3,4]}]`, 204, "")

	req := newReqV1(http.MethodGet, "/data/x?pretty=true", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	lines := strings.Split(f.recorder.Body.String(), "\n")
	if len(lines) != 8 {
		t.Errorf("Expected 8 lines in output but got %d:\n%v", len(lines), lines)
	}

	req = newReqV1(http.MethodGet, "/query?q=data.x[i]&pretty=true", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	lines = strings.Split(f.recorder.Body.String(), "\n")
	if len(lines) != 16 {
		t.Errorf("Expected 16 lines of output but got %d:\n%v", len(lines), lines)
	}
}

func TestIndexGetEscaped(t *testing.T) {
	f := newFixture(t)
	get, err := http.NewRequest(http.MethodGet, `/?q=</textarea><script>alert(1)</script>`, strings.NewReader(""))
	if err != nil {
		panic(err)
	}
	f.server.Handler.ServeHTTP(f.recorder, get)
	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got: %v", f.recorder)
		return
	}
	page := f.recorder.Body.String()
	exp := "&lt;/textarea&gt;&lt;script&gt;alert(1)&lt;/script&gt;"
	if !strings.Contains(page, exp) {
		t.Fatalf("Expected page to contain escaped URL parameter but got: %v", page)
	}

}

func TestIndexGet(t *testing.T) {
	f := newFixture(t)
	get, err := http.NewRequest(http.MethodGet, `/?q=foo = 1&input=`, strings.NewReader(""))
	if err != nil {
		panic(err)
	}
	f.server.Handler.ServeHTTP(f.recorder, get)
	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got: %v", f.recorder)
		return
	}
	page := f.recorder.Body.String()
	if !strings.Contains(page, "Query result") {
		t.Errorf("Expected page to contain 'Query result' but got: %v", page)
		return
	}
}

func TestIndexGetCompileError(t *testing.T) {
	f := newFixture(t)
	// "foo" is not bound
	get, err := http.NewRequest(http.MethodGet, `/?q=foo`, strings.NewReader(""))
	if err != nil {
		panic(err)
	}
	f.server.Handler.ServeHTTP(f.recorder, get)
	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got: %v", f.recorder)
		return
	}
	page := f.recorder.Body.String()
	if !strings.Contains(page, "foo is unsafe") {
		t.Errorf("Expected page to contain 'foo is unsafe' but got: %v", page)
		return
	}
}

func TestVersionGet(t *testing.T) {

	f := newFixture(t)

	get := newReqV1(http.MethodGet, "/data/system/version", "")
	f.server.Handler.ServeHTTP(f.recorder, get)
	if f.recorder.Code != 200 {
		t.Fatalf("Expected 200 OK but got %v", f.recorder)
		return
	}

	page := f.recorder.Body.String()
	var re = regexp.MustCompile(`[\s\S]*Version\b[\s\S]*BuildCommit\b[\s\S]*BuildTimestamp\b[\s\S]*BuildHostname\b`)
	if !re.MatchString(page) {
		t.Errorf("Expected page to contain 'version' but got: %v", page)
		return
	}
}

func TestPoliciesPutV1(t *testing.T) {
	f := newFixture(t)
	req := newReqV1(http.MethodPut, "/policies/1", testMod)

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(f.recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Unexpected error while unmarshalling response: %v", err)
	}

	if len(response) != 0 {
		t.Fatalf("Expected empty wrapper object")
	}
}

func TestPoliciesPutV1Empty(t *testing.T) {
	f := newFixture(t)
	req := newReqV1(http.MethodPut, "/policies/1", "")

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}
}

func TestPoliciesPutV1ParseError(t *testing.T) {
	f := newFixture(t)
	req := newReqV1(http.MethodPut, "/policies/test", `
    package a.b.c

    p ;- true
    `)

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}

	response := map[string]interface{}{}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if !reflect.DeepEqual(response["code"], types.CodeInvalidParameter) {
		t.Fatalf("Expected code %v but got: %v", types.CodeInvalidParameter, response)
	}

	v := ast.MustInterfaceToValue(response)

	name, err := v.Find(ast.MustParseRef("_.errors[0].location.file")[1:])
	if err != nil {
		t.Fatalf("Expecfted to find name in errors but: %v", err)
	}

	if name.Compare(ast.String("test")) != 0 {
		t.Fatalf("Expected name ot equal test but got: %v", name)
	}
}

func TestPoliciesPutV1CompileError(t *testing.T) {
	f := newFixture(t)
	req := newReqV1(http.MethodPut, "/policies/test", `package a.b.c

p[x] { q[x] }
q[x] { p[x] }`,
	)

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}

	response := map[string]interface{}{}

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if !reflect.DeepEqual(response["code"], types.CodeInvalidParameter) {
		t.Fatalf("Expected code %v but got: %v", types.CodeInvalidParameter, response)
	}

	v := ast.MustInterfaceToValue(response)

	name, err := v.Find(ast.MustParseRef("_.errors[0].location.file")[1:])
	if err != nil {
		t.Fatalf("Expecfted to find name in errors but: %v", err)
	}

	if name.Compare(ast.String("test")) != 0 {
		t.Fatalf("Expected name ot equal test but got: %v", name)
	}
}

func TestPoliciesListV1(t *testing.T) {
	f := newFixture(t)
	put := newReqV1(http.MethodPut, "/policies/1", testMod)
	f.server.Handler.ServeHTTP(f.recorder, put)
	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}
	f.reset()
	list := newReqV1(http.MethodGet, "/policies", "")

	f.server.Handler.ServeHTTP(f.recorder, list)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	// var policies []*PolicyV1
	var response types.PolicyListResponseV1

	err := util.NewJSONDecoder(f.recorder.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Expected policy list but got error: %v", err)
	}

	expected := []types.PolicyV1{
		newPolicy("1", testMod),
	}
	if len(expected) != len(response.Result) {
		t.Fatalf("Expected %d policies but got: %v", len(expected), response.Result)
	}
	for i := range expected {
		if !expected[i].Equal(response.Result[i]) {
			t.Fatalf("Expected policies to be equal. Expected:\n\n%v\n\nGot:\n\n%v\n", expected, response.Result)
		}
	}
}

func TestPoliciesGetV1(t *testing.T) {
	f := newFixture(t)
	put := newReqV1(http.MethodPut, "/policies/1", testMod)
	f.server.Handler.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	f.reset()
	get := newReqV1(http.MethodGet, "/policies/1", "")

	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	var response types.PolicyGetResponseV1
	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := newPolicy("1", testMod)

	if !expected.Equal(response.Result) {
		t.Errorf("Expected policies to be equal. Expected:\n\n%v\n\nGot:\n\n%v\n", expected, response.Result)
	}
}

func TestPoliciesDeleteV1(t *testing.T) {
	f := newFixture(t)
	put := newReqV1(http.MethodPut, "/policies/1", testMod)
	f.server.Handler.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	f.reset()
	del := newReqV1(http.MethodDelete, "/policies/1", "")

	f.server.Handler.ServeHTTP(f.recorder, del)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(f.recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Unexpected unmarshal error: %v", err)
	}

	if len(response) > 0 {
		t.Fatalf("Expected empty response but got: %v", response)
	}

	f.reset()
	get := newReqV1(http.MethodGet, "/policies/1", "")
	f.server.Handler.ServeHTTP(f.recorder, get)
	if f.recorder.Code != 404 {
		t.Fatalf("Expected not found but got %v", f.recorder)
	}
}

func TestPoliciesPathSlashes(t *testing.T) {
	f := newFixture(t)
	if err := f.v1(http.MethodPut, "/policies/a/b/c.rego", testMod, 200, ""); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := f.v1(http.MethodGet, "/policies/a/b/c.rego", testMod, 200, ""); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestQueryPostBasic(t *testing.T) {
	f := newFixture(t)
	f.server, _ = New().
		WithAddresses([]string{":8182"}).
		WithStore(f.server.store).
		WithManager(f.server.manager).
		WithDiagnosticsBuffer(NewBoundedBuffer(8)).
		Init(context.Background())

	setup := []tr{
		{http.MethodPost, "/query", `{"query": "a=data.k.x with data.k as {\"x\" : 7}"}`, 200, `{"result":[{"a":7}]}`},
	}

	for _, tr := range setup {
		req := newReqV1(tr.method, tr.path, tr.body)
		req.RemoteAddr = "testaddr"

		if err := f.executeRequest(req, tr.code, tr.resp); err != nil {
			t.Fatal(err)
		}
	}
}

func TestQueryWatchBasic(t *testing.T) {
	// Test basic watch results.
	exp := strings.Join([]string{
		"HTTP/1.1 200 OK\nContent-Type: application/json\nTransfer-Encoding: chunked\n\n10",
		`{"result":null}
`,
		`7c`,
		`{"result":[{"expressions":[{"value":true,"text":"a=data.x","location":{"row":1,"col":1}}],"bindings":{"a":{"a":1,"b":2}}}]}
`,
		`74`,
		`{"result":[{"expressions":[{"value":true,"text":"a=data.x","location":{"row":1,"col":1}}],"bindings":{"a":"foo"}}]}
`,
		`70`,
		`{"result":[{"expressions":[{"value":true,"text":"a=data.x","location":{"row":1,"col":1}}],"bindings":{"a":7}}]}
`,
		``,
	}, "\r\n")

	requests := []*http.Request{
		newReqV1(http.MethodGet, `/query?q=a=data.x&watch`, ""),
		newReqV1(http.MethodPost, `/query?&watch`, `{"query": "a=data.x"}`),
	}

	for _, get := range requests {
		f := newFixture(t)
		recorder := newMockConn()
		go f.server.Handler.ServeHTTP(recorder, get)
		<-recorder.hijacked
		<-recorder.write

		tests := []trw{
			{tr{http.MethodPut, "/data/x", `{"a":1,"b":2}`, 204, ""}, recorder.write},
			{tr{http.MethodPut, "/data/x", `"foo"`, 204, ""}, recorder.write},
			{tr{http.MethodPut, "/data/x", `7`, 204, ""}, recorder.write},
		}

		for _, test := range tests {
			tr := test.tr
			if err := f.v1(tr.method, tr.path, tr.body, tr.code, tr.resp); err != nil {
				t.Fatal(err)
			}
			if test.wait != nil {
				<-test.wait
			}
		}
		recorder.Close()

		if result := recorder.buf.String(); result != exp {
			t.Fatalf("Expected stream to equal %s, got %s", exp, result)
		}
	}
}

func TestQueryWatchConcurrent(t *testing.T) {

	f := newFixture(t)

	r1, r2 := newMockConn(), newMockConn()

	setup := []tr{
		{http.MethodPut, "/data/x", `7`, 204, ""},
		{http.MethodPut, "/policies/foo", "package z\nr = y { y = data.a }", 200, ""},
		{http.MethodPut, "/data/y", `"foo"`, 204, ""},
		{http.MethodPut, "/data/a", `5`, 204, ""},
	}
	for _, s := range setup {
		if err := f.v1(s.method, s.path, s.body, s.code, s.resp); err != nil {
			t.Fatal(err)
		}
	}

	get1 := newReqV1(http.MethodGet, `/query?q=a=data.z.r%2Bdata.x&watch`, "")
	go f.server.Handler.ServeHTTP(r1, get1)
	<-r1.hijacked
	<-r1.write

	get2 := newReqV1(http.MethodGet, `/query?q=a=data.y&watch`, "")
	go f.server.Handler.ServeHTTP(r2, get2)
	<-r2.hijacked
	<-r2.write

	tests := []trw{
		{tr{http.MethodPut, "/data/a", `6`, 204, ""}, r1.write},
		{tr{http.MethodPut, "/data/a", `7`, 204, ""}, r1.write},
		{tr{http.MethodPut, "/data/y", `"bar"`, 204, ""}, r2.write},
		{tr{http.MethodPut, "/data/a", `8`, 204, ""}, r1.write},
		{tr{http.MethodPut, "/data/y", `"baz"`, 204, ""}, r2.write},
		{tr{http.MethodPut, "/data/a", `9`, 204, ""}, r1.write},
		{tr{http.MethodPut, "/data/a", `10`, 204, ""}, r1.write},
	}

	for _, test := range tests {
		tr := test.tr
		if err := f.v1(tr.method, tr.path, tr.body, tr.code, tr.resp); err != nil {
			t.Fatal(err)
		}
		if test.wait != nil {
			<-test.wait
		}
	}
	r1.Close()
	r2.Close()

	exp1 := util.MustUnmarshalJSON([]byte(`[
		{"a": 12},
		{"a": 13},
		{"a": 14},
		{"a": 15},
		{"a": 16},
		{"a": 17}
	]`))

	exp2 := util.MustUnmarshalJSON([]byte(`[
		{"a": "foo"},
		{"a": "bar"},
		{"a": "baz"}
	]`))

	stream1, err := r1.consumeQueryResultStream()
	if err != nil {
		t.Fatal(err)
	}

	result1 := queryResultStreamBindingSet(stream1)

	if !reflect.DeepEqual(result1, exp1) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp1, result1)
	}

	stream2, err := r2.consumeQueryResultStream()
	if err != nil {
		t.Fatal(err)
	}

	result2 := queryResultStreamBindingSet(stream2)

	if !reflect.DeepEqual(result2, exp2) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp2, result2)
	}
}

func TestQueryWatchMigrate(t *testing.T) {

	f := newFixture(t)

	testPolicy := `
		package z

		r = y { y = data.a }
	`

	if err := f.v1TestRequests([]tr{
		{http.MethodPut, "/data/x", "7", 204, ""},
		{http.MethodPut, "/data/a", "10", 204, ""},
		{http.MethodPut, "/policies/foo", testPolicy, 200, ""},
	}); err != nil {
		t.Fatal(err)
	}

	// Test migrating to a new compiler.
	recorder := newMockConn()

	get := newReqV1(http.MethodGet, `/query?q=a=data.z.r%2Bdata.x&watch`, "")
	go f.server.Handler.ServeHTTP(recorder, get)
	<-recorder.hijacked
	<-recorder.write

	if err := f.v1(http.MethodPut, "/policies/foo", "package z\nr = y { y = data.x }", 200, ""); err != nil {
		t.Fatal(err)
	}
	<-recorder.write

	tests := []trw{
		{tr{http.MethodPut, "/data/x", `100`, 204, ""}, recorder.write},
		{tr{http.MethodPut, "/data/x", `-100`, 204, ""}, recorder.write},
	}

	for _, test := range tests {
		tr := test.tr
		if err := f.v1(tr.method, tr.path, tr.body, tr.code, tr.resp); err != nil {
			t.Fatal(err)
		}

		if test.wait != nil {
			<-test.wait
		}
	}
	recorder.Close()

	exp1 := util.MustUnmarshalJSON([]byte(`[
		{"a": 17},
		{"a": 14},
		{"a": 200},
		{"a": -200}]`))

	stream, err := recorder.consumeQueryResultStream()
	if err != nil {
		t.Fatal(err)
	}

	result := queryResultStreamBindingSet(stream)

	if !reflect.DeepEqual(exp1, result) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", exp1, result)
	}
}

func TestQueryWatchMigrateInvalidate(t *testing.T) {

	f := newFixture(t)

	testPolicy := `
		package z

		r = y { y = data.x }
	`

	if err := f.v1TestRequests([]tr{
		{http.MethodPut, "/data/x", "-100", 204, ""},
		{http.MethodPut, "/policies/foo", testPolicy, 200, ""},
	}); err != nil {
		t.Fatal(err)
	}

	// Test migrating to a new compiler that invalidates a query watch.
	if err := f.v1(http.MethodPut, "/policies/foo", "package z\nr = y { y = data.x }", 200, ""); err != nil {
		t.Fatal(err)
	}

	recorder := newMockConn()
	get := newReqV1(http.MethodGet, `/query?q=a=data.z.r%2Bdata.x&watch`, "")
	go f.server.Handler.ServeHTTP(recorder, get)
	<-recorder.hijacked
	<-recorder.write

	if err := f.v1(http.MethodPut, "/policies/foo", "package z\nr = \"foo\"", 200, ""); err != nil {
		t.Fatal(err)
	}
	<-recorder.write
	<-recorder.write // 2nd read will consume the flush call made by the server.
	recorder.Close()

	stream, err := recorder.consumeQueryResultStream()
	if err != nil {
		t.Fatal(err)
	}

	if stream[0].Result[0].Bindings["a"] != json.Number("-200") {
		t.Fatalf("Expected -200 but got: %v", stream[0].Result[0].Bindings["a"])
	}

	expMsg := "watch invalidated: 1 error occurred: 1:3: rego_type_error: plus: invalid argument(s)\n\thave: (string, any, ???)\n\twant: (number, number, number)"

	if stream[1].Error.Message != expMsg {
		t.Fatalf("Unexpected error: %v", stream[1])
	}
}

func TestDiagnostics(t *testing.T) {
	f := newFixture(t)
	f.server, _ = New().
		WithAddresses([]string{":8182"}).
		WithStore(f.server.store).
		WithManager(f.server.manager).
		WithDiagnosticsBuffer(NewBoundedBuffer(8)).
		Init(context.Background())

	queriesOnly := `package system.diagnostics

	default config = {"mode": "off"}

	config = {"mode": "on"} {
		input.path = "/v1/query"
	}`

	setup := []tr{
		{http.MethodPut, "/data", `{"foo": "bar", "y": 7, "x": [1, 2, 3], "bar": null}`, 204, ""},
		{http.MethodPut, "/policies/main", "package system\nmain = \"foo\"", 200, ""},

		// Diagnostics should be disabled by default.
		{http.MethodGet, "/data/y", "", 200, `{"result":7}`},
		{http.MethodPost, "/data/y", "", 200, `{"result":7}`},
		{http.MethodGet, "/query?q=a=data.y", "", 200, `{"result":[{"a":7}]}`},

		// We should only get back metrics.
		{http.MethodPut, "/policies/diagnostics", "package system.diagnostics\nconfig = {\"mode\": \"on\"}", 200, ""},
		{http.MethodGet, "/data/y", "", 200, `{"result":7}`}, // This one should fall off the ring buffer.
		{http.MethodGet, "/data/x", "", 200, `{"result":[1,2,3]}`},
		{http.MethodPost, "/data/x", `{"input":{"test":"foo"}}`, 200, `{"result":[1,2,3]}`},
		{http.MethodGet, "/query?q=a=data.x", "", 200, `{"result":[{"a":[1,2,3]}]}`},

		// We should get back everything.
		{http.MethodPut, "/policies/diagnostics", "package system.diagnostics\nconfig = {\"mode\": \"all\"}", 200, ""},
		{http.MethodGet, "/data/x", "", 200, `{"result":[1,2,3]}`},
		{http.MethodPost, "/data/z", "", 200, ``},
		{http.MethodGet, "/query?q=a=data.x", "", 200, `{"result":[{"a":[1,2,3]}]}`},

		// We should get back nothing.
		{http.MethodPut, "/policies/diagnostics", "package system.diagnostics\nconfig = {\"mode\": \"off\"}", 200, ""},
		{http.MethodGet, "/data/x", "", 200, `{"result":[1,2,3]}`},
		{http.MethodPost, "/data/x", "", 200, `{"result":[1,2,3]}`},
		{http.MethodGet, "/query?q=a=data.x", "", 200, `{"result":[{"a":[1,2,3]}]}`},

		// We should get back only the query request.
		{http.MethodPut, "/policies/diagnostics", queriesOnly, 200, ""},
		{http.MethodGet, "/data/y", "", 200, `{"result":7}`},
		{http.MethodPost, "/data/y", "", 200, `{"result":7}`},
		{http.MethodGet, "/query?q=a=data.y", "", 200, `{"result":[{"a":7}]}`},

		// We should get back the results of the webhook.
		{http.MethodPut, "/policies/diagnostics", "package system.diagnostics\nconfig = {\"mode\": \"on\"}", 200, ""},
	}

	for _, tr := range setup {

		req := newReqV1(tr.method, tr.path, tr.body)
		req.RemoteAddr = "testaddr"

		if err := f.executeRequest(req, tr.code, tr.resp); err != nil {
			t.Fatal(err)
		}
	}

	get := newReqUnversioned(http.MethodPost, `/`, "")
	if err := f.executeRequest(get, 200, `"foo"`); err != nil {
		t.Fatal(err)
	}

	expList := interface{}([]interface{}{json.Number("1"), json.Number("2"), json.Number("3")})
	expMap1 := interface{}([]interface{}{map[string]interface{}{"a": expList}})
	expMap2 := interface{}([]interface{}{map[string]interface{}{"a": json.Number("7")}})
	expStr := interface{}("foo")

	exp := []struct {
		remoteAddr string
		query      string
		input      interface{}
		result     *interface{}
		metrics    bool
		instrument bool
		explainLen int
	}{
		{
			remoteAddr: "testaddr",
			query:      "data.x",
			result:     &expList,
			metrics:    true,
		},
		{
			query:   "data.x",
			input:   map[string]interface{}{"test": "foo"},
			result:  &expList,
			metrics: true,
		},
		{
			query:   "a=data.x",
			result:  &expMap1,
			metrics: true,
		},
		{
			query:      "data.x",
			result:     &expList,
			metrics:    true,
			instrument: true,
			explainLen: 5,
		},
		{
			query:      "data.z",
			result:     nil,
			metrics:    true,
			instrument: true,
			explainLen: 3,
		},
		{
			query:      "a=data.x",
			result:     &expMap1,
			metrics:    true,
			instrument: true,
			explainLen: 5,
		},
		{
			query:  "a=data.y",
			result: &expMap2,
		},
		{
			query:  "data.system.main",
			result: &expStr,
		},
	}

	get = newReqV1(http.MethodGet, `/data/system/diagnostics`, "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, get)

	var resp types.DiagnosticsResponseV1
	decoder := util.NewJSONDecoder(f.recorder.Body)
	if err := decoder.Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.Result) != len(exp) {
		t.Fatalf("Expected %d diagnostics, got %d", len(exp), len(resp.Result))
	}

	for i, d := range resp.Result {
		test.Subtest(t, fmt.Sprint(i), func(t *testing.T) {
			e := exp[i]
			if e.query != d.Query {
				t.Fatalf("Expected query to be %v, got %v", e.query, d.Query)
			}

			if !reflect.DeepEqual(e.input, d.Input) {
				t.Fatalf("Expected input to be %v, got %v", e.input, d.Input)
			}

			if !reflect.DeepEqual(e.result, d.Result) {
				t.Fatalf("Expected result to be %v but got: %v", e.result, d.Result)
			}

			if e.metrics {
				if len(d.Metrics) == 0 {
					t.Fatal("Expected metrics")
				}
			}

			if e.instrument {
				found := false
				for k := range d.Metrics {
					if strings.Contains(k, "eval_op_plug") {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("Expected to find instrumentation result: %v", d.Metrics)
				}
			}

			var trace types.TraceV1Raw
			if d.Explanation != nil {
				if err := trace.UnmarshalJSON(d.Explanation); err != nil {
					t.Fatal(err)
				}
			}

			if len(trace) != e.explainLen {
				t.Fatalf("Expected explanation of length %d, got %d", e.explainLen, len(trace))
			}
		})
	}
}

func TestMetricsEndpoint(t *testing.T) {

	f := newFixture(t)

	module := `package test

	p = true`

	err := f.v1TestRequests([]tr{
		{"PUT", "/policies/test", module, http.StatusOK, "{}"},
		{"POST", "/data/test/p", "", http.StatusOK, `{"result": true}`},
	})

	if err != nil {
		t.Fatal(err)
	}

	f.reset()

	metricsRequest, err := http.NewRequest("GET", "/metrics", nil)
	if err != nil {
		t.Fatal(err)
	}

	f.server.Handler.ServeHTTP(f.recorder, metricsRequest)

	resp := f.recorder.Body.String()

	expected := []string{
		`http_request_duration_seconds_count{code="200",handler="v1/policies",method="put"} 1`,
		`http_request_duration_seconds_count{code="200",handler="v1/data",method="post"} 1`,
	}

	for _, exp := range expected {
		if !strings.Contains(resp, exp) {
			t.Fatalf("Expected to find %q but got:\n\n%v", exp, resp)
		}
	}

}

func TestDecisionIDs(t *testing.T) {
	f := newFixture(t)
	f.server = f.server.WithDiagnosticsBuffer(NewBoundedBuffer(4))
	ctr := 0

	f.server = f.server.WithDecisionIDFactory(func() string {
		ctr++
		return fmt.Sprint(ctr)
	})

	enableDiagnostics := `
		package system.diagnostics

		config = {"mode": "on"}
	`

	if err := f.v1("PUT", "/policies/test", enableDiagnostics, 200, "{}"); err != nil {
		t.Fatal(err)
	}

	if err := f.v1("GET", "/data/undefined", "", 200, `{"decision_id": "1"}`); err != nil {
		t.Fatal(err)
	}

	if err := f.v1("POST", "/data/undefined", "", 200, `{"decision_id": "2"}`); err != nil {
		t.Fatal(err)
	}

	if err := f.v1("GET", "/data", "", 200, `{"decision_id": "3", "result": {}}`); err != nil {
		t.Fatal(err)
	}

	if err := f.v1("POST", "/data", "", 200, `{"decision_id": "4", "result": {}}`); err != nil {
		t.Fatal(err)
	}

	infos := []*Info{}
	ctr = 0

	f.server.diagnostics.Iter(func(info *Info) {
		ctr++
		if info.DecisionID != fmt.Sprint(ctr) {
			t.Fatalf("Expected decision ID to be %v but got: %v", ctr, info.DecisionID)
		}
		infos = append(infos, info)
	})

	if len(infos) != 4 {
		t.Fatalf("Expected exactly 4 elements but got: %v", infos)
	}
}

func TestDecisonLogging(t *testing.T) {
	f := newFixture(t)
	decisions := []*Info{}
	f.server = f.server.WithDecisionLogger(func(_ context.Context, info *Info) {
		decisions = append(decisions, info)
	})

	if err := f.v1("PUT", "/policies/test", "package system\nmain=true", 200, "{}"); err != nil {
		t.Fatal(err)
	}

	if err := f.v1("POST", "/data", "", 200, `{"result": {}}`); err != nil {
		t.Fatal(err)
	}

	if err := f.v1("GET", "/data", "", 200, `{"result": {}}`); err != nil {
		t.Fatal(err)
	}

	if err := f.v0("POST", "/data", "", 200, `{}`); err != nil {
		t.Fatal(err)
	}

	req := newReqUnversioned("POST", "/", "")

	if err := f.executeRequest(req, 200, "true"); err != nil {
		t.Fatal(err)
	}

	if err := f.v1("GET", "/query?q=data=x", "", 200, `{"result": [{"x": {}}]}`); err != nil {
		t.Fatal(err)
	}

	if len(decisions) != 5 {
		t.Fatalf("Expected exactly 5 decisions but got: %d", len(decisions))
	}
}

func TestWatchParams(t *testing.T) {
	f := newFixture(t)
	r1 := newMockConn()
	r2 := newMockConn()

	if err := f.v1(http.MethodPut, "/data/x", `{"a":1,"b":2}`, 204, ""); err != nil {
		t.Fatal(err)
	}

	get := newReqV1(http.MethodGet, `/query?q=a=data.x&watch&metrics=true&explain=full`, "")
	go f.server.Handler.ServeHTTP(r1, get)
	<-r1.hijacked
	<-r1.write

	get = newReqV1(http.MethodGet, `/query?q=a=data.x&watch&pretty=true`, "")
	go f.server.Handler.ServeHTTP(r2, get)
	<-r2.hijacked
	<-r2.write

	// Test watch metrics and explanations.
	expOne := []struct {
		result        map[string]interface{}
		explainLength int
	}{
		{map[string]interface{}{
			"a": map[string]interface{}{
				"a": json.Number("1"),
				"b": json.Number("2"),
			},
		}, 5},
		{map[string]interface{}{"a": "foo"}, 5},
		{map[string]interface{}{"a": json.Number("7")}, 5},
	}

	// Test watch pretty.
	expTwo := strings.Join([]string{
		"HTTP/1.1 200 OK\nContent-Type: application/json\nTransfer-Encoding: chunked\n\n134",
		`{
  "result": [
    {
      "expressions": [
        {
          "value": true,
          "text": "a=data.x",
          "location": {
            "row": 1,
            "col": 1
          }
        }
      ],
      "bindings": {
        "a": {
          "a": 1,
          "b": 2
        }
      }
    }
  ]
}
`,
		`10b`,
		`{
  "result": [
    {
      "expressions": [
        {
          "value": true,
          "text": "a=data.x",
          "location": {
            "row": 1,
            "col": 1
          }
        }
      ],
      "bindings": {
        "a": "foo"
      }
    }
  ]
}
`,
		`107`,
		`{
  "result": [
    {
      "expressions": [
        {
          "value": true,
          "text": "a=data.x",
          "location": {
            "row": 1,
            "col": 1
          }
        }
      ],
      "bindings": {
        "a": 7
      }
    }
  ]
}
`,
		``,
	}, "\r\n")

	tests := []tr{
		{http.MethodPut, "/data/x", `"foo"`, 204, ""},
		{http.MethodPut, "/data/x", `7`, 204, ""},
	}

	for _, tr := range tests {
		if err := f.v1(tr.method, tr.path, tr.body, tr.code, tr.resp); err != nil {
			t.Fatal(err)
		}
		<-r1.write
		<-r2.write
	}
	r1.Close()
	r2.Close()

	if result := r2.buf.String(); result != expTwo {
		t.Fatalf("Expected stream to equal %s, got %s", expTwo, result)
	}

	// Skip the header
	headerLen := len("HTTP/1.1 200 OK\nContent-Type: application/json\nTransfer-Encoding: chunked\n\n")
	r1.buf.Read(make([]byte, headerLen))

	reader := httputil.NewChunkedReader(&r1.buf)
	decoder := util.NewJSONDecoder(reader)

	metricsKeys := []string{
		"timer_rego_query_parse_ns",
		"timer_rego_query_compile_ns",
		"timer_rego_query_eval_ns",
	}

	for _, exp := range expOne {
		var v interface{}
		if err := decoder.Decode(&v); err != nil {
			t.Fatalf("Failed to decode JSON stream: %v", err)
		}
		m := v.(map[string]interface{})

		met, ok := m["metrics"]
		if !ok {
			t.Fatalf("Expected metrics")
		}
		metrics := met.(map[string]interface{})

		for _, key := range metricsKeys {
			if v, ok := metrics[key]; !ok || v == 0 {
				t.Fatalf("Expected non-zero metric for %v but got: %v", key, v)
			}
		}

		expl, ok := m["explanation"]
		if !ok {
			t.Fatalf("Expected explanation")
		}
		explain := expl.([]interface{})
		if len(explain) != exp.explainLength {
			t.Fatalf("Expected %d explanations, got %d", exp.explainLength, len(explain))
		}

		result, ok := m["result"].([]interface{})[0].(map[string]interface{})["bindings"]
		if !ok {
			t.Fatalf("Expected bindings")
		}
		if !reflect.DeepEqual(exp.result, result) {
			t.Fatalf("Expected bindings %v, got %v", exp.result, result)
		}
	}
}

func TestQueryV1(t *testing.T) {
	f := newFixture(t)
	get := newReqV1(http.MethodGet, `/query?q=a=[1,2,3]%3Ba[i]=x`, "")
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected success but got %v", f.recorder)
	}

	var expected types.QueryResponseV1
	err := util.UnmarshalJSON([]byte(`{
		"result": [{"a":[1,2,3],"i":0,"x":1},{"a":[1,2,3],"i":1,"x":2},{"a":[1,2,3],"i":2,"x":3}]
	}`), &expected)
	if err != nil {
		panic(err)
	}

	var result types.QueryResponseV1
	err = util.UnmarshalJSON(f.recorder.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Unexpected error while unmarshalling result: %v", err)
	}

	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}
}

func TestQueryV1UnsafeBuiltin(t *testing.T) {
	f := newFixture(t)
	get := newReqV1(http.MethodGet, `/query?q=http.send({"method": "get", "url": "foo.com"}, x)`, "")
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 400 {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}

	expected := `{
  "code": "invalid_parameter",
  "message": "unsafe built-in function calls in query: http.send"
}`

	if f.recorder.Body.String() != expected {
		t.Fatalf(`Expected %v but got: %v`, expected, f.recorder.Body.String())
	}
}

func TestUnversionedPost(t *testing.T) {

	f := newFixture(t)

	post := func() *http.Request {
		return newReqUnversioned(http.MethodPost, "/", `
		{
			"foo": {
				"bar": [1,2,3]
			}
		}`)
	}

	f.server.Handler.ServeHTTP(f.recorder, post())

	if f.recorder.Code != 404 {
		t.Fatalf("Expected not found before policy added but got %v", f.recorder)
	}

	module := `
	package system.main

	agg = x {
		sum(input.foo.bar, x)
	}
	`

	if err := f.v1("PUT", "/policies/test", module, 200, ""); err != nil {
		t.Fatal(err)
	}

	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, post())

	expected := `{"agg":6}`
	if f.recorder.Code != 200 || f.recorder.Body.String() != expected {
		t.Fatalf(`Expected HTTP 200 / %v but got: %v`, expected, f.recorder)
	}
}

func TestQueryV1Explain(t *testing.T) {
	f := newFixture(t)
	get := newReqV1(http.MethodGet, `/query?q=a=[1,2,3]%3Ba[i]=x&explain=full`, "")
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected 200 but got: %v", f.recorder)
	}

	var result types.QueryResponseV1

	if err := util.NewJSONDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	explain := mustUnmarshalTrace(result.Explanation)
	if len(explain) != 13 {
		t.Fatalf("Expected exactly 10 trace events for full query but got %d", len(explain))
	}
}

func TestAuthorization(t *testing.T) {

	ctx := context.Background()
	store := inmem.New()
	m, err := plugins.New([]byte{}, "test", store)
	if err != nil {
		panic(err)
	}

	if err := m.Start(ctx); err != nil {
		panic(err)
	}

	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	authzPolicy := `package system.authz

		import input.identity

		default allow = false

		allow {
			identity = "bob"
		}
		`

	if err := store.UpsertPolicy(ctx, txn, "test", []byte(authzPolicy)); err != nil {
		panic(err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}

	server, err := New().
		WithAddresses([]string{":8182"}).
		WithStore(store).
		WithManager(m).
		WithAuthorization(AuthorizationBasic).
		Init(ctx)

	if err != nil {
		panic(err)
	}

	recorder := httptest.NewRecorder()

	// Test that bob can do stuff.
	req1, err := http.NewRequest(http.MethodGet, "http://localhost:8182/v1/data/foo", nil)
	if err != nil {
		panic(err)
	}

	req1 = identifier.SetIdentity(req1, "bob")
	server.Handler.ServeHTTP(recorder, req1)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected success but got: %v", recorder)
	}

	recorder = httptest.NewRecorder()

	// Test that alice can't do stuff.
	req2, err := http.NewRequest(http.MethodGet, "http://localhost:8182/v1/data/foo", nil)
	if err != nil {
		panic(err)
	}

	req2 = identifier.SetIdentity(req2, "alice")
	server.Handler.ServeHTTP(recorder, req2)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("Expected unauthorized but got: %v", recorder)
	}

	// Reverse the policy.
	update := identifier.SetIdentity(newReqV1(http.MethodPut, "/policies/test", `
		package system.authz

		import input.identity

		default allow = false

		allow {
			identity = "alice"
		}
	`), "bob")

	recorder = httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, update)
	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected policy update to succeed but got: %v", recorder)
	}

	// Try alice again.
	recorder = httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, req2)
	if recorder.Code != http.StatusOK {
		t.Fatalf("Expected OK but got: %v", recorder)
	}

	// Try bob again.
	recorder = httptest.NewRecorder()
	server.Handler.ServeHTTP(recorder, req1)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401 but got: %v", recorder)
	}
}

func TestServerReloadTrigger(t *testing.T) {
	f := newFixture(t)
	store := f.server.store
	ctx := context.Background()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	if err := store.UpsertPolicy(ctx, txn, "test", []byte("package test\np = 1")); err != nil {
		panic(err)
	}
	if err := f.v1(http.MethodGet, "/data/test", "", 200, `{}`); err != nil {
		t.Fatalf("Unexpected error from server: %v", err)
	}

	if err := store.Commit(ctx, txn); err != nil {
		panic(err)
	}
	if err := f.v1(http.MethodGet, "/data/test", "", 200, `{"result": {"p": 1}}`); err != nil {
		t.Fatalf("Unexpected error from server: %v", err)
	}
}

type queryBindingErrStore struct {
	storage.WritesNotSupported
	storage.PolicyNotSupported
	storage.IndexingNotSupported
	count int
}

func (s *queryBindingErrStore) Read(ctx context.Context, txn storage.Transaction, path storage.Path) (interface{}, error) {
	return nil, fmt.Errorf("unknown error")
}

func (*queryBindingErrStore) ListPolicies(ctx context.Context, txn storage.Transaction) ([]string, error) {
	return nil, nil
}

func (queryBindingErrStore) NewTransaction(ctx context.Context, params ...storage.TransactionParams) (storage.Transaction, error) {
	return nil, nil
}

func (queryBindingErrStore) Commit(ctx context.Context, txn storage.Transaction) error {
	return nil
}

func (queryBindingErrStore) Abort(ctx context.Context, txn storage.Transaction) {

}

func (queryBindingErrStore) Register(context.Context, storage.Transaction, storage.TriggerConfig) (storage.TriggerHandle, error) {
	return nil, nil
}

func (queryBindingErrStore) Unregister(context.Context, storage.Transaction, string) {

}

func TestQueryBindingIterationError(t *testing.T) {

	ctx := context.Background()
	mock := &queryBindingErrStore{}
	m, err := plugins.New([]byte{}, "test", mock)
	if err != nil {
		panic(err)
	}

	if err := m.Start(ctx); err != nil {
		panic(err)
	}

	server, err := New().WithStore(mock).WithManager(m).WithAddresses([]string{":8182"}).Init(ctx)
	if err != nil {
		panic(err)
	}
	recorder := httptest.NewRecorder()

	f := &fixture{
		server:   server,
		recorder: recorder,
		t:        t,
	}

	get := newReqV1(http.MethodGet, `/query?q=a=data.foo.bar`, "")
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 500 {
		t.Fatalf("Expected 500 error due to unknown storage error but got: %v", f.recorder)
	}
}

const (
	testMod = `package a.b.c

import data.x.y as z
import data.p

q[x] { p[x]; not r[x] }
r[x] { z[x] = 4 }`
)

type fixture struct {
	server   *Server
	recorder *httptest.ResponseRecorder
	t        *testing.T
}

func newFixture(t *testing.T) *fixture {
	ctx := context.Background()
	store := inmem.New()
	m, err := plugins.New([]byte{}, "test", store)
	if err != nil {
		panic(err)
	}

	if err := m.Start(ctx); err != nil {
		panic(err)
	}

	server, err := New().
		WithAddresses([]string{":8182"}).
		WithStore(store).
		WithManager(m).
		Init(ctx)
	if err != nil {
		panic(err)
	}
	recorder := httptest.NewRecorder()

	return &fixture{
		server:   server,
		recorder: recorder,
		t:        t,
	}
}

func (f *fixture) loadResponse() interface{} {
	var v interface{}
	err := util.NewJSONDecoder(f.recorder.Body).Decode(&v)
	if err != nil {
		panic(err)
	}
	return v
}

func (f *fixture) v1TestRequests(trs []tr) error {
	for i, tr := range trs {
		if err := f.v1(tr.method, tr.path, tr.body, tr.code, tr.resp); err != nil {
			return errors.Wrapf(err, "error on test request #%d", i+1)
		}
	}
	return nil
}

func (f *fixture) v1(method string, path string, body string, code int, resp string) error {
	req := newReqV1(method, path, body)
	return f.executeRequest(req, code, resp)
}

func (f *fixture) v0(method string, path string, body string, code int, resp string) error {
	req := newReqV0(method, path, body)
	return f.executeRequest(req, code, resp)
}

func (f *fixture) executeRequest(req *http.Request, code int, resp string) error {
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)
	if f.recorder.Code != code {
		return fmt.Errorf("Expected code %v from %v %v but got: %v", code, req.Method, req.URL, f.recorder)
	}
	if resp != "" {
		var result interface{}
		if err := util.UnmarshalJSON([]byte(f.recorder.Body.String()), &result); err != nil {
			return fmt.Errorf("Expected JSON response from %v %v but got: %v", req.Method, req.URL, f.recorder)
		}
		var expected interface{}
		if err := util.UnmarshalJSON([]byte(resp), &expected); err != nil {
			panic(err)
		}
		if !reflect.DeepEqual(result, expected) {
			a, err := json.MarshalIndent(expected, "", "  ")
			if err != nil {
				panic(err)
			}
			b, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				panic(err)
			}
			return fmt.Errorf("Expected JSON response from %v %v to equal:\n\n%s\n\nGot:\n\n%s", req.Method, req.URL, a, b)
		}
	}
	return nil
}

func (f *fixture) reset() {
	f.recorder = httptest.NewRecorder()
}

func executeRequests(t *testing.T, reqs []tr) {
	f := newFixture(t)
	for i, req := range reqs {
		if err := f.v1(req.method, req.path, req.body, req.code, req.resp); err != nil {
			t.Errorf("Unexpected response on request %d: %v", i+1, err)
		}
	}
}

// Runs through an array of test cases against the v0 REST API tree
func executeRequestsv0(t *testing.T, reqs []tr) {
	f := newFixture(t)
	for i, req := range reqs {
		if err := f.v0(req.method, req.path, req.body, req.code, req.resp); err != nil {
			t.Errorf("Unexpected response on request %d: %v", i+1, err)
		}
	}
}

func newPolicy(id, s string) types.PolicyV1 {
	compiler := ast.NewCompiler()
	parsed := ast.MustParseModule(s)
	if compiler.Compile(map[string]*ast.Module{"": parsed}); compiler.Failed() {
		panic(compiler.Errors)
	}
	mod := compiler.Modules[""]
	return types.PolicyV1{ID: id, AST: mod, Raw: s}
}

func newReqV1(method string, path string, body string) *http.Request {
	return newReq(1, method, path, body)
}

func newReqV0(method string, path string, body string) *http.Request {
	return newReq(0, method, path, body)
}

func newReq(version int, method, path, body string) *http.Request {
	return newReqUnversioned(method, fmt.Sprintf("/v%d", version)+path, body)
}

func newReqUnversioned(method, path, body string) *http.Request {
	req, err := http.NewRequest(method, path, strings.NewReader(body))
	if err != nil {
		panic(err)
	}
	return req
}

func mustUnmarshalTrace(t types.TraceV1) (trace types.TraceV1Raw) {
	if err := json.Unmarshal(t, &trace); err != nil {
		panic("not reached")
	}
	return trace
}

// A mock http.ResponseWriter, http.Hijacker and net.Conn to test watch streams
// Most operations are simple no-ops, except for writes and hijacks.
type mockResponseWriterConn struct {
	t   *testing.T
	exp []byte
	buf bytes.Buffer

	write    chan struct{}
	hijacked chan struct{}
}

func newMockConn() *mockResponseWriterConn {
	return &mockResponseWriterConn{
		write:    make(chan struct{}),
		hijacked: make(chan struct{}),
	}
}

func (m *mockResponseWriterConn) Read(b []byte) (n int, err error) {
	return 0, nil
}

func (m *mockResponseWriterConn) Write(b []byte) (int, error) {
	defer func() {
		m.write <- struct{}{}
	}()
	return m.buf.Write(b)
}

func (m *mockResponseWriterConn) Close() error {
	return nil
}

func (m *mockResponseWriterConn) LocalAddr() net.Addr {
	return nil
}

func (m *mockResponseWriterConn) RemoteAddr() net.Addr {
	return nil
}

func (m *mockResponseWriterConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockResponseWriterConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockResponseWriterConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *mockResponseWriterConn) Header() http.Header {
	return http.Header(map[string][]string{})
}

func (m *mockResponseWriterConn) WriteHeader(code int) {
	m.buf.WriteString(fmt.Sprintf("Code: %d\n", code))
}

func (m *mockResponseWriterConn) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	defer close(m.hijacked)
	return m, bufio.NewReadWriter(bufio.NewReader(m), bufio.NewWriter(m)), nil
}

type queryResultStreamMsg struct {
	Result []struct {
		Bindings map[string]interface{} `json:"bindings"`
	} `json:"result"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
}

func queryResultStreamBindingSet(qs []queryResultStreamMsg) []interface{} {
	result := []interface{}{}
	for i := range qs {
		for j := range qs[i].Result {
			result = append(result, qs[i].Result[j].Bindings)
		}
	}
	return result
}

func (m *mockResponseWriterConn) consumeQueryResultStream() ([]queryResultStreamMsg, error) {
	result := []queryResultStreamMsg{}
	for _, line := range strings.Split(m.buf.String(), "\n") {
		if strings.HasPrefix(line, `{"result":`) {
			var qr queryResultStreamMsg
			err := util.NewJSONDecoder(bytes.NewBufferString(line)).Decode(&qr)
			if err != nil {
				return nil, err
			}
			result = append(result, qr)
		}
	}
	return result, nil
}
