// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
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

func TestDataPatchV1(t *testing.T) {
	f := newFixture(t)
	patch := newReqV1("PATCH", "/data/x", `[{"op": "add", "path": "/", "value": {"a": 1, "b": 2}}]`)
	f.server.Router.ServeHTTP(f.recorder, patch)

	if f.recorder.Code != 204 {
		t.Errorf("Expected success/no-content but got %v", f.recorder)
		return
	}

	get := newReqV1("GET", "/data/x/a", "")
	f.reset()
	f.server.Router.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	resp := f.loadResponse().(float64)
	exp := float64(1)
	if resp != exp {
		t.Errorf("Expected %v but got: %v", exp, resp)
	}
}

func TestDataPatchArrayAccessV1(t *testing.T) {
	f := newFixture(t)
	patch1 := newReqV1("PATCH", "/data/x", `[{"op": "add", "path": "/", "value": {
		"y": [
			{"z": [
				1, 2, 3
			]},
			{"z": [
				4, 5, 6
			]}
		]
	}}]`)

	f.server.Router.ServeHTTP(f.recorder, patch1)

	if f.recorder.Code != 204 {
		t.Errorf("Unexpected error: %v", f.recorder)
		return
	}

	get1 := newReqV1("GET", "/data/x/y/1/z/2", "")
	f.reset()
	f.server.Router.ServeHTTP(f.recorder, get1)

	resp := f.loadResponse().(float64)
	exp1 := float64(6)
	if exp1 != resp {
		t.Errorf("Expected %v but got: %v", exp1, resp)
		return
	}

	patch2 := newReqV1("PATCH", "/data/x/y/1", `[{"op": "add", "path": "/z/1", "value": 100}]`)
	f.reset()
	f.server.Router.ServeHTTP(f.recorder, patch2)

	if f.recorder.Code != 204 {
		t.Errorf("Unexpected error: %v", f.recorder)
		return
	}

	get2 := newReqV1("GET", "/data/x/y/1/z", "")
	f.reset()
	f.server.Router.ServeHTTP(f.recorder, get2)

	if f.recorder.Code != 200 {
		t.Errorf("Unexpected error: %v", f.recorder)
		return
	}

	resp2 := f.loadResponse().([]interface{})
	exp2 := []interface{}{float64(4), float64(100), float64(5), float64(6)}
	if !reflect.DeepEqual(exp2, resp2) {
		t.Errorf("Expected %v but got: %v", exp2, resp2)
		return
	}

}

func TestDataGetVirtualDoc(t *testing.T) {

	f := newFixture(t)
	testMod := `package testmod

	p[x] :- q[x], not r[x]
	q[x] :- data.x.y[i] = x
	r[x] :- data.x.z[i] = x
	`

	put := newReqV1("PUT", "/policies/test", testMod)

	f.server.Router.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Errorf("Expected policy creation to succeed but got: %v", f.recorder)
		return
	}

	f.reset()

	patch1 := newReqV1("PATCH", "/data/x", `[{
		"op": "add",
		"path": "/",
		"value": {"y": [1,2,3,4], "z": [3,4,5,6]}
	}]`)

	f.server.Router.ServeHTTP(f.recorder, patch1)

	if f.recorder.Code != 204 {
		t.Errorf("Expected data patch to succeed but got: %v", f.recorder)
		return
	}

	f.reset()

	get := newReqV1("GET", "/data/testmod/p", "")

	f.server.Router.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Errorf("Expected data get to succeed but got: %v", f.recorder)
		return
	}

	var result interface{}
	err := json.Unmarshal(f.recorder.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Expected JSON response from data get but got: %v", err)
		return
	}

	expected := []interface{}{float64(1), float64(2)}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected response to equal [1,2] but got: %v", result)
		return
	}

}

