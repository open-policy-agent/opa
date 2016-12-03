// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util/test"
)

var policyDir string

// TestMain creates a temporary direcotry for the server to
// save policies to. The directory name is stored in policyDir
// and is used by the newFixture function.
func TestMain(m *testing.M) {
	d, err := ioutil.TempDir("", "server_test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(d)
	policyDir = d
	rc := m.Run()
	os.Exit(rc)
}

type tr struct {
	method string
	path   string
	body   string
	code   int
	resp   string
}

func TestDataV1(t *testing.T) {

	testMod1 := `package testmod
                p[x] :- q[x], not r[x]
                q[x] :- data.x.y[i] = x
                r[x] :- data.x.z[i] = x

				import req1
				import req2 as reqx
				import req3.attr1
				g :- req1.a[0] = 1, reqx.b[i] = 1
				h :- attr1[i] > 1

				gt1 :- req1 > 1
				arr = [1,2,3,4]

				undef :- false
				`

	testMod2 := `package testmod

	p = [1,2,3,4]
	q = {"a": 1, "b": 2}
	`

	tests := []struct {
		note string
		reqs []tr
	}{
		{"add root", []tr{
			tr{"PATCH", "/data/x", `[{"op": "add", "path": "/", "value": {"a": 1}}]`, 204, ""},
			tr{"GET", "/data/x/a", "", 200, "1"},
		}},
		{"append array", []tr{
			tr{"PATCH", "/data/x", `[{"op": "add", "path": "/", "value": []}]`, 204, ""},
			tr{"PATCH", "/data/x", `[{"op": "add", "path": "-", "value": {"a": 1}}]`, 204, ""},
			tr{"PATCH", "/data/x", `[{"op": "add", "path": "-", "value": {"a": 2}}]`, 204, ""},
			tr{"GET", "/data/x/0/a", "", 200, "1"},
			tr{"GET", "/data/x/1/a", "", 200, "2"},
		}},
		{"append array one-shot", []tr{
			tr{"PATCH", "/data/x", `[
                {"op": "add", "path": "/", "value": []},
                {"op": "add", "path": "-", "value": {"a": 1}},
                {"op": "add", "path": "-", "value": {"a": 2}}
            ]`, 204, ""},
			tr{"GET", "/data/x/1/a", "", 200, "2"},
		}},
		{"insert array", []tr{
			tr{"PATCH", "/data/x", `[{"op": "add", "path": "/", "value": {
                "y": [
                    {"z": [1,2,3]},
                    {"z": [4,5,6]}
                ]
            }}]`, 204, ""},
			tr{"GET", "/data/x/y/1/z/2", "", 200, "6"},
			tr{"PATCH", "/data/x/y/1", `[{"op": "add", "path": "/z/1", "value": 100}]`, 204, ""},
			tr{"GET", "/data/x/y/1/z", "", 200, "[4, 100, 5, 6]"},
		}},
		{"patch root", []tr{
			tr{"PATCH", "/data", `[
				{"op": "add",
				 "path": "/",
				 "value": {"a": 1, "b": 2}
				}
			]`, 204, ""},
			tr{"GET", "/data", "", 200, `{"a": 1, "b": 2}`},
		}},
		{"patch invalid", []tr{
			tr{"PATCH", "/data", `[
				{
					"op": "remove",
					"path": "/"
				}
			]`, 400, ""},
		}},
		{"put root", []tr{
			tr{"PUT", "/data", `{"foo": [1,2,3]}`, 204, ""},
			tr{"GET", "/data", "", 200, `{"foo": [1,2,3]}`},
		}},
		{"put deep makedir", []tr{
			tr{"PUT", "/data/a/b/c/d", `1`, 204, ""},
			tr{"GET", "/data/a/b/c", "", 200, `{"d": 1}`},
		}},
		{"put deep makedir partial", []tr{
			tr{"PUT", "/data/a/b", `{}`, 204, ""},
			tr{"PUT", "/data/a/b/c/d", `0`, 204, ""},
			tr{"GET", "/data/a/b/c", "", 200, `{"d": 0}`},
		}},
		{"put exists overwrite", []tr{
			tr{"PUT", "/data/a/b/c", `"hello"`, 204, ""},
			tr{"PUT", "/data/a/b", `"goodbye"`, 204, ""},
			tr{"GET", "/data/a", "", 200, `{"b": "goodbye"}`},
		}},
		{"put base write conflict", []tr{
			tr{"PUT", "/data/a/b", `[1,2,3,4]`, 204, ""},
			tr{"PUT", "/data/a/b/c/d", "0", 404, `{
				"Code": 404,
				"Message": "write conflict: /a/b"
			}`},
		}},
		{"put virtual write conflict", []tr{
			tr{"PUT", "/policies/test", testMod2, 200, ""},
			tr{"PUT", "/data/testmod/q/x", "0", 404, `{
				"Code": 404,
				"Message": "write conflict: /testmod/q"
			}`},
		}},
		{"get virtual", []tr{
			tr{"PUT", "/policies/test", testMod1, 200, ""},
			tr{"PATCH", "/data/x", `[{"op": "add", "path": "/", "value": {"y": [1,2,3,4], "z": [3,4,5,6]}}]`, 204, ""},
			tr{"GET", "/data/testmod/p", "", 200, "[1,2]"},
		}},
		{"patch virtual error", []tr{
			tr{"PUT", "/policies/test", testMod1, 200, ""},
			tr{"PATCH", "/data/testmod/p", `[{"op": "add", "path": "-", "value": 1}]`, 404, `{
                "Code": 404,
                "Message": "write conflict: /testmod/p"
            }`},
		}},
		{"get with global", []tr{
			tr{"PUT", "/policies/test", testMod1, 200, ""},
			tr{"GET", "/data/testmod/g?global=req1%3A%7B%22a%22%3A%5B1%5D%7D&global=req2%3A%7B%22b%22%3A%5B0%2C1%5D%7D", "", 200, "true"},
		}},
		{"get with global (unbound error)", []tr{
			tr{"PUT", "/policies/test", testMod1, 200, ""},
			tr{"GET", "/data/testmod/g?global=req1%3A%7B%22a%22%3A%5B1%5D%7D", "", 400, `{
				"Code": 400,
				"Message": "evaluation error (code: 1): unbound variable req2: req2.b[i]"
			}`},
		}},
		{"get with global (namespaced)", []tr{
			tr{"PUT", "/policies/test", testMod1, 200, ""},
			tr{"GET", "/data/testmod/h?global=req3.attr1%3A%5B4%2C3%2C2%2C1%5D", "", 200, `true`},
		}},
		{"get with global (non-ground ref)", []tr{
			tr{"PUT", "/policies/test", testMod1, 200, ""},
			tr{"GET", "/data/testmod/gt1?global=req1:data.testmod.arr[i]", "", 200, `[[true, {"i": 1}], [true, {"i": 2}], [true, {"i": 3}]]`},
		}},
		{"get with global (bad format)", []tr{
			tr{"GET", "/data/deadbeef?global=[1,2,3]", "", 400, `{
				"Code": 400,
				"Message": "global format: <path>:<value> where <path> is either var or ref"
			}`},
		}},
		{"get undefined", []tr{
			tr{"PUT", "/policies/test", testMod1, 200, ""},
			tr{"GET", "/data/testmod/undef", "", 404, ""},
			tr{"GET", "/data/does/not/exist", "", 404, ""},
		}},
		{"get root", []tr{
			tr{"PUT", "/policies/test", testMod2, 200, ""},
			tr{"PATCH", "/data/x", `[{"op": "add", "path": "/", "value": [1,2,3,4]}]`, 204, ""},
			tr{"GET", "/data", "", 200, `{"testmod": {"p": [1,2,3,4], "q": {"a":1, "b": 2}}, "x": [1,2,3,4]}`},
		}},
		{"query wildcards omitted", []tr{
			tr{"PATCH", "/data/x", `[{"op": "add", "path": "/", "value": [1,2,3,4]}]`, 204, ""},
			tr{"GET", "/query?q=data.x[_]%20=%20x", "", 200, `[{"x": 1}, {"x": 2}, {"x": 3}, {"x": 4}]`},
		}},
		{"query compiler error", []tr{
			tr{"GET", "/query?q=x", "", 400, ""},
			// Subsequent query should not fail.
			tr{"GET", "/query?q=x=1", "", 200, `[{"x": 1}]`},
		}},
	}

	for _, tc := range tests {
		test.Subtest(t, tc.note, func(t *testing.T) {
			executeRequests(t, tc.reqs)
		})
	}
}

func TestDataPutV1IfNoneMatch(t *testing.T) {
	f := newFixture(t)
	if err := f.v1("PUT", "/data/a/b/c", "0", 204, ""); err != nil {
		t.Fatalf("Unexpected error from PUT /data/a/b/c: %v", err)
	}
	req := newReqV1("PUT", "/data/a/b/c", "1")
	req.Header.Set("If-None-Match", "*")
	if err := f.executeRequest(req, 304, ""); err != nil {
		t.Fatalf("Unexpected error from PUT with If-None-Match=*: %v", err)
	}
}

func TestDataGetExplainFull(t *testing.T) {
	f := newFixture(t)

	f.v1("PUT", "/data/x", `{"a":1,"b":2}`, 204, "")

	req := newReqV1("GET", "/data/x?explain=full", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result traceV1

	if err := json.NewDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected exactly 3 events but got %d", len(result))
	}

	_, ok := result[2].Node.(ast.Body)
	if !ok {
		t.Fatalf("Expected body for node but got: %v", result[2].Node)
	}

	if len(result[2].Locals) != 1 {
		t.Fatalf("Expected one binding but got: %v", result[2].Locals)
	}

	req = newReqV1("GET", "/data/deadbeef?explain=full", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	result = traceV1{}

	if f.recorder.Code != 404 {
		t.Fatalf("Expected status code to be 404 but got: %v", f.recorder.Code)
	}

	if err := json.NewDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("Expected exactly 3 events but got %d", len(result))
	}

	if result[2].Op != "Fail" {
		t.Fatalf("Expected last event to be 'Fail' but got: %v", result[2])
	}

}

func TestDataGetExplainTruth(t *testing.T) {
	f := newFixture(t)

	f.v1("PUT", "/policies/test", `package test
	p :- a = [1,2,3,4], a[_] = x, x > 1
	`, 204, "")

	req := newReqV1("GET", "/data/test/p?explain=truth", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	var result traceV1

	if err := json.NewDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if len(result) != 8 {
		t.Fatalf("Expected exactly 9 events but got %d", len(result))
	}

	req = newReqV1("GET", "/data/deadbeef?explain=truth", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 404 {
		t.Fatalf("Expected status code to be 404 but got: %v", f.recorder)
	}
}

func TestV1Pretty(t *testing.T) {

	f := newFixture(t)
	f.v1("PATCH", "/data/x", `[{"op": "add", "path":"/", "value": [1,2,3,4]}]`, 204, "")

	req := newReqV1("GET", "/data/x?pretty=true", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	lines := strings.Split(f.recorder.Body.String(), "\n")
	if len(lines) != 6 {
		t.Errorf("Expected 5 lines in output but got %d:\n%v", len(lines), lines)
	}

	req = newReqV1("GET", "/query?q=data.x[i]&pretty=true", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)

	lines = strings.Split(f.recorder.Body.String(), "\n")
	if len(lines) != 14 {
		t.Errorf("Expected 14 lines of output but got %d:\n%v", len(lines), lines)
	}
}

func TestIndexGet(t *testing.T) {
	f := newFixture(t)
	get, err := http.NewRequest("GET", `/?q=foo = 1`, strings.NewReader(""))
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
	get, err := http.NewRequest("GET", `/?q=foo`, strings.NewReader(""))
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

func TestPoliciesPutV1(t *testing.T) {
	f := newFixture(t)
	req := newReqV1("PUT", "/policies/1", testMod)

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	policy := f.loadPolicy()
	expected := newPolicy("1", testMod)
	if !expected.Equal(policy) {
		t.Errorf("Expected policies to be equal. Expected:\n\n%v\n\nGot:\n\n%v\n", expected, policy)
	}
}

func TestPoliciesPutV1Empty(t *testing.T) {
	f := newFixture(t)
	req := newReqV1("PUT", "/policies/1", "")

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Errorf("Expected bad request but got %v", f.recorder)
		return
	}
}

func TestPoliciesPutV1ParseError(t *testing.T) {
	f := newFixture(t)
	req := newReqV1("PUT", "/policies/test", `
    package a.b.c

    p ;- true
    `)

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Errorf("Expected bad request but got %v", f.recorder)
		return
	}

	errs := astErrorV1{}
	if err := json.NewDecoder(f.recorder.Body).Decode(&errs); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	expected := ast.NewLocation(nil, "test", 4, 8)

	if !reflect.DeepEqual(errs.Errors[0].Location, expected) {
		t.Errorf("Expected error location to be %v but got: %v", expected, errs)
	}
}

func TestPoliciesPutV1CompileError(t *testing.T) {
	f := newFixture(t)
	req := newReqV1("PUT", "/policies/test", `
    package a.b.c
    p[x] :- q[x]
    q[x] :- p[x]
    `)

	f.server.Handler.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Fatalf("Expected bad request but got %v", f.recorder)
	}

	errs := astErrorV1{}
	if err := json.NewDecoder(f.recorder.Body).Decode(&errs); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	expected := ast.NewLocation(nil, "test", 3, 5)

	if len(errs.Errors) != 2 {
		t.Fatalf("Expected exactly two errors but got %d: %v", len(errs.Errors), errs)
	}

	found := false

	for _, err := range errs.Errors {
		if reflect.DeepEqual(err.Location, expected) {
			found = true
		}
	}

	if !found {
		t.Fatalf("Missing expected error %v: %v", expected, errs)
	}
}

func TestPoliciesListV1(t *testing.T) {
	f := newFixture(t)
	put := newReqV1("PUT", "/policies/1", testMod)
	f.server.Handler.ServeHTTP(f.recorder, put)
	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}
	f.reset()
	list := newReqV1("GET", "/policies", "")

	f.server.Handler.ServeHTTP(f.recorder, list)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	var policies []*policyV1
	err := json.NewDecoder(f.recorder.Body).Decode(&policies)
	if err != nil {
		t.Errorf("Expected policy list but got error: %v", err)
		return
	}

	expected := []*policyV1{
		newPolicy("1", testMod),
	}
	if len(expected) != len(policies) {
		t.Errorf("Expected %d policies but got: %v", len(expected), policies)
		return
	}
	for i := range expected {
		if !expected[i].Equal(policies[i]) {
			t.Errorf("Expected policies to be equal. Expected:\n\n%v\n\nGot:\n\n%v\n", expected[i], policies[i])
		}
	}
}

func TestPoliciesGetV1(t *testing.T) {
	f := newFixture(t)
	put := newReqV1("PUT", "/policies/1", testMod)
	f.server.Handler.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	f.reset()
	get := newReqV1("GET", "/policies/1", "")

	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	policy := f.loadPolicy()
	expected := newPolicy("1", testMod)
	if !expected.Equal(policy) {
		t.Errorf("Expected policies to be equal. Expected:\n\n%v\n\nGot:\n\n%v\n", expected, policy)
	}
}

func TestPoliciesGetRawV1(t *testing.T) {
	f := newFixture(t)
	put := newReqV1("PUT", "/policies/1", testMod)
	f.server.Handler.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	f.reset()
	get := newReqV1("GET", "/policies/1/raw", "")

	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	raw := f.recorder.Body.String()
	if raw != testMod {
		t.Errorf("Expected raw string to equal testMod:\n\nExpected:\n\n%v\n\nGot:\n\n%v\n", testMod, raw)
	}

}

func TestPoliciesDeleteV1(t *testing.T) {
	f := newFixture(t)
	put := newReqV1("PUT", "/policies/1", testMod)
	f.server.Handler.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	f.reset()
	del := newReqV1("DELETE", "/policies/1", "")

	f.server.Handler.ServeHTTP(f.recorder, del)

	if f.recorder.Code != 204 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	f.reset()
	get := newReqV1("GET", "/policies/1", "")
	f.server.Handler.ServeHTTP(f.recorder, get)
	if f.recorder.Code != 404 {
		t.Errorf("Expected not found but got %v", f.recorder)
		return
	}
}

func TestQueryV1(t *testing.T) {
	f := newFixture(t)
	get := newReqV1("GET", `/query?q=a=[1,2,3],a[i]=x`, "")
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	var expected adhocQueryResultSetV1
	err := json.Unmarshal([]byte(`[{"a":[1,2,3],"i":0,"x":1},{"a":[1,2,3],"i":1,"x":2},{"a":[1,2,3],"i":2,"x":3}]`), &expected)
	if err != nil {
		panic(err)
	}

	var result adhocQueryResultSetV1
	err = json.Unmarshal(f.recorder.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Unexpected error while unmarshalling result: %v", err)
		return
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v but got: %v", expected, result)
	}
}

func TestQueryV1Explain(t *testing.T) {
	f := newFixture(t)
	get := newReqV1("GET", `/query?q=a=[1,2,3],a[i]=x&explain=full`, "")
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected 200 but got: %v", f.recorder)
	}

	var result traceV1

	if err := json.NewDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if len(result) != 10 {
		t.Fatalf("Expected exactly 10 trace events for full query but got %d", len(result))
	}

	get = newReqV1("GET", "/query?q=a=[1,2,3],a[_]=x,x>1&explain=truth", "")
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Fatalf("Expected 200 but got: %v", f.recorder)
	}

	result = traceV1{}

	if err := json.NewDecoder(f.recorder.Body).Decode(&result); err != nil {
		t.Fatalf("Unexpected JSON decode error: %v", err)
	}

	if len(result) != 5 {
		t.Fatalf("Expected exactly 5 trace events for truth query but got %d", len(result))
	}
}