func TestIndexGet(t *testing.T) {
	f := newFixture(t)
	get, err := http.NewRequest("GET", `/?q=foo = 1`, strings.NewReader(""))
	if err != nil {
		panic(err)
	}
	f.server.Router.ServeHTTP(f.recorder, get)
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

func TestPoliciesPutV1(t *testing.T) {
	f := newFixture(t)
	req := newReqV1("PUT", "/policies/1", testMod)

	f.server.Router.ServeHTTP(f.recorder, req)

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

	f.server.Router.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Errorf("Expected bad request but got %v", f.recorder)
		return
	}
}

func TestPoliciesPutV1ParseError(t *testing.T) {
	f := newFixture(t)
	req := newReqV1("PUT", "/policies/1", `
	package a.b.c

	p[x] %%^ ;-
	`)

	f.server.Router.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Errorf("Expected bad request but got %v", f.recorder)
		return
	}
}

// TODO(tsandall): revisit once safety checks are in place
func testPoliciesPutV1CompileError(t *testing.T) {
	f := newFixture(t)
	req := newReqV1("PUT", "/policies/1", `
	package a.b.c
	p[x] :- q[x]
	q[x] :- p[x]
	`)

	f.server.Router.ServeHTTP(f.recorder, req)

	if f.recorder.Code != 400 {
		t.Errorf("Expected bad request but got %v", f.recorder)
		return
	}
}

func TestPoliciesListV1(t *testing.T) {
	f := newFixture(t)
	put := newReqV1("PUT", "/policies/1", testMod)
	f.server.Router.ServeHTTP(f.recorder, put)
	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}
	f.reset()
	list := newReqV1("GET", "/policies", "")

	f.server.Router.ServeHTTP(f.recorder, list)

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
	f.server.Router.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	f.reset()
	get := newReqV1("GET", "/policies/1", "")

	f.server.Router.ServeHTTP(f.recorder, get)

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
	f.server.Router.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	f.reset()
	get := newReqV1("GET", "/policies/1/raw", "")

	f.server.Router.ServeHTTP(f.recorder, get)

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
	f.server.Router.ServeHTTP(f.recorder, put)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	f.reset()
	del := newReqV1("DELETE", "/policies/1", "")

	f.server.Router.ServeHTTP(f.recorder, del)

	if f.recorder.Code != 204 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	f.reset()
	get := newReqV1("GET", "/policies/1", "")
	f.server.Router.ServeHTTP(f.recorder, get)
	if f.recorder.Code != 404 {
		t.Errorf("Expected not found but got %v", f.recorder)
		return
	}
}

func TestQueryV1(t *testing.T) {
	f := newFixture(t)
	get := newReqV1("GET", `/query?q=a=[1,2,3],a[i]=x`, "")
	f.server.Router.ServeHTTP(f.recorder, get)

	if f.recorder.Code != 200 {
		t.Errorf("Expected success but got %v", f.recorder)
		return
	}

	var expected resultSetV1
	err := json.Unmarshal([]byte(`[{"a":[1,2,3],"i":0,"x":1},{"a":[1,2,3],"i":1,"x":2},{"a":[1,2,3],"i":2,"x":3}]`), &expected)
	if err != nil {
		panic(err)
	}

	var result resultSetV1
	err = json.Unmarshal(f.recorder.Body.Bytes(), &result)
	if err != nil {
		t.Errorf("Unexpected error while unmarshalling result: %v", err)
		return
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v but got: %v", expected, result)
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
	runtime  *Runtime
	server   *Server
	recorder *httptest.ResponseRecorder
	t        *testing.T
}

func newFixture(t *testing.T) *fixture {
	runtime := &Runtime{}
	runtime.Init(&Params{Server: true, PolicyDir: policyDir})
	server := NewServer(runtime, ":8182", false)
	recorder := httptest.NewRecorder()
	return &fixture{
		runtime:  runtime,
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

func (f *fixture) reset() {
	f.recorder = httptest.NewRecorder()
}

func newPolicy(id, s string) *policyV1 {
	compiler := ast.NewCompiler()
	parsed := ast.MustParseModule(s)
	if compiler.Compile(map[string]*ast.Module{"": parsed}); compiler.Failed() {
		panic(compiler.FlattenErrors())
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