type queryBindingErrStore struct {
	storage.TriggersNotSupported
	storage.WritesNotSupported
	count int
}

func (queryBindingErrStore) ID() string {
	return "mock"
}

func (s *queryBindingErrStore) Read(txn storage.Transaction, path storage.Path) (interface{}, error) {
	// At this time, the store will receive two reads:
	// - The first during evaluation
	// - The second when the server tries to accumulate the bindings
	s.count++
	if s.count == 2 {
		return nil, fmt.Errorf("unknown error")
	}
	return "", nil
}

func (queryBindingErrStore) Begin(txn storage.Transaction, params storage.TransactionParams) error {
	return nil
}

func (queryBindingErrStore) Close(txn storage.Transaction) {

}

func TestQueryBindingIterationError(t *testing.T) {

	store := storage.New(storage.InMemoryConfig())
	mock := &queryBindingErrStore{}

	if err := store.Mount(mock, storage.MustParsePath("/foo/bar")); err != nil {
		panic(err)
	}

	server, err := New(store, ":8182", false)
	if err != nil {
		panic(err)
	}
	recorder := httptest.NewRecorder()

	f := &fixture{
		server:   server,
		recorder: recorder,
		t:        t,
	}

	get := newReqV1("GET", `/query?q=a=data.foo.bar`, "")
	f.server.Handler.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 500 {
		t.Fatalf("Expected 500 error due to unknown storage error but got: %v", f.recorder)
	}
}

const (
	testMod = `
    package a.b.c
    import data.x.y as z
    import data.p
    q[x] :- p[x], not r[x]
    r[x] :- z[x] = 4`
)

type fixture struct {
	server   *Server
	recorder *httptest.ResponseRecorder
	t        *testing.T
}

func newFixture(t *testing.T) *fixture {

	store := storage.New(storage.InMemoryConfig().WithPolicyDir(policyDir))
	server, err := New(store, ":8182", false)
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

func (f *fixture) loadPolicy() *policyV1 {
	policy := &policyV1{}
	err := json.NewDecoder(f.recorder.Body).Decode(policy)
	if err != nil {
		panic(err)
	}
	return policy
}

func (f *fixture) loadResponse() interface{} {
	var v interface{}
	err := json.NewDecoder(f.recorder.Body).Decode(&v)
	if err != nil {
		panic(err)
	}
	return v
}

func (f *fixture) v1(method string, path string, body string, code int, resp string) error {
	req := newReqV1(method, path, body)
	return f.executeRequest(req, code, resp)
}

func (f *fixture) executeRequest(req *http.Request, code int, resp string) error {
	f.reset()
	f.server.Handler.ServeHTTP(f.recorder, req)
	if f.recorder.Code != code {
		return fmt.Errorf("Expected code %v from %v %v but got: %v", req.Method, code, req.URL, f.recorder)
	}
	if resp != "" {
		var result interface{}
		if err := json.Unmarshal([]byte(f.recorder.Body.String()), &result); err != nil {
			return fmt.Errorf("Expected JSON response from %v %v but got: %v", req.Method, req.URL, f.recorder)
		}
		var expected interface{}
		if err := json.Unmarshal([]byte(resp), &expected); err != nil {
			panic(err)
		}
		if !reflect.DeepEqual(result, expected) {
			return fmt.Errorf("Expected JSON response from %v %v to equal %v but got: %v", req.Method, req.URL, expected, result)
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

func newPolicy(id, s string) *policyV1 {
	compiler := ast.NewCompiler()
	parsed := ast.MustParseModule(s)
	if compiler.Compile(map[string]*ast.Module{"": parsed}); compiler.Failed() {
		panic(compiler.Errors)
	}
	mod := compiler.Modules[""]
	return &policyV1{ID: id, Module: mod}
}

func newReqV1(method string, path string, body string) *http.Request {
	req, err := http.NewRequest(method, "/v1"+path, strings.NewReader(body))
	if err != nil {
		panic(err)
	}
	return req
}
