// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/format"

	"github.com/ghodss/yaml"

	iCache "github.com/open-policy-agent/opa/topdown/cache"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
	testutil "github.com/open-policy-agent/opa/util/test"
)

func TestTopDownCompleteDoc(t *testing.T) {
	tests := []struct {
		note     string
		rule     string
		expected interface{}
	}{
		{"undefined", `p = null { false }`, ""}, // "" will be converted to Undefined
		{"null", `p = null { true }`, "null"},
		{"bool: true", `p = true { true }`, "true"},
		{"bool: false", `p = false { true }`, "false"},
		{"number: 3", `p = 3 { true }`, "3"},
		{"number: 3.0", `p = 3 { true }`, "3"},
		{"number: 66.66667", `p = 66.66667 { true }`, "66.66667"},
		{`string: "hello"`, `p = "hello" { true }`, `"hello"`},
		{`string: ""`, `p = "" { true }`, `""`},
		{"array: [1,2,3,4]", `p = [1, 2, 3, 4] { true }`, "[1,2,3,4]"},
		{"array: []", `p = [] { true }`, "[]"},
		{`object/nested composites: {"a": [1], "b": [2], "c": [3]}`,
			`p = {"a": [1], "b": [2], "c": [3]} { true }`,
			`{"a": [1], "b": [2], "c": [3]}`},
		{"object/non-string key:", `p = {1: 2, {3: 4}: 5} { true }`, `{"1": 2, "{\"3\":4}": 5}`},
		{"set/nested: {{1,2},{2,3}}", `p = {{1, 2}, {2, 3}} { true }`, "[[1,2], [2,3]]"},
		{"vars", `p = {"a": [x, y]} { x = 1; y = 2 }`, `{"a": [1,2]}`},
		{"vars conflict", `p = {"a": [x, y]} { xs = [1, 2]; ys = [1, 2]; x = xs[_]; y = ys[_] }`,
			completeDocConflictErr(nil)},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownQueryIDsUnique(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	inputTerm := &ast.Term{}
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	compiler := compileModules([]string{
		`package x
  p { 1 }
  p { 2 }`})

	tr := []*Event{}

	query := NewQuery(ast.MustParseBody("data.x.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer((*BufferTracer)(&tr)).
		WithInput(inputTerm)

	_, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	queryIDs := map[uint64]bool{} // set of seen queryIDs (in EnterOps)
	for _, evt := range tr {
		if evt.Op != EnterOp {
			continue
		}
		if queryIDs[evt.QueryID] {
			t.Errorf("duplicate queryID: %v", evt)
		}
		queryIDs[evt.QueryID] = true
	}
}

func TestTopDownIndexExpr(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	compiler := compileModules([]string{
		`package test

		p = true {
		     1 > 0
		     q
		}

		q = true { true }`})

	tr := []*Event{}

	query := NewQuery(ast.MustParseBody("data.test.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer((*BufferTracer)(&tr))

	_, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	exp := []*ast.Expr{
		ast.MustParseExpr("data.test.p"),
		ast.MustParseExpr("data.test.q"),
	}

	i := 0
	for _, evt := range tr {
		if evt.Op != IndexOp {
			continue
		}

		expr, ok := evt.Node.(*ast.Expr)
		if !ok {
			t.Fatal("Expected expr node but got:", evt.Node)
		}

		exp[i].Index = i
		if ast.Compare(expr, exp[i]) != 0 {
			t.Fatalf("Expected %v but got: %v", exp[i], expr)
		}
		i++
	}
}

func TestTopDownPartialSetDoc(t *testing.T) {

	tests := []struct {
		note     string
		rule     string
		expected string
	}{
		{"array values", `p[x] { a[i] = x }`, `[1, 2, 3, 4]`},
		{"array indices", `p[x] { a[x] = _ }`, `[0, 1, 2, 3]`},
		{"object keys", `p[x] { b[x] = _ }`, `["v1", "v2"]`},
		{"object values", `p[x] { b[i] = x }`, `["hello", "goodbye"]`},
		{"nested composites", `p[x] { f[i] = x }`, `[{"xs": [1.0], "ys": [2.0]}, {"xs": [2.0], "ys": [3.0]}]`},
		{"deep ref/heterogeneous", `p[x] { c[i][j][k] = x }`, `[null, 3.14159, false, true, "foo"]`},
		{"composite var value", `p[x] { x = [i, a[i]] }`, "[[0,1],[1,2],[2,3],[3,4]]"},
		{"composite key", `p[[x, {"y": y}]] { x = 1; y = 2 }`, `[[1,{"y": 2}]]`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownPartialObjectDoc(t *testing.T) {
	tests := []struct {
		note     string
		rule     string
		expected interface{}
	}{
		{"identity", `p[k] = v { b[k] = v }`, `{"v1": "hello", "v2": "goodbye"}`},
		{"composites", `p[k] = v { d[k] = v }`, `{"e": ["bar", "baz"]}`},
		{"body/join var", `p[k] = v { a[i] = v; g[k][i] = v }`, `{"a": 1, "b": 2, "c": 4}`},
		{"composite value", `p[k] = [v1, {"v2": v2}] { g[k] = x; x[v1] = v2; v2 != 0 }`, `{
			"a": [0, {"v2": 1}],
			"b": [1, {"v2": 2}],
			"c": [3, {"v2": 4}]
		}`},
		{"same key/value pair", `p[k] = 1 { ks = ["a", "b", "c", "a"]; ks[_] = k }`, `{"a":1,"b":1,"c":1}`},
		{"non-string key", `p[k] = 1 { ks = [1,{},null]; ks[_] = k }`, `{"1": 1, "{}": 1, "null": 1}`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownEvalTermExpr(t *testing.T) {

	tests := []struct {
		note     string
		rule     string
		expected string
	}{
		{"true", `p = true { true }`, "true"},
		{"false", `p = true { false }`, ""},
		{"number non-zero", `p = true { -3.14 }`, "true"},
		{"number zero", `p = true { null }`, "true"},
		{"null", `p = true { null }`, "true"},
		{"string non-empty", `p = true { "abc" }`, "true"},
		{"string empty", `p = true { "" }`, "true"},
		{"array non-empty", `p = true { [1, 2, 3] }`, "true"},
		{"array empty", `p = true { [] }`, "true"},
		{"object non-empty", `p = true { {"a": 1} }`, "true"},
		{"object empty", `p = true { {} }`, "true"},
		{"set non-empty", `p = true { {1, 2, 3} }`, "true"},
		{"set empty", `p = true { set() }`, "true"},
		{"ref", `p = true { a[i] }`, "true"},
		{"ref undefined", `p = true { data.deadbeef[i] }`, ""},
		{"ref undefined (path)", `p = true { data.a[true] }`, ""},
		{"ref false", `p = true { data.c[0].x[1] }`, ""},
		{"array comprehension", `p = true { [x | x = 1] }`, "true"},
		{"array comprehension empty", `p = true { [x | x = 1; x = 2] }`, "true"},
		{"arbitrary position", `p = true { a[i] = x; x; i }`, "true"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownEqExpr(t *testing.T) {

	tests := []struct {
		note     string
		rule     string
		expected interface{}
	}{
		// undefined cases
		{"undefined: same type", `p = true { true = false }`, ""},
		{"undefined: array order", `p = true { [1, 2, 3] = [1, 3, 2] }`, ""},
		{"undefined: ref value", `p = true { a[3] = 9999 }`, ""},
		{"undefined: ref values", `p = true { a[i] = 9999 }`, ""},
		{"undefined: ground var", `p = true { a[3] = x; x = 3 }`, ""},
		{"undefined: array var 1", `p = true { [1, x, x] = [1, 2, 3] }`, ""},
		{"undefined: array var 2", `p = true { [1, x, 3] = [1, 2, x] }`, ""},
		{"undefined: object var 1", `p = true { {"a": 1, "b": 2} = {"a": a, "b": a} }`, ""},
		{"undefined: array deep var 1", `p = true { [[1, x], [3, x]] = [[1, 2], [3, 4]] }`, ""},
		{"undefined: array deep var 2", `p = true { [[1, x], [3, 4]] = [[1, 2], [x, 4]] }`, ""},
		{"undefined: set", `p = true { {1, 2, 3} = {1, 2, 4} }`, ""},

		// ground terms
		{"ground: bool", `p = true { true = true }`, "true"},
		{"ground: string", `p = true { "string" = "string" }`, "true"},
		{"ground: number", `p = true { 17 = 17 }`, "true"},
		{"ground: null", `p = true { null = null }`, "true"},
		{"ground: array", `p = true { [1, 2, 3] = [1, 2, 3] }`, "true"},
		{"ground: set", `p = true { {1, 2, 3} = {3, 2, 1} }`, "true"},
		{"ground: object", `p = true { {"b": false, "a": [1, 2, 3]} = {"a": [1, 2, 3], "b": false} }`, "true"},
		{"ground: ref 1", `p = true { a[2] = 3 }`, "true"},
		{"ground: ref 2", `p = true { b.v2 = "goodbye" }`, "true"},
		{"ground: ref 3", `p = true { d.e = ["bar", "baz"] }`, "true"},
		{"ground: ref 4", `p = true { c[0].x[1] = c[0].z.q }`, "true"},

		// variables
		{"var: x=y=z", `p[x] { x = y; z = 42; y = z }`, "[42]"},
		{"var: ref value", `p = true { a[3] = x; x = 4 }`, "true"},
		{"var: ref values", `p = true { a[i] = x; x = 2 }`, "true"},
		{"var: ref key", `p = true { a[i] = 4; x = 3 }`, "true"},
		{"var: ref keys", `p = true { a[i] = x; i = 2 }`, "true"},
		{"var: ref ground var", `p[x] { i = 2; a[i] = x }`, "[3]"},
		{"var: ref ref", `p[x] { c[0].x[i] = c[0].z[j]; x = [i, j] }`, `[[0, "p"], [1, "q"]]`},

		// arrays and variables
		{"pattern: array", `p[x] { [1, x, 3] = [1, 2, 3] }`, "[2]"},
		{"pattern: array 2", `p[x] { [[1, x], [3, 4]] = [[1, 2], [3, 4]] }`, "[2]"},
		{"pattern: array same var", `p[x] { [2, x, 3] = [x, 2, 3] }`, "[2]"},
		{"pattern: array multiple vars", `p[z] { [1, x, y] = [1, 2, 3]; z = [x, y] }`, "[[2, 3]]"},
		{"pattern: array multiple vars 2", `p[z] { [1, x, 3] = [y, 2, 3]; z = [x, y] }`, "[[2, 1]]"},
		{"pattern: array ref", `p[x] { [1, 2, 3, x] = [a[0], a[1], a[2], a[3]] }`, "[4]"},
		{"pattern: array non-ground ref", `p[x] { [1, 2, 3, x] = [a[0], a[1], a[2], a[i]] }`, "[1,2,3,4]"},
		{"pattern: array = ref", `p[x] { [true, false, x] = c[i][j] }`, `["foo"]`},
		{"pattern: array = ref (reversed)", `p[x] { c[i][j] = [true, false, x] }`, `["foo"]`},
		{"pattern: array = var", `p[y] { [1, 2, x] = y; x = 3 }`, "[[1,2,3]]"},

		// objects and variables
		{"pattern: object val", `p[y] { {"x": y} = {"x": "y"} }`, `["y"]`},
		{"pattern: object same var", `p[x] { {"x": x, "y": x} = {"x": 1, "y": 1} }`, "[1]"},
		{"pattern: object multiple vars", `p[z] { {"x": x, "y": y} = {"x": 1, "y": 2}; z = [x, y] }`, "[[1, 2]]"},
		{"pattern: object multiple vars 2", `p[z] { {"x": x, "y": 2} = {"x": 1, "y": y}; z = [x, y] }`, "[[1, 2]]"},
		{"pattern: object ref", `p[x] { {"p": c[0].x[0], "q": x} = c[i][j] }`, `[false]`},
		{"pattern: object non-ground ref", `p[x] { {"a": 1, "b": x} = {"a": 1, "b": c[0].x[i]} }`, `[true, false, "foo"]`},
		{"pattern: object = ref", `p[x] { {"p": y, "q": z} = c[i][j]; x = [i, j, y, z] }`, `[[0, "z", true, false]]`},
		{"pattern: object = ref (reversed)", `p[x] { c[i][j] = {"p": y, "q": z}; x = [i, j, y, z] }`, `[[0, "z", true, false]]`},
		{"pattern: object = var", `p[x] { {"a": 1, "b": y} = x; y = 2 }`, `[{"a": 1, "b": 2}]`},
		{"pattern: object/array nested", `p[ys] { f[i] = {"xs": [2], "ys": ys} }`, `[[3.0]]`},
		{"pattern: object/array nested 2", `p[v] { f[i] = {"xs": [x], "ys": [y]}; v = [x, y] }`, `[[1.0, 2.0], [2.0, 3.0]]`},

		// unordered collections requiring plug
		{"unordered: sets", `p[x] { x = 2; {1,x,3} = {1,2,3} }`, `[2]`},
		{"unordered: object keys", `p[x] { x = "a"; {x: 1} = {"a": 1} }`, `["a"]`},
		{"unordered: object keys (reverse)", `p[x] { x = "a"; {"a": 1} = {x: 1} }`, `["a"]`},

		// indexing
		{"indexing: intersection", `p = true { a[i] = g[i][j] }`, ""},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownUndos(t *testing.T) {
	tests := []struct {
		note     string
		rule     string
		expected interface{}
	}{
		{
			note:     "array-type",
			rule:     "p[x] { arr = [[1, [2]], [1, null], [2, [2]]]; [x, [2]] = arr[_] }",
			expected: "[1, 2]",
		},
		{
			note:     "arrays-element",
			rule:     "p[x] { arr = [[1, 2], [1, null], [2, 2]]; arr[_] = [x, 2] }",
			expected: "[1, 2]",
		},
		{
			note:     "arrays-length",
			rule:     "p[x] { arr = [[1, [2]], [1, []], [2, [2]]]; arr[_] = [x, [2]] }",
			expected: "[1, 2]",
		},
		{
			note:     "array-ref-element",
			rule:     "p[x] { arr = [[1, 2], data.arr_ref, [2, 2]]; arr[_] = [x, 2] }",
			expected: "[1, 2]",
		},
		{
			note:     "object-type",
			rule:     `p[x] { obj = {"a": {"x": 1, "y": {"v": 2}}, "b": {"x": 1, "y": null}, "c": {"x": 2, "y": {"v": 2}}}; {"x": x, "y": {"v": 2}} = obj[_] }`,
			expected: "[1, 2]",
		},
		{
			note:     "objects-element",
			rule:     `p[x] { obj = {"a": {"x": 1, "y": 2}, "b": {"x": 1, "y": null}, "c": {"x": 2, "y": 2}}; obj[_] = {"x": x, "y": 2}}`,
			expected: "[1, 2]",
		},
		{
			note:     "objects-length",
			rule:     `p[x] { obj = {"a": {"x": 1, "y": {"v": 2}}, "b": {"x": 1, "y": {}}, "c": {"x": 2, "y": {"v": 2}}}; obj[_] = {"x": x, "y": {"v": 2}}}`,
			expected: "[1, 2]",
		},
		{
			note:     "object-ref-element",
			rule:     `p[x] { obj = {"a": {"x": 1, "y": 2}, "b": obj_ref, "c": {"x": 2, "y": 2}}; obj[_] = {"x": x, "y": 2}}`,
			expected: "[1, 2]",
		},
		{
			note:     "object-ref-missing-key",
			rule:     `p[x] { obj = {"a": {"x": 1, "y": 2}, "b": obj_ref_missing_key, "c": {"x": 2, "y": 2}}; obj[_] = {"x": x, "y": 2}}`,
			expected: "[1, 2]",
		},
	}

	data := util.MustUnmarshalJSON([]byte(`
		{
			"arr_ref": [1, null],
			"obj_ref": {"x": 1, "y": null},
			"obj_ref_missing_key": {"x": 3, "z": 2}
		}
		`)).(map[string]interface{})

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownComparisonExpr(t *testing.T) {

	tests := []struct {
		note     string
		rule     string
		expected interface{}
	}{
		{"equals", `p = true { 1 == 1; a[i] = x; x == 2 }`, "true"},
		{"noteq", `p = true { 0 != 1; a[i] = x; x != 2 }`, "true"},
		{"gt", `p = true { 1 > 0; a[i] = x; x > 2 }`, "true"},
		{"gteq", `p = true { 1 >= 1; a[i] = x; x >= 4 }`, "true"},
		{"lt", `p = true { -1 < 0; a[i] = x; x < 5 }`, "true"},
		{"lteq", `p = true { -1 <= 0; a[i] = x; x <= 1 }`, "true"},
		{"undefined: equals", `p = true { 0 == 1 }`, ""},
		{"undefined: noteq", `p = true { 0 != 0 }`, ""},
		{"undefined: gt", `p = true { 1 > 2 }`, ""},
		{"undefined: gteq", `p = true { 1 >= 2 }`, ""},
		{"undefined: lt", `p = true { 1 < -1 }`, ""},
		{"undefined: lteq", `p = true { 1 < -1 }`, ""},
	}

	data := loadSmallTestData()
	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownVirtualDocs(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		// input to partial set and object docs
		{"input: set 1", []string{`p = true { q[1] }`, `q[x] { a[i] = x }`}, "true"},
		{"input: set 2", []string{`p[x] { q[1] = x }`, `q[x] { a[i] = x }`}, "[1]"},
		{"input: set embedded", []string{`p[x] { x = {"b": [q[2]]} }`, `q[x] { a[i] = x }`}, `[{"b": [2]}]`},
		{"input: set undefined", []string{`p = true { q[1000] }`, `q[x] { a[x] = y }`}, ""},
		{"input: set dereference", []string{`p = y { x = [1]; q[x][0] = y }`, `q[[x]] { a[_] = x }`}, "1"},
		{"input: set ground var", []string{`p[x] { x = 1; q[x] }`, `q[y] { a[y] = i }`}, "[1]"},
		{"input: set ground composite (1)", []string{
			`p = true { z = [[1, 2], 2]; q[z] }`,
			`q[[x, y]] { x = [1, y]; y = 2 }`,
		}, "true"},
		{"input: set ground composite (2)", []string{
			`p = true { y = 2; z = [[1, y], y]; q[z] }`,
			`q[[x, y]] { x = [1, y]; y = 2 }`,
		}, "true"},
		{"input: set ground composite (3)", []string{
			`p = true { y = 2; x = [1, y]; z = [x, y]; q[z] }`,
			`q[[x, y]] { x = [1, y]; y = 2 }`,
		}, "true"},
		{"input: set partially ground composite", []string{
			`p[u] { y = 2; x = [1, u]; z = [x, y]; q[z] }`, // "u" is not ground here
			`q[[x, y]] { x = [1, y]; y = 2 }`,
		}, "[2]"},
		{"input: object 1", []string{`p = true { q[1] = 2 }`, `q[i] = x { a[i] = x }`}, "true"},
		{"input: object 2", []string{`p = true { q[1] = 0 }`, `q[x] = i { a[i] = x }`}, "true"},
		{"input: object embedded 1", []string{`p[x] { x = [1, q[3], q[2]] }`, `q[i] = x { a[i] = x }`}, "[[1,4,3]]"},
		{"input: object embedded 2", []string{`p[x] { x = {"a": [q[3]], "b": [q[2]]} }`, `q[i] = x { a[i] = x }`}, `[{"a": [4], "b": [3]}]`},
		{"input: object undefined val", []string{`p = true { q[1] = 9999 }`, `q[i] = x { a[i] = x }`}, ""},
		{"input: object undefined key 1", []string{`p = true { q[9999] = 2 }`, `q[i] = x { a[i] = x }`}, ""},
		{"input: object undefined key 2", []string{`p = true { q.foo = 2 }`, `q[i] = x { a[i] = x }`}, ""},
		{"input: object dereference ground", []string{`p = true { q[0].x[1] = false }`, `q[i] = x { x = c[i] }`}, "true"},
		{"input: object dereference ground 2", []string{`p[v] { x = "a"; q[x][y] = v }`, `q[k] = v { k = "a"; v = data.a }`}, "[1,2,3,4]"},
		{"input: object defererence non-ground", []string{`p = true { q[0][x][y] = false }`, `q[i] = x { x = c[i] }`}, "true"},
		{"input: object ground var key", []string{`p[y] { x = "b"; q[x] = y }`, `q[k] = v { x = {"a": 1, "b": 2}; x[k] = v }`}, "[2]"},
		{"input: object non-string key", []string{`p[y] { x = 1; q[x] = y }`, `q[k] = v { x = {2: 1, 1: 3}; x[k] = v }`}, "[3]"},
		{"input: variable binding substitution", []string{
			`p[x] = y { r[z] = y; q[x] = z }`,
			`r[k] = v { x = {"a": 1, "b": 2, "c": 3, "d": 4}; x[k] = v }`,
			`q[y] = x { z = {"a": "a", "b": "b", "d": "d"}; z[y] = x }`},
			`{"a": 1, "b": 2, "d": 4}`},

		// output from partial set and object docs
		{"output: set", []string{`p[x] { q[x] }`, `q[y] { a[i] = y }`}, "[1,2,3,4]"},
		{"output: set embedded", []string{`p[i] { {i: [i]} = {i: [q[i]]} }`, `q[x] { d.e[i] = x }`}, `["bar", "baz"]`},
		{"output: set var binding", []string{`p[x] { q[x] }`, `q[y] { y = [i, j]; i = 1; j = 2 }`}, `[[1,2]]`},
		{"output: set dereference", []string{`p[y] { q[x][0] = y }`, `q[[x]] { a[_] = x }`}, `[1,2,3,4]`},
		{"output: set dereference deep", []string{`p[y] { q[i][j][k][x] = y }`, `q[{{[1], [2]}, {[3], [4]}}] { true }`}, "[1,2,3,4]"},
		{"output: set falsy values", []string{`p[x] { q[x] }`, `q = {0, "", false, null, [], {}, set()} { true }`}, `[0, "", null, [], {}, []]`},
		{"output: object key", []string{`p[x] { q[x] = 4 }`, `q[i] = x { a[i] = x }`}, "[3]"},
		{"output: object non-string key", []string{`p[x] { q[x] = 1 }`, `q[k] = 1 { a[_] = k; k < 3 }`}, "[1,2]"},
		{"output: object value", []string{`p[x] = y { q[x] = y }`, `q[k] = v { b[k] = v }`}, `{"v1": "hello", "v2": "goodbye"}`},
		{"output: object embedded", []string{`p[k] = v { {k: [q[k]]} = {k: [v]} }`, `q[x] = y { b[x] = y }`}, `{"v1": "hello", "v2": "goodbye"}`},
		{"output: object dereference ground", []string{`p[i] { q[i].x[1] = false }`, `q[i] = x { x = c[i] }`}, "[0]"},
		{"output: object defererence non-ground", []string{
			`p[r] { q[x][y][z] = false; r = [x, y, z] }`,
			`q[i] = x { x = c[i] }`},
			`[[0, "x", 1], [0, "z", "q"]]`},
		{"output: object dereference array of refs", []string{
			`p[x] { q[_][0].c[_] = x }`,
			`q[k] = v { d.e[_] = k; v = [r | r = l[_]] }`,
		}, "[1,2,3,4]"},
		{"output: object dereference array of refs within object", []string{
			`p[x] { q[_].x[0].c[_] = x }`,
			`q[k] = v { d.e[_] = k; v = {"x": [r | r = l[_]]} }`,
		}, "[1,2,3,4]"},
		{"output: object dereference object with key refs", []string{
			`p = true { q.bar[1].alice[0] = 1 }`,
			`q[k] = v { d.e[_] = k; v = [x | x = {l[_].a: [1]}] }`,
		}, "true"},
		{"output: object var binding", []string{
			`p[z] { q[x] = y; z = [x, y] }`,
			`q[k] = v { v = [x, y]; x = "a"; y = "b"; k = "foo" }`},
			`[["foo", ["a", "b"]]]`},
		{"output: object key var binding", []string{
			`p[z] { q[x] = y; z = [x, y] }`,
			`q[k] = v { k = y; y = x; x = "a"; v = "foo" }`},
			`[["a", "foo"]]`},
		{"object: self-join", []string{
			`p[[x, y]] { q[x] = 1; q[y] = x }`,
			`q[x] = i { a[i] = x }`},
			"[[2,3]]"},

		// input+output from partial set/object docs
		{"i/o: objects", []string{
			`p[x] { q[x] = r[x] }`,
			`q[x] = y { z = {"a": 1, "b": 2, "d": 4}; z[x] = y }`,
			`r[k] = v { x = {"a": 1, "b": 2, "c": 4, "d": 3}; x[k] = v }`},
			`["a", "b"]`},

		{"i/o: undefined keys", []string{
			`p[y] { q[x]; r[x] = y }`,
			`q[x] { z = ["a", "b", "c", "d"]; z[y] = x }`,
			`r[k] = v { x = {"a": 1, "b": 2, "d": 4}; x[k] = v }`},
			`[1, 2, 4]`},

		// input/output to/from complete docs
		{"input: complete array", []string{`p = true { q[1] = 2 }`, `q = [1, 2, 3, 4] { true }`}, "true"},
		{"input: complete object", []string{`p = true { q.b = 2 }`, `q = {"a": 1, "b": 2} { true }`}, "true"},
		{"input: complete set", []string{`p = true { q[3] }`, `q = {1, 2, 3, 4} { true }`}, "true"},
		{"input: complete array dereference ground", []string{`p = true { q[1][1] = 3 }`, `q = [[0, 1], [2, 3]] { true }`}, "true"},
		{"input: complete object dereference ground", []string{`p = true { q.b[1] = 4 }`, `q = {"a": [1, 2], "b": [3, 4]} { true }`}, "true"},
		{"input: complete array ground index", []string{`p[x] { z = [1, 2]; z[i] = y; q[y] = x }`, `q = [1, 2, 3, 4] { true }`}, "[2,3]"},
		{"input: complete object ground key", []string{`p[x] { z = ["b", "c"]; z[i] = y; q[y] = x }`, `q = {"a": 1, "b": 2, "c": 3, "d": 4} { true }`}, "[2,3]"},
		{"input: complete vars", []string{
			`p = true { q[1][1] = 2 }`,
			`q = [{"x": x, "y": y}, z] { x = 1; y = 2; z = [1, 2, 3] }`,
		}, `true`},
		{"output: complete array", []string{`p[x] { q[i] = e; x = [i, e] }`, `q = [1, 2, 3, 4] { true }`}, "[[0,1],[1,2],[2,3],[3,4]]"},
		{"output: complete object", []string{`p[x] { q[i] = e; x = [i, e] }`, `q = {"a": 1, "b": 2} { true }`}, `[["a", 1], ["b", 2]]`},
		{"output: complete set", []string{`p[x] { q[x] }`, `q = {1, 2, 3, 4} { true }`}, "[1,2,3,4]"},
		{"output: complete array dereference non-ground", []string{`p[r] { q[i][j] = 2; r = [i, j] }`, `q = [[1, 2], [3, 2]] { true }`}, "[[0, 1], [1, 1]]"},
		{"output: complete object defererence non-ground", []string{`p[r] { q[x][y] = 2; r = [x, y] }`, `q = {"a": {"x": 1}, "b": {"y": 2}, "c": {"z": 2}} { true }`}, `[["b", "y"], ["c", "z"]]`},
		{"output: complete vars", []string{
			`p[x] { q[_][_] = x }`,
			`q = [{"x": x, "y": y}, z] { x = 1; y = 2; z = [1, 2, 3] }`,
		}, `[1,2,3]`},

		// no dereferencing
		{"no suffix: complete", []string{`p = true { q }`, `q = true { true }`}, "true"},
		{"no suffix: complete vars", []string{
			`p = true { q }`, `q = x { x = true }`,
		}, "true"},
		{"no suffix: complete incr (error)", []string{`p = true { q }`, `q = false { true }`, `q = true { true }`}, completeDocConflictErr(nil)},
		{"no suffix: complete incr", []string{`p = true { not q }`, `q = true { false }`, `q = false { true }`}, "true"},
		{"no suffix: object", []string{`p[x] = y { q = o; o[x] = y }`, `q[x] = y { b[x] = y }`}, `{"v1": "hello", "v2": "goodbye"}`},
		{"no suffix: object incr", []string{
			`p[x] = y { q = o; o[x] = y }`,
			`q[x] = y { b[x] = y }`,
			`q[x1] = y1 { d.e[y1] = x1 }`},
			`{"v1": "hello", "v2": "goodbye", "bar": 0, "baz": 1}`},
		{"no suffix: chained", []string{
			`p = true { q = x; x[i] = 4 }`,
			`q[k] = v { r = x; x[k] = v }`,
			`r[k] = v { s = x; x[k] = v }`,
			`r[k] = v { t = x; x[v] = k }`,
			`s = {"a": 1, "b": 2, "c": 4} { true }`,
			`t = ["d", "e", "g"] { true }`},
			"true"},
		{"no suffix: object var binding", []string{
			`p[x] { q = x }`,
			`q[k] = v { v = [i, j]; k = i; i = "a"; j = 1 }`},
			`[{"a": ["a", 1]}]`},
		{"no suffix: object composite value", []string{
			`p[x] { q = x }`,
			`q[k] = {"v": v} { v = [i, j]; k = i; i = "a"; j = 1 }`},
			`[{"a": {"v": ["a", 1]}}]`},
		// data.c[0].z.p is longer than data.q
		{"no suffix: bound ref with long prefix (#238)", []string{
			`p = true { q; q }`,
			`q = x { x = data.c[0].z.p }`}, "true"},
		{"no suffix: object conflict (error)", []string{
			`p[x] = y { xs = ["a", "b", "c", "a"]; x = xs[i]; y = a[i] }`},
			objectDocKeyConflictErr(nil)},
		{"no suffix: set", []string{`p[x] { q = s; s[x] }`, `q[x] { a[i] = x }`}, "[1,2,3,4]"},

		{"empty partial set", []string{"p[1] { a[0] = 100 }"}, "[]"},
		{"empty partial object", []string{`p["x"] = 1 { a[0] = 100 }`}, "{}"},

		{"input: non-ground object keys", []string{
			`p = x { q.a.b = x }`,
			`q = {x: {y: 1}} { x = "a"; y = "b" }`,
		}, "1"},

		{"input: non-ground set elements", []string{
			`p { q["c"] }`,
			`q = {x, "b", z} { x = "a"; z = "c" }`,
		}, "true"},

		{"output: non-ground object keys", []string{
			`p[x] { q[i][j] = x }`,
			`q = {x: {x1: 1}, y: {y1: 2}} { x = "a"; y = "b"; x1 = "a1"; y1 = "b1" }`,
		}, "[1, 2]"},

		{"output: non-ground set elements", []string{
			`p[x] { q[x] }`,
			`q = {x, "b", z} { x = "a"; z = "c" }`,
		}, `["a", "b", "c"]`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownBaseAndVirtualDocs(t *testing.T) {

	// Define base docs that will overlap with virtual docs.
	var data map[string]interface{}

	input := `
	{
		"topdown": {
			"a": {
				"b": {
					"c": {
						"x": [100,200],
						"y": false,
						"z": {
							"a": "b"
						}
					}
				}
			},
			"g": {
				"h": {
					"k": [1,2,3]
				}
			},
			"set": {
				"u": [1,2,3,4]
			},
			"conflicts": {
				"k": "foo"
			}
		}
	}
	`
	if err := util.UnmarshalJSON([]byte(input), &data); err != nil {
		panic(err)
	}

	compiler := compileModules([]string{
		// Define virtual docs that will overlap with base docs.
		`package topdown.a.b.c

p = [1, 2] { true }
q = [3, 4] { true }
r["a"] = 1 { true }
r["b"] = 2 { true }`,

		`package topdown.a.b.c.s

w = {"f": 10, "g": 9.9} { true }`,

		`package topdown.set

v[data.topdown.set.u[_]] { true }`,

		`package topdown.no.base.doc

p = true { true }`,

		`package topdown.a.b.c.undefined1

p = true { false }
p = true { false }
q = true { false }`,

		`package topdown.a.b.c.undefined2

p = true { input.foo }`,

		`package topdown.a.b.c.empty`,

		`package topdown.g.h

p = true { false }`,

		`package topdown.virtual.constants

		p = 1
		q = 2
		r = 1`,

		`package topdown.missing.input.value

		p = input.deadbeef`,

		// Define virtual docs that we can query to obtain merged result.
		`package topdown

p[[x1, x2, x3, x4]] { data.topdown.a.b[x1][x2][x3] = x4 }
q[[x1, x2, x3]] { data.topdown.a.b[x1][x2][0] = x3 }
r[[x1, x2]] { data.topdown.a.b[x1] = x2 }
s = data.topdown.no { true }
t = data.topdown.a.b.c.undefined1 { true }
u = data.topdown.missing.input.value { true }
v = data.topdown.g { true }
w = data.topdown.set { true }

iterate_ground[x] { data.topdown.virtual.constants[x] = 1 }
`,
		`package topdown.conflicts

		k = "bar"`,
		`package enum_errors.a.b.c

p = x { x = 1/0 }`,
		`package enum_errors.caller

p[x] = y { data.enum_errors.a[x] = y }`,
	})

	store := inmem.NewFromObject(data)

	assertTopDownWithPath(t, compiler, store, "base/virtual", []string{"topdown", "p"}, "{}", `[
		["c", "p", 0, 1],
		["c", "p", 1, 2],
		["c", "q", 0, 3],
		["c", "q", 1, 4],
		["c", "r", "a", 1],
		["c", "r", "b", 2],
		["c", "x", 0, 100],
		["c", "x", 1, 200],
		["c", "z", "a", "b"],
		["c", "s", "w", {"f":10, "g": 9.9}]
	]`)

	assertTopDownWithPath(t, compiler, store, "base/virtual: ground key", []string{"topdown", "q"}, "{}", `[
		["c", "p", 1],
		["c", "q", 3],
		["c", "x", 100]
	]`)

	assertTopDownWithPath(t, compiler, store, "base/virtual: prefix", []string{"topdown", "r"}, "{}", `[
		["c", {
			"p": [1,2],
			"q": [3,4],
			"r": {"a": 1, "b": 2},
			"s": {"w": {"f": 10, "g": 9.9}},
			"x": [100,200],
			"y": false,
			"z": {"a": "b"},
			"undefined1": {},
			"undefined2": {},
			"empty": {}
		}]
	]`)

	assertTopDownWithPath(t, compiler, store, "base/virtual: set", []string{"topdown", "w"}, "{}", `{
		"v": [1,2,3,4],
		"u": [1,2,3,4]
	}`)

	assertTopDownWithPath(t, compiler, store, "base/virtual: no base", []string{"topdown", "s"}, "{}", `{"base": {"doc": {"p": true}}}`)
	assertTopDownWithPath(t, compiler, store, "base/virtual: undefined", []string{"topdown", "t"}, "{}", "{}")
	assertTopDownWithPath(t, compiler, store, "base/virtual: undefined-2", []string{"topdown", "v"}, "{}", `{"h": {"k": [1,2,3]}}`)
	assertTopDownWithPath(t, compiler, store, "base/virtual: missing input value", []string{"topdown", "u"}, "{}", "{}")
	assertTopDownWithPath(t, compiler, store, "iterate ground", []string{"topdown", "iterate_ground"}, "{}", `["p", "r"]`)
	assertTopDownWithPath(t, compiler, store, "base/virtual: conflicts", []string{"topdown.conflicts"}, "{}", `{"k": "foo"}`)
	assertTopDownWithPath(t, compiler, store, "enumerate virtual errors", []string{"enum_errors", "caller", "p"}, `{}`, &Error{Code: BuiltinErr, Message: "divide by zero"})
}

func TestTopDownFix1863(t *testing.T) {

	compiler := ast.MustCompileModules(map[string]string{
		"test1.rego": `
			package a.b

			# this module is empty
		`,
		"test2.rego": `
			package x

			p = data.a.b  # p should be defined (an empty object)
		`,
	})

	store := inmem.New()

	assertTopDownWithPath(t, compiler, store, "is defined", []string{}, ``, `{"a": {"b": {}}, "x": {"p": {}}}`)
	assertTopDownWithPath(t, compiler, store, "is defined", []string{"x"}, ``, `{"p": {}}`)
	assertTopDownWithPath(t, compiler, store, "is defined", []string{"x", "p"}, ``, `{}`)
}

func TestTopDownNestedReferences(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		// nested base document references
		{"ground ref", []string{`p = true { a[h[0][0]] = 2 }`}, "true"},
		{"non-ground ref", []string{`p[x] { x = a[h[i][j]] }`}, "[2,3,4]"},
		{"two deep", []string{`p[x] { x = a[a[a[i]]] }`}, "[3,4]"},
		{"two deep", []string{`p[x] { x = a[h[i][a[j]]] }`}, "[3,4]"},
		{"two deep repeated var", []string{`p[x] { x = a[h[i][a[i]]] }`}, "[3]"},
		{"no suffix", []string{`p = true { 4 = a[three] }`}, "true"},
		{"var ref", []string{`p[y] { x = [1, 2, 3]; y = a[x[_]] }`}, "[2,3,4]"},
		{"undefined", []string{`p = true { a[three.deadbeef] = x }`}, ""},

		// nested virtual document references
		{"vdoc ref: complete", []string{`p[x] { x = a[q[_]] }`, `q = [2, 3] { true }`}, "[3,4]"},
		{"vdoc ref: complete: ground", []string{`p[x] { x = a[q[1]] }`, `q = [2, 3] { true }`}, "[4]"},
		{"vdoc ref: complete: no suffix", []string{`p = true { 2 = a[q] }`, `q = 1 { true }`}, "true"},
		{"vdoc ref: partial object", []string{
			`p[x] { x = a[q[_]] }`,
			`q[k] = v { o = {"a": 2, "b": 3, "c": 100}; o[k] = v }`},
			"[3,4]"},
		{"vdoc ref: partial object: ground", []string{
			`p[x] { x = a[q.b] }`,
			`q[k] = v { o = {"a": 2, "b": 3, "c": 100}; o[k] = v }`},
			"[4]"},

		// mixed cases
		{"vdoc ref: complete: nested bdoc ref", []string{
			`p[x] { x = a[q[b[_]]] }`,
			`q = {"hello": 1, "goodbye": 3, "deadbeef": 1000} { true }`}, "[2,4]"},
		{"vdoc ref: partial object: nested bdoc ref", []string{
			`p[x] { x = a[q[b[_]]] }`,
			// bind to value
			`q[k] = v { o = {"hello": 1, "goodbye": 3, "deadbeef": 1000}; o[k] = v }`}, "[2,4]"},
		{"vdoc ref: partial object: nested bdoc ref-2", []string{
			`p[x] { x = a[q[d.e[_]]] }`,
			// bind to reference
			`q[k] = v { strings[k] = v }`}, "[3,4]"},
		{"vdoc ref: multiple", []string{
			`p[x] { x = q[a[_]].v[r[a[_]]] }`,
			`q = [{"v": {}}, {"v": [0, 0, 1, 2]}, {"v": [0, 0, 3, 4]}, {"v": [0, 0]}, {}] { true }`,
			`r = [1, 2, 3, 4] { true }`}, "[1,2,3,4]"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownVarReferences(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"ground", []string{`p[x] { v = [[1, 2], [2, 3], [3, 4]]; x = v[2][1] }`}, "[4]"},
		{"non-ground", []string{`p[x] { v = [[1, 2], [2, 3], [3, 4]]; x = v[i][j] }`}, "[1,2,3,4]"},
		{"mixed", []string{`p[x] = y { v = [{"a": 1, "b": 2}, {"c": 3, "z": [4]}]; y = v[i][x][j] }`}, `{"z": 4}`},
		{"ref binding", []string{`p[x] { v = c[i][j]; x = v[k]; x = true }`}, "[true]"},
		{"existing ref binding", []string{`p = x { q = a; q[0] = x; q[0] }`}, `1`},
		{"embedded", []string{`p[x] { v = [1, 2, 3]; x = [{"a": v[i]}] }`}, `[[{"a": 1}], [{"a": 2}], [{"a": 3}]]`},
		{"embedded ref binding", []string{`p[x] { v = c[i][j]; w = [v[0], v[1]]; x = w[y] }`}, "[null, false, true, 3.14159]"},
		{"array: ground var", []string{`p[x] { i = [1, 2, 3, 4]; j = [1, 2, 999]; j[k] = y; i[y] = x }`}, "[2,3]"},
		{"array: ref", []string{`p[y] { i = [1,2,3,4]; x = data.a[_]; i[x] = y }`}, `[2, 3, 4]`},
		{"object: ground var", []string{`p[x] { i = {"a": 1, "b": 2, "c": 3}; j = ["a", "c", "deadbeef"]; j[k] = y; i[y] = x }`}, "[1, 3]"},
		{"object: ref", []string{`p[y] { i = {"1": 1, "2": 2, "4": 4}; x = data.numbers[_]; i[x] = y }`}, `[1, 2, 4]`},
		{"set: ground var", []string{`p[x] { i = {1, 2, 3, 4}; j = {1, 2, 99}; j[x]; i[x] }`}, "[1,2]"},
		{"set: ref", []string{`p[x] { i = {1, 2, 3, 4}; x = data.a[_]; i[x] }`}, `[1, 2, 3, 4]`},
		{"set: lookup: base docs", []string{`p = true { v = {[1, 999], [3, 4]}; pair = [a[2], 4]; v[pair] }`}, "true"},
		{"set: lookup: embedded", []string{`p = true { x = [{}, {[1, 2], [3, 4]}]; y = [3, 4]; x[i][y] }`}, "true"},
		{"set: lookup: dereference", []string{`p[[i, z, r]] { x = [{}, {[1, 2], [3, 4]}]; y = [3, 4]; x[i][y][z] = r }`}, "[[1,0,3], [1,1,4]]"},
		{"avoids indexer", []string{`p = true { somevar = [1, 2, 3]; somevar[i] = 2 }`}, "true"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownCompositeReferences(t *testing.T) {
	tests := []struct {
		note     string
		rule     string
		expected interface{}
	}{
		{"array", "p = fixture.r[[1, 2]]", "[1, 2]"},
		{"object", `p = fixture.r[{"foo": "bar"}]`, `{"foo": "bar"}`},
		{"set", `p = fixture.r[{1, 2}]`, "[1, 2]"},

		{"unify array", `p = [x | fixture.r[[1, x]]]`, "[2, 3]"},
		{"unify object", `p = [x | fixture.r[{"foo": x}]]`, `["bar"]`},
		{"unify partial ground array", `p = [x | fixture.p1[[x,2]]]`, `[1,2]`},

		{"complete doc unify", `p = [[x,y] | fixture.s[[x, y]]]`, `[[1, 2], [1, 3], [2, 7], [[1,1], 4]]`},
		{"partial doc unify", `p = [[x,y] | fixture.r[[x, y]]]`, `[[1, 2], [1, 3], [2, 7], [[1,1], 4]]`},

		{"empty set", `p { fixture.empty[set()]} `, "true"},

		{"ref", `p = fixture.r[[fixture.foo.bar, 3]]`, "[1,3]"},
		{"nested ref", `p = fixture.r[[fixture.foo[fixture.o.foo], 3]]`, "[1,3]"},

		{"comprehension", `p = fixture.s[[[x | x = y[_]; y = [1, 1]], 4]]`, "[[1,1],4]"},

		{"missing array", `p = fixture.r[[1, 4]]`, ``},
		{"missing object value", `p = fixture.r[{"foo": "baz"}]`, ``},
		{"missing set", `p = fixture.r[{1, 3}]`, ``},
	}

	fixture := `package fixture
		empty = {set()}
		s = {[1, 2], [1, 3], {"foo": "bar"}, {1, 2}, [2, 7], [[1,1], 4]}
		r[x] { s[x] }
		a = [1, 2]
		o = {"foo": "bar"}
		foo = {"bar": 1}

		p1[[1,2]]
		p1[[1,3]]
		p1[[2,2]]
	`

	for _, tc := range tests {
		module := "package test\nimport data.fixture\n" + tc.rule
		compiler := compileModules([]string{fixture, module})
		assertTopDownWithPath(t, compiler, inmem.New(), tc.note, []string{"test", "p"}, "", tc.expected)
	}
}

func TestTopDownIndirectReferences(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"array", []string{`p[x] {[1, 2, 3][x]}`}, "[0, 1, 2]"},
		{"call", []string{`p {split("foo.bar", ".")[0] == "foo"}`}, "true"},
		{"multiple call", []string{`p[x] {split(split("foo.bar:qux", ".")[_], ":")[i] = x}`}, `["foo", "bar", "qux"]`},
		{"user call", []string{`fn(x) = [x] {true}`, `p[x] {x = fn(1)[0]}`}, "[1]"},
		{"user call in comprehension", []string{`fn(x) = [x] {true}`, `p[x] {x = [y | y = fn(1)][_][_]}`}, "[1]"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownDisjunction(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"incr: query set", []string{`p[x] { a[i] = x }`, `p[y] { b[j] = y }`}, `[1,2,3,4,"hello","goodbye"]`},
		{"incr: query set constants", []string{
			`p[100] { true }`,
			`p[x] { a[x] }`},
			"[0,1,2,3,100]"},
		{"incr: query object", []string{
			`p[k] = v { b[v] = k }`,
			`p[k] = v { a[i] = v; g[k][j] = v }`},
			`{"b": 2, "c": 4, "hello": "v1", "goodbye": "v2", "a": 1}`},
		{"incr: query object constant key", []string{
			`p["a"] = 1 { true }`,
			`p["b"] = 2 { true }`},
			`{"a": 1, "b": 2}`},
		{"incr: iter set", []string{
			`p[x] { q[x] }`,
			`q[x] { a[i] = x }`,
			`q[y] { b[j] = y }`},
			`[1,2,3,4,"hello","goodbye"]`},
		{"incr: eval set", []string{
			`p[x] { q = s; s[x] }`, // make p a set so that test assertion orders result
			`q[x] { a[_] = x }`,
			`q[y] { b[_] = y }`},
			`[1,2,3,4,"hello","goodbye"]`},
		{"incr: eval object", []string{
			`p[k] = v { q[k] = v }`,
			`q[k] = v { b[v] = k }`,
			`q[k] = v { a[i] = v; g[k][j] = v }`},
			`{"b": 2, "c": 4, "hello": "v1", "goodbye": "v2", "a": 1}`},
		{"incr: eval object constant key", []string{
			`p[k] = v { q[k] = v }`,
			`q["a"] = 1 { true }`,
			`q["b"] = 2 { true }`},
			`{"a": 1, "b": 2}`},
		{"complete: undefined", []string{`p = true { false }`, `p = true { false }`}, ""},
		{"complete: error", []string{`p = true { true }`, `p = false { false }`, `p = false { true }`}, completeDocConflictErr(nil)},
		{"complete: valid", []string{`p = true { true }`, `p = true { true }`}, "true"},
		{"complete: valid-2", []string{`p = true { true }`, `p = false { false }`}, "true"},
		{"complete: reference error", []string{`p = true { q }`, `q = true { true }`, `q = false { true }`}, completeDocConflictErr(nil)},
		{"complete: reference valid", []string{`p = true { q }`, `q = true { true }`, `q = true { true }`}, "true"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownNegation(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"neg: constants", []string{`p = true { not true = false }`}, "true"},
		{"neg: constants", []string{`p = true { not true = true }`}, ""},
		{"neg: set contains", []string{`p = true { not q.v0 }`, `q[x] { b[x] = v }`}, "true"},
		{"neg: set contains undefined", []string{`p = true { not q.v2 }`, `q[x] { b[x] = v }`}, ""},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownComprehensions(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"array simple", []string{`p[i] { xs = [x | x = a[_]]; xs[i] > 1 }`}, "[1,2,3]"},
		{"array nested", []string{`p[i] { ys = [y | y = x[_]; x = [z | z = a[_]]]; ys[i] > 1 }`}, "[1,2,3]"},
		{"array embedded array", []string{`p[i] { xs = [[x | x = a[_]]]; xs[0][i] > 1 }`}, "[1,2,3]"},
		{"array embedded object", []string{`p[i] { xs = {"a": [x | x = a[_]]}; xs.a[i] > 1 }`}, "[1,2,3]"},
		{"array embedded set", []string{`p = xs { xs = {[x | x = a[_]]} }`}, "[[1,2,3,4]]"},
		{"array closure", []string{`p[x] { y = 1; x = [y | y = 1] }`}, "[[1]]"},
		{"array dereference embedded", []string{
			`p[x] { q.a[2][i] = x }`,
			`q[k] = v { k = "a"; v = [y | i[_] = _; i = y; i = [z | z = a[_]]] }`,
		}, "[1,2,3,4]"},

		{"object simple", []string{`p[i] { xs = {s: x | x = a[_]; format_int(x, 10, s)}; y = xs[i]; y > 1 }`}, `["2","3","4"]`},
		{"object non-string key", []string{`p[x] { xs = {k: 1 | a[_] = k}; xs[x]}`}, `[1,2,3,4]`},
		{"object nested", []string{`p = r { r = {x: y | z = {i: q | i = b[q]}; x = z[y]}}`}, `{"v1": "hello", "v2": "goodbye"}`},
		{"object embedded array", []string{`p[i] { xs = [{s: x | x = a[_]; format_int(x, 10, s)}]; xs[0][i] > 1 }`}, `["2","3","4"]`},
		{"object embedded object", []string{`p[i] { xs = {"a": {s: x | x = a[_]; format_int(x, 10, s)}}; xs.a[i] > 1 }`}, `["2","3","4"]`},
		{"object embedded set", []string{`p = xs { xs = {{s: x | x = a[_]; format_int(x, 10, s)}} }`}, `[{"1":1,"2":2,"3":3,"4":4}]`},
		{"object closure", []string{`p[x] { y = 1; x = {"foo":y | y = 1} }`}, `[{"foo": 1}]`},
		{"object dereference embedded", []string{
			`arr = [4] { true }`,
			`p[x] { q.a = x }`,
			`q[k] = v { k = "a"; v = {"bar": y | i[_] = _; i = y; i = {"foo": z | z = arr[_]}} }`,
		}, `[{"bar": {"foo": 4}}]`},
		{"object conflict", []string{
			`p[x] { q.a = x }`,
			`q[k] = v { k = "a"; v = {"bar": y | i[_] = _; i = y; i = {"foo": z | z = a[_]}} }`,
		}, objectDocKeyConflictErr(nil)},

		{"set simple", []string{`p = y {y = {x | x = a[_]; x > 1}}`}, "[2,3,4]"},
		{"set nested", []string{`p[i] { ys = {y | y = x[_]; x = {z | z = a[_]}}; ys[i] > 1 }`}, "[2,3,4]"},
		{"set embedded array", []string{`p[i] { xs = [{x | x = a[_]}]; xs[0][i] > 1 }`}, "[2,3,4]"},
		{"set embedded object", []string{`p[i] { xs = {"a": {x | x = a[_]}}; xs.a[i] > 1 }`}, "[2,3,4]"},
		{"set embedded set", []string{`p = xs { xs = {{x | x = a[_]}} }`}, "[[1,2,3,4]]"},
		{"set closure", []string{`p[x] { y = 1; x = {y | y = 1} }`}, "[[1]]"},
		{"set dereference embedded", []string{
			`p[x] { q.a = x }`,
			`q[k] = v { k = "a"; v = {y | i[_] = _; i = y; i = {z | z = a[_]}} }`,
		}, "[[[1,2,3,4]]]"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownDefaultKeyword(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"undefined", []string{`p = 1 { false }`, `default p = 0`, `p = 2 { false }`}, "0"},
		{"defined", []string{`default p = 0`, `p = 1 { true }`, `p = 2 { false }`}, `1`},
		{"defined-ooo", []string{`p = 1 { true }`, `default p = 0`, `p = 2 { false }`}, "1"},
		{"array comprehension", []string{`p = 1 { false }`, `default p = [x | a[_] = x]`}, "[1,2,3,4]"},
		{"object comprehension", []string{`p = 1 { false }`, `default p = {x: k | d[k][_] = x}`}, `{"bar": "e", "baz": "e"}`},
		{"set comprehension", []string{`p = 1 { false }`, `default p = {x | a[_] = x}`}, `[1,2,3,4]`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownArithmetic(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"plus", []string{`p[y] { a[i] = x; y = i + x }`}, "[1,3,5,7]"},
		{"minus", []string{`p[y] { a[i] = x; y = i - x }`}, "[-1]"},
		{"multiply", []string{`p[y] { a[i] = x; y = i * x }`}, "[0,2,6,12]"},
		{"divide+round", []string{`p[z] { a[i] = x; y = i / x; round(y, z) }`}, "[0, 1]"},
		{"divide+error", []string{`p[y] { a[i] = x; y = x / i }`}, &Error{Code: BuiltinErr, Message: "divide by zero"}},
		{"abs", []string{`p = true { abs(-10, x); x = 10 }`}, "true"},
		{"remainder", []string{`p = x { x = 7 % 4 }`}, "3"},
		{"remainder+error", []string{`p = x { x = 7 % 0 }`}, &Error{Code: BuiltinErr, Message: "modulo by zero"}},
		{"remainder+error+floating", []string{`p = x { x = 1.1 % 1 }`}, &Error{Code: BuiltinErr, Message: "modulo on floating-point number"}},
		{"arity 1 ref dest", []string{`p = true { abs(-4, a[3]) }`}, "true"},
		{"arity 1 ref dest (2)", []string{`p = true { not abs(-5, a[3]) }`}, "true"},
		{"arity 2 ref dest", []string{`p = true { a[2] = 1 + 2 }`}, "true"},
		{"arity 2 ref dest (2)", []string{`p = true { not a[2] = 2 + 3 }`}, "true"},
		{"bug 2469 - precision", []string{"p = true { 49649733057 + 1 == 49649733058 }"}, "true"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownCasts(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"to_number", []string{
			`p = [x, y, z, i, j] { to_number("-42.0", x); to_number(false, y); to_number(100.1, z); to_number(null, i); to_number(true, j) }`,
		},
			"[-42.0, 0, 100.1, 0, 1]"},
		{"to_number ref dest", []string{`p = true { to_number("3", a[2]) }`}, "true"},
		{"to_number ref dest", []string{`p = true { not to_number("-1", a[2]) }`}, "true"},
		{"to_number: bad input", []string{`p { to_number("broken", x) }`}, &Error{Code: BuiltinErr, Message: "invalid syntax"}},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownTypeBuiltin(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"is_number", []string{
			`p = [x, y, z] { is_number(-42.0, x); is_number(0, y); is_number(100.1, z) }`,
		}, "[true, true, true]"},

		{"is_number", []string{
			`p = x { is_number(null, x) }`,
		}, ""},

		{"is_number", []string{
			`p = x { is_number(false, x) }`,
		}, ""},

		{"is_number", []string{
			`p[x] {arr = [true, 1]; arr[_] = x; is_number(x) }`,
		}, "[1]"},

		{"is_string", []string{
			`p = [x, y, z] { is_string("Hello", x); is_string("There", y); is_string("OPA", z) }`,
		}, "[true, true, true]"},

		{"is_string", []string{
			`p = x { is_string(null, x) }`,
		}, ""},

		{"is_string", []string{
			`p = x { is_string(false, x) }`,
		}, ""},

		{"is_string", []string{
			`p[x] {arr = [true, 1, "Hey"]; arr[_] = x; is_string(x) }`,
		}, "[\"Hey\"]"},

		{"is_boolean", []string{
			`p = [x, y] { is_boolean(true, x); is_boolean(false, y) }`,
		}, "[true, true]"},

		{"is_boolean", []string{
			`p = x { is_boolean(null, x) }`,
		}, ""},

		{"is_boolean", []string{
			`p = x { is_boolean("Hello", x) }`,
		}, ""},

		{"is_boolean", []string{
			`p[x] {arr = [false, 1, "Hey"]; arr[_] = x; is_boolean(x) }`,
		}, "[false]"},

		{"is_array", []string{
			`p = [x, y] { is_array([1,2,3], x); is_array(["a", "b"], y) }`,
		}, "[true, true]"},

		{"is_array", []string{
			`p = x { is_array({1,2,3}, x) }`,
		}, ""},

		{"is_set", []string{
			`p = [x, y] { is_set({1,2,3}, x); is_set({"a", "b"}, y) }`,
		}, "[true, true]"},

		{"is_set", []string{
			`p = x { is_set([1,2,3], x) }`,
		}, ""},

		{"is_object", []string{
			`p = x { is_object({"foo": yy | yy = 1}, x) }`,
		}, "true"},

		{"is_object", []string{
			`p = x { is_object("foo", x) }`,
		}, ""},

		{"is_null", []string{
			`p = x { is_null(null, x) }`,
		}, "true"},

		{"is_null", []string{
			`p = x { is_null(true, x) }`,
		}, ""},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownTypeNameBuiltin(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"type_name", []string{
			`p = x { type_name(null, x) }`}, `"null"`},
		{"type_name", []string{
			`p = x { type_name(true, x) }`}, `"boolean"`},
		{"type_name", []string{
			`p = x { type_name(100, x) }`}, `"number"`},
		{"type_name", []string{
			`p = x { type_name("Hello", x) }`}, `"string"`},
		{"type_name", []string{
			`p = x { type_name([1,2,3], x) }`}, `"array"`},
		{"type_name", []string{
			`p = x { type_name({1,2,3}, x) }`}, `"set"`},
		{"type_name", []string{
			`p = x { type_name({"foo": yy | yy = 1}, x) }`}, `"object"`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}

}

func TestTopDownRegexMatch(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"re_match", []string{`p = true { re_match("^[a-z]+\\[[0-9]+\\]$", "foo[1]") }`}, "true"},
		{"re_match: undefined", []string{`p = true { re_match("^[a-z]+\\[[0-9]+\\]$", "foo[\"bar\"]") }`}, ""},
		{"re_match: bad pattern err", []string{`p = true { re_match("][", "foo[\"bar\"]") }`}, &Error{Code: BuiltinErr, Message: "re_match: error parsing regexp: missing closing ]: `[`"}},
		{"re_match: ref", []string{`p[x] { re_match("^b.*$", d.e[x]) }`}, "[0,1]"},

		{"re_match: raw", []string{fmt.Sprintf(`p = true { re_match(%s, "foo[1]") }`, "`^[a-z]+\\[[0-9]+\\]$`")}, "true"},
		{"re_match: raw: undefined", []string{fmt.Sprintf(`p = true { re_match(%s, "foo[\"bar\"]") }`, "`^[a-z]+\\[[0-9]+\\]$`")}, ""},
		{"regex.match", []string{`p = true { regex.match("^[a-z]+\\[[0-9]+\\]$", "foo[1]") }`}, "true"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownRegexSplit(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"regex.split: empty string", []string{`p = x { regex.split("^[a-z]+\\[[0-9]+\\]$", "", [x]) }`}, `""`},
		{"regex.split: non-repeat pattern", []string{`p = [v,w,x,y] { regex.split("a", "banana", [v,w,x,y]) }`}, `["b","n","n",""]`},
		{"regex.split: repeat pattern", []string{`p = [v,w] { regex.split("z+", "pizza", [v,w]) }`}, `["pi","a"]`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownGlobsMatch(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"regex.globs_match", []string{`p = true { regex.globs_match("a.a.[0-9]+z", ".b.b2359825792*594823z") }`}, "true"},
		{"regex.globs_match", []string{`p = true { regex.globs_match("[a-z]+", "[0-9]*") }`}, ""},
		{"regex.globs_match: bad pattern err", []string{`p = true { regex.globs_match("pqrs]", "[a-b]+") }`}, &Error{Code: BuiltinErr, Message: "input:pqrs], pos:5, set-close ']' with no preceding '[': the input provided is invalid"}},
		{"regex.globs_match: ref", []string{`p[x] { regex.globs_match("b.*", d.e[x]) }`}, "[0,1]"},

		{"regex.globs_match: raw", []string{fmt.Sprintf(`p = true { regex.globs_match(%s, "foo\\[1\\]") }`, "`[a-z]+\\[[0-9]+\\]`")}, "true"},
		{"regex.globs_match: raw: undefined", []string{fmt.Sprintf(`p = true { regex.globs_match(%s, "foo[\"bar\"]") }`, "`[a-z]+\\[[0-9]+\\]`")}, ""},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownSets(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"set_diff", []string{`p = x { s1 = {1, 2, 3, 4}; s2 = {1, 3}; x = s1 - s2 }`}, `[2,4]`},
		{"set_diff: refs", []string{`p = x { s1 = {a[2], a[1], a[0]}; s2 = {a[0], 2}; set_diff(s1, s2, x) }`}, "[3]"},
		{"set_diff: ground output", []string{`p = true { {1} = {1, 2, 3} - {2, 3} }`}, "true"},
		{"set_diff: virt docs", []string{`p = x { x = s1 - s2 }`, `s1[1] { true }`, `s1[2] { true }`, `s1["c"] { true }`, `s2 = {"c", 1} { true }`}, "[2]"},
		{"intersect", []string{`p = x { x = {a[1], a[2], 3} & {a[2], 4, 3} }`}, "[3]"},
		{"union", []string{`p = true { {2, 3, 4} = {a[1], a[2], 3} | {a[2], 4, 3} }`}, "true"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownStrings(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"format_int", []string{`p = x { format_int(15.5, 16, x) }`}, `"f"`},
		{"format_int: undefined", []string{`p = true { format_int(15.5, 16, "10000") }`}, ""},
		{"format_int: ref dest", []string{`p = true { format_int(3.1, 10, numbers[2]) }`}, "true"},
		{"format_int: ref dest (2)", []string{`p = true { not format_int(4.1, 10, numbers[2]) }`}, "true"},
		{"format_int: err: bad base", []string{`p = true { format_int(4.1, 199, x) }`}, &Error{Code: TypeErr, Message: "operand 2 must be one of {2, 8, 10, 16}"}},
		{"concat", []string{`p = x { concat("/", ["", "foo", "bar", "0", "baz"], x) }`}, `"/foo/bar/0/baz"`},
		{"concat: set", []string{`p = x { concat(",", {"1", "2", "3"}, x) }`}, `"1,2,3"`},
		{"concat: undefined", []string{`p = true { concat("/", ["a", "b"], "deadbeef") }`}, ""},
		{"concat: ref dest", []string{`p = true { concat("", ["f", "o", "o"], c[0].x[2]) }`}, "true"},
		{"concat: ref dest (2)", []string{`p = true { not concat("", ["b", "a", "r"], c[0].x[2]) }`}, "true"},
		{"indexof", []string{`p = x { indexof("abcdefgh", "cde", x) }`}, "2"},
		{"indexof: not found", []string{`p = x { indexof("abcdefgh", "xyz", x) }`}, "-1"},
		{"substring", []string{`p = x { substring("abcdefgh", 2, 3, x) }`}, `"cde"`},
		{"substring: remainder", []string{`p = x { substring("abcdefgh", 2, -1, x) }`}, `"cdefgh"`},
		{"substring: too long", []string{`p = x { substring("abcdefgh", 2, 10000, x) }`}, `"cdefgh"`},
		{"substring: offset negative", []string{`p = x { substring("aaa", -1, -1, x) }`}, &Error{Code: BuiltinErr, Message: "negative offset"}},
		{"substring: offset too long", []string{`p = x { substring("aaa", 3, -1, x) }`}, `""`},
		{"substring: offset too long 2", []string{`p = x { substring("aaa", 4, -1, x) }`}, `""`},
		{"contains", []string{`p = true { contains("abcdefgh", "defg") }`}, "true"},
		{"contains: undefined", []string{`p = true { contains("abcdefgh", "ac") }`}, ""},
		{"startswith", []string{`p = true { startswith("abcdefgh", "abcd") }`}, "true"},
		{"startswith: undefined", []string{`p = true { startswith("abcdefgh", "bcd") }`}, ""},
		{"endswith", []string{`p = true { endswith("abcdefgh", "fgh") }`}, "true"},
		{"endswith: undefined", []string{`p = true { endswith("abcdefgh", "fg") }`}, ""},
		{"lower", []string{`p = x { lower("AbCdEf", x) }`}, `"abcdef"`},
		{"upper", []string{`p = x { upper("AbCdEf", x) }`}, `"ABCDEF"`},
		{"split: empty string", []string{`p = x { split("", ".", [x]) }`}, `""`},
		{"split: one", []string{`p = x { split("foo", ".", [x]) }`}, `"foo"`},
		{"split: many", []string{`p = [x,y] { split("foo.bar.baz", ".", [x,"bar",y]) }`}, `["foo","baz"]`},
		{"replace: empty string", []string{`p = x { replace("", "hi", "bye", x) }`}, `""`},
		{"replace: one", []string{`p = x { replace("foo.bar", ".", ",", x) }`}, `"foo,bar"`},
		{"replace: many", []string{`p = x { replace("foo.bar.baz", ".", ",", x) }`}, `"foo,bar,baz"`},
		{"replace: overlap", []string{`p = x { replace("foo...bar", "..", ",,", x) }`}, `"foo,,.bar"`},
		{"trim: empty string", []string{`p = x { trim("", ".", x) }`}, `""`},
		{"trim: end", []string{`p = x { trim("foo.bar...", ".", x) }`}, `"foo.bar"`},
		{"trim: start", []string{`p = x { trim("...foo.bar", ".", x) }`}, `"foo.bar"`},
		{"trim: both", []string{`p = x { trim("...foo.bar...", ".", x) }`}, `"foo.bar"`},
		{"trim: multi-cutset", []string{`p = x { trim("...foo.bar...", ".fr", x) }`}, `"oo.ba"`},
		{"trim: multi-cutset-none", []string{`p = x { trim("...foo.bar...", ".o", x) }`}, `"foo.bar"`},
		{"sprintf: none", []string{`p = x { sprintf("hi", [], x) }`}, `"hi"`},
		{"sprintf: string", []string{`p = x { sprintf("hi %s", ["there"], x) }`}, `"hi there"`},
		{"sprintf: int", []string{`p = x { sprintf("hi %02d", [5], x) }`}, `"hi 05"`},
		{"sprintf: hex", []string{`p = x { sprintf("hi %02X.%02X", [127, 1], x) }`}, `"hi 7F.01"`},
		{"sprintf: float", []string{`p = x { sprintf("hi %.2f", [3.1415], x) }`}, `"hi 3.14"`},
		{"sprintf: float too big", []string{`p = x { sprintf("hi %v", [2e308], x) }`}, `"hi 2e308"`},
		{"sprintf: bool", []string{`p = x { sprintf("hi %s", [true], x) }`}, `"hi true"`},
		{"sprintf: composite", []string{`p = x { sprintf("hi %v", [["there", 5, 3.14]], x) }`}, `"hi [\"there\", 5, 3.14]"`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownJSONBuiltins(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"marshal", []string{`p = x { json.marshal([{"foo": {1,2,3}}], x) }`}, `"[{\"foo\":[1,2,3]}]"`},
		{"unmarshal", []string{`p = x { json.unmarshal("[{\"foo\":[1,2,3]}]", x) }`}, `[{"foo": [1,2,3]}]`},
		{"unmarshal-non-string", []string{`p = x { json.unmarshal(data.a[0], x) }`}, &Error{Code: TypeErr, Message: "operand 1 must be string but got number"}},
		{"yaml round-trip", []string{`p = y { yaml.marshal([{"foo": {1,2,3}}], x); yaml.unmarshal(x, y) }`}, `[{"foo": [1,2,3]}]`},
		{"yaml unmarshal error", []string{`p { yaml.unmarshal("[1,2,3", _) } `}, &Error{Code: BuiltinErr, Message: "yaml: line 1: did not find"}},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}

}

func TestTopDownBase64Builtins(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"encode-1", []string{`p = x { base64.encode("hello", x) }`}, `"aGVsbG8="`},
		{"encode-2", []string{`p = x { base64.encode("there", x) }`}, `"dGhlcmU="`},
		{"decode-1", []string{`p = x { base64.decode("aGVsbG8=", x) }`}, `"hello"`},
		{"decode-2", []string{`p = x { base64.decode("dGhlcmU=", x) }`}, `"there"`},
		{"encode-slash", []string{`p = x { base64.encode("subjects?_d", x) }`}, `"c3ViamVjdHM/X2Q="`},
		{"decode-slash", []string{`p = x { base64.decode("c3ViamVjdHM/X2Q=", x) }`}, `"subjects?_d"`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownBase64UrlBuiltins(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"encode-1", []string{`p = x { base64url.encode("hello", x) }`}, `"aGVsbG8="`},
		{"encode-2", []string{`p = x { base64url.encode("there", x) }`}, `"dGhlcmU="`},
		{"decode-1", []string{`p = x { base64url.decode("aGVsbG8=", x) }`}, `"hello"`},
		{"decode-2", []string{`p = x { base64url.decode("dGhlcmU=", x) }`}, `"there"`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownURLBuiltins(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"encode", []string{`p = x { urlquery.encode("a=b+1", x) }`}, `"a%3Db%2B1"`},
		{"encode empty", []string{`p = x { urlquery.encode("", x) }`}, `""`},
		{"decode", []string{`p = x { urlquery.decode("a%3Db%2B1", x) }`}, `"a=b+1"`},
		{"encode_object empty", []string{`p = x { urlquery.encode_object({}, x) }`}, `""`},
		{"encode_object strings", []string{`p = x { urlquery.encode_object({"a": "b", "c": "d"}, x) }`}, `"a=b&c=d"`},
		{"encode_object escape", []string{`p = x { urlquery.encode_object({"a": "c=b+1"}, x) }`}, `"a=c%3Db%2B1"`},
		{"encode_object array", []string{`p = x { urlquery.encode_object({"a": ["b+1","c+2"]}, x) }`}, `"a=b%2B1&a=c%2B2"`},
		{"encode_object set", []string{`p = x { urlquery.encode_object({"a": {"b+1"}}, x) }`}, `"a=b%2B1"`},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownTime(t *testing.T) {

	data := loadSmallTestData()

	runTopDownTestCase(t, data, "time caching", []string{`
		p { time.now_ns(t0); test.sleep("10ms"); time.now_ns(t1); t1 = t2 }
	`}, "true")

	runTopDownTestCase(t, data, "parse nanos", []string{`
		p = ns { time.parse_ns("2006-01-02T15:04:05Z07:00", "2017-06-02T19:00:00-07:00", ns) }
	`}, "1496455200000000000")

	runTopDownTestCase(t, data, "parse rfc3339 nanos", []string{`
		p = ns { time.parse_rfc3339_ns("2017-06-02T19:00:00-07:00", ns) }
		`}, "1496455200000000000")

	runTopDownTestCase(t, data, "parse duration nanos", []string{`
		p = ns { time.parse_duration_ns("100ms", ns) }
	`}, "100000000")

	runTopDownTestCase(t, data, "date", []string{`
		p = [year, month, day] { [year, month, day] := time.date(1517814000*1000*1000*1000) }`}, "[2018, 2, 5]")

	runTopDownTestCase(t, data, "date with LA tz", []string{`
		p = [year, month, day] { [year, month, day] := time.date([ 1517814000*1000*1000*1000, "America/Los_Angeles" ]) }`}, "[2018, 2, 4]")

	runTopDownTestCase(t, data, "date with empty tz", []string{`
		p = [year, month, day] { [year, month, day] := time.date([ 1517832000*1000*1000*1000, "" ]) }`}, "[2018, 2, 5]")

	runTopDownTestCase(t, data, "date leap day", []string{`
		p = [year, month, day] { [year, month, day] := time.date(1582977600*1000*1000*1000) }`}, "[2020, 2, 29]")

	runTopDownTestCase(t, data, "date too big", []string{`
		p = [year, month, day] { [year, month, day] := time.date(1582977600*1000*1000*1000*1000) }`}, &Error{Code: BuiltinErr, Message: "timestamp too big"})

	runTopDownTestCase(t, data, "clock", []string{`
		p = [hour, minute, second] { [hour, minute, second] := time.clock(1517832000*1000*1000*1000) }`}, "[12, 0, 0]")

	runTopDownTestCase(t, data, "clock with NY tz", []string{`
		p = [hour, minute, second] { [hour, minute, second] := time.clock([ 1517832000*1000*1000*1000, "America/New_York" ]) }`}, "[7, 0, 0]")

	runTopDownTestCase(t, data, "clock leap day", []string{`
		p = [hour, minute, second] { [hour, minute, second] := time.clock(1582977600*1000*1000*1000) }`}, "[12, 0, 0]")

	runTopDownTestCase(t, data, "clock too big", []string{`
		p = [hour, minute, second] { [hour, minute, second] := time.clock(1582977600*1000*1000*1000*1000) }`}, &Error{Code: BuiltinErr, Message: "timestamp too big"})

	for i, day := range []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"} {
		ts := 1517832000*1000*1000*1000 + i*24*int(time.Hour)
		runTopDownTestCase(t, data, "weekday", []string{fmt.Sprintf(`p = weekday { weekday := time.weekday(%d)}`, ts)},
			fmt.Sprintf("%q", day))
	}

	runTopDownTestCase(t, data, "weekday too big", []string{`
		p = weekday { weekday := time.weekday(1582977600*1000*1000*1000*1000) }`}, &Error{Code: BuiltinErr, Message: "timestamp too big"})

	runTopDownTestCase(t, data, "add_date year month day", []string{`
		p = ns { ns := time.add_date(1585852421593912000, 3, 9, 12) }`}, "1705257221593912000")

	runTopDownTestCase(t, data, "add_date negative values", []string{`
		p = ns { ns := time.add_date(1585852421593912000, -1, -1, -1) }`}, "1551465221593912000")
}

func TestTopDownWalkBuiltin(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{
			note: "scalar",
			rules: []string{
				`p[x] { walk(data.a[0], x) }`,
			},
			expected: `[
				[[], 1]
			]`,
		},
		{
			note: "arrays",
			rules: []string{
				`p[x] { walk(data.a, x) }`,
			},
			expected: `[
				[[], [1,2,3,4]],
				[[0], 1],
				[[1], 2],
				[[2], 3],
				[[3], 4]
			]`,
		},
		{
			note: "objects",
			rules: []string{
				"p[x] { walk(data.b, x) }",
			},
			expected: `[
				[[], {"v1": "hello", "v2": "goodbye"}],
				[["v1"], "hello"],
				[["v2"], "goodbye"]
			]`,
		},
		{
			note: "sets",
			rules: []string{
				"p[x] { walk(q, x) }",
				`q = {{1,2,3}} { true }`,
			},
			expected: `[
				[[], [[1,2,3]]],
				[[[1,2,3]], [1,2,3]],
				[[[1,2,3], 1], 1],
				[[[1,2,3], 2], 2],
				[[[1,2,3], 3], 3]
			]`,
		},
		{
			note: "match and filter",
			rules: []string{
				`p[[k,x]] { walk(q, [k, x]); contains(k[1], "oo") }`,
				`q = [
					{
						"foo": 1,
						"bar": 2,
						"bazoo": 3,
					}
				] { true }`,
			},
			expected: `[[[0, "foo"], 1], [[0, "bazoo"], 3]]`,
		},
		{
			note: "partially ground path",
			rules: []string{
				`p[[k1,k2,x]] {
					walk(q, [["a", k1, "b", k2], x])
				}`,
				`q = {
					"a": [
						{
							"b": {"foo": 1, "bar": 2},
						},
						{
							"b": {"baz": 3, "qux": 4},
						}
					]
				} { true }
				`,
			},
			expected: `[[0, "foo", 1], [0, "bar", 2], [1, "baz", 3], [1, "qux", 4]]`,
		},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, tc.rules, tc.expected)
	}

}

func TestTopDownEmbeddedVirtualDoc(t *testing.T) {

	compiler := compileModules([]string{
		`package b.c.d

import data.a
import data.g

p[x] { a[i] = x; q[x] }
q[x] { g[j][k] = x }`})

	store := inmem.NewFromObject(loadSmallTestData())

	assertTopDownWithPath(t, compiler, store, "deep embedded vdoc", []string{"b", "c", "d", "p"}, "{}", "[1, 2, 4]")
}

func TestTopDownInputValues(t *testing.T) {
	compiler := compileModules([]string{
		`package z

import data.a
import input.req1
import input.req2 as req2as
import input.req3.a.b
import input.req4.a.b as req4as

p = true { a[i] = x; req1.foo = x; req2as.bar = x; q[x] }
q[x] { req1.foo = x; req2as.bar = x; r[x] }
r[x] { {"foo": req2as.bar, "bar": [x]} = {"foo": x, "bar": [req1.foo]} }
s = true { b.x[0] = 1 }
t = true { req4as.x[0] = 1 }
u[x] { b[_] = x; x > 1 }
w = [[1, 2], [3, 4]] { true }
gt1 = true { req1 > 1 }
keys[x] = y { data.numbers[_] = x; to_number(x, y) }
loopback = input { true }
sets { input.foo[{1}][1] = 1 } `})

	store := inmem.NewFromObject(loadSmallTestData())

	assertTopDownWithPath(t, compiler, store, "loopback", []string{"z", "loopback"}, `{"foo": 1}`, `{"foo": 1}`)

	assertTopDownWithPath(t, compiler, store, "loopback undefined", []string{"z", "loopback"}, ``, ``)

	assertTopDownWithPath(t, compiler, store, "simple", []string{"z", "p"}, `{
		"req1": {"foo": 4},
		"req2": {"bar": 4}
	}`, "true")

	assertTopDownWithPath(t, compiler, store, "missing", []string{"z", "p"}, `{
		"req1": {"foo": 4}
	}`, "")

	assertTopDownWithPath(t, compiler, store, "namespaced", []string{"z", "s"}, `{
		"req3": {
			"a": {
				"b": {
					"x": [1,2,3,4]
				}
			}
		}
	}`, "true")

	assertTopDownWithPath(t, compiler, store, "namespaced with alias", []string{"z", "t"}, `{
		"req4": {
			"a": {
				"b": {
					"x": [1,2,3,4]
				}
			}
		}
	}`, "true")

	assertTopDownWithPath(t, compiler, store, "input set", []string{"z", "sets"}, `{"foo": {{1}}}`, `true`)
}

func TestTopDownPartialDocConstants(t *testing.T) {
	compiler := compileModules([]string{
		`package ex

		foo["bar"] = 0
		foo["baz"] = 1
		foo["*"] = [1, 2, 3] {
			input.foo = 7
		}

		bar["x"]
		bar["y"]
		bar["*"] {
			input.foo = 7
		}
	`})

	store := inmem.NewFromObject(loadSmallTestData())
	ctx := context.Background()
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tests := []struct {
		note     string
		path     string
		input    string
		expected string
	}{
		{
			note:     "obj-1",
			path:     "ex.foo.bar",
			expected: "0",
		},
		{
			note:     "obj",
			path:     "ex.foo",
			expected: `{"bar": 0, "baz": 1}`,
		},
		{
			note:     "obj-all",
			path:     "ex.foo",
			input:    `{"foo": 7}`,
			expected: `{"bar": 0, "baz": 1, "*": [1,2,3]}`,
		},
		{
			note:     "set-1",
			path:     "ex.bar.x",
			expected: `"x"`,
		},
		{
			note:     "set",
			path:     "ex.bar",
			expected: `["x", "y"]`,
		},
		{
			note:     "set-all",
			path:     "ex.bar",
			input:    `{"foo": 7}`,
			expected: `["x", "y", "*"]`,
		},
	}

	for _, tc := range tests {
		assertTopDownWithPath(t, compiler, store, tc.note, strings.Split(tc.path, "."), tc.input, tc.expected)
	}
}

func TestTopDownFunctions(t *testing.T) {
	modules := []string{`package ex

		foo(x) = y {
			split(x, "i", y)
		}

		bar[x] = y {
			data.l[_].a = x
			foo(x, y)
		}

		chain0(x) = y {
			foo(x, y)
		}

		chain1(a) = b {
			chain0(a, b)
		}

		chain2 = d {
			chain1("fooibar", d)
		}

		cross(x) = [a, b] {
			split(x, "i", y)
			foo(y[1], b)
			data.test.foo(y[2], a)
		}

		falsy_func(x) = false

		falsy_func_else(x) = true { x = 1 } else = false { true }

		falsy_undefined {
			falsy_func(1)
		}

		falsy_negation {
			not falsy_func(1)
		}

		falsy_else_value = falsy_func_else(2)

		falsy_else_undefined {
			falsy_func_else(2)
		}

		falsy_else_negation {
			not falsy_func_else(2)
		}

		arrays([x, y]) = [a, b] {
			foo(x, a)
			foo(y, b)
		}

		arraysrule = y {
			arrays(["hih", "foo"], y)
		}

		objects({"foo": x, "bar": y}) = z {
			foo(x, a)
			data.test.foo(y, b)
			z = [a, b]
		}

		objectsrule = y {
			objects({"foo": "hih", "bar": "hi ho"}, y)
		}

		refoutput = y {
			foo("hih", z)
			y = z[1]
		}

		void(x) {
			x = "foo"
		}

		voidGood {
			not void("bar", true)
		}

		voidBad {
			void("bar", true)
		}

		multi(1, x) = y {
			y = x
		}

		multi(2, x) = y {
			a = 2*x
			y = a+1
		}

		multi(3, x) = y {
			y = x*10
		}

		multi("foo", x) = y {
			y = "bar"
		}

		multi1 = y {
			multi(1, 2, y)
		}

		multi2 = y {
			multi(2, 2, y)
		}

		multi3 = y {
			multi(3, 2, y)
		}

		multi4 = y {
			multi("foo", 2, y)
		}

		always_true_fn(x)

		always_true {
			always_true_fn(1)
		}
		`,
		`
		package test

		import data.ex

		foo(x) = y {
			trim(x, "h o", y)
		}

		cross = y {
			ex.cross("hi, my name is foo", y)
		}

		multi("foo", x) = y {
			y = x
		}

		multi("bar", x) = y {
			y = "baz"
		}

		multi_cross_pkg = [y, z] {
			multi("foo", "bar", y)
			ex.multi(2, 1, z)
		}`,
		`
		package test

		samepkg = y {
			foo("how do you do?", y)
		}`,
		`
		package test.l1.l3

		g(x) = x`,
		`
		package test.l1.l2

		p = true
		f(x) = x`,
		`
		package test.omit_result

		f(x) = x

		p { f(1) }
		`,
	}

	compiler := compileModules(modules)
	store := inmem.NewFromObject(loadSmallTestData())
	ctx := context.Background()
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	assertTopDownWithPath(t, compiler, store, "basic call", []string{"ex", "bar", "alice"}, "", `["al", "ce"]`)
	assertTopDownWithPath(t, compiler, store, "false result", []string{"ex", "falsy_undefined"}, "", ``)
	assertTopDownWithPath(t, compiler, store, "false result negation", []string{"ex", "falsy_negation"}, "", `true`)
	assertTopDownWithPath(t, compiler, store, "false else value", []string{"ex", "falsy_else_value"}, "", `false`)
	assertTopDownWithPath(t, compiler, store, "false else undefined", []string{"ex", "falsy_else_undefined"}, "", ``)
	assertTopDownWithPath(t, compiler, store, "false else negation", []string{"ex", "falsy_else_negation"}, "", `true`)
	assertTopDownWithPath(t, compiler, store, "chained", []string{"ex", "chain2"}, "", `["foo", "bar"]`)
	assertTopDownWithPath(t, compiler, store, "cross package", []string{"test", "cross"}, "", `["s f", [", my name "]]`)
	assertTopDownWithPath(t, compiler, store, "array params", []string{"ex", "arraysrule"}, "", `[["h", "h"], ["foo"]]`)
	assertTopDownWithPath(t, compiler, store, "object params", []string{"ex", "objectsrule"}, "", `[["h", "h"], "i"]`)
	assertTopDownWithPath(t, compiler, store, "ref func output", []string{"ex", "refoutput"}, "", `"h"`)
	assertTopDownWithPath(t, compiler, store, "always_true", []string{"ex.always_true"}, ``, `true`)
	assertTopDownWithPath(t, compiler, store, "same package call", []string{"test", "samepkg"}, "", `"w do you do?"`)
	assertTopDownWithPath(t, compiler, store, "void good", []string{"ex", "voidGood"}, "", `true`)
	assertTopDownWithPath(t, compiler, store, "void bad", []string{"ex", "voidBad"}, "", "")
	assertTopDownWithPath(t, compiler, store, "multi1", []string{"ex", "multi1"}, "", `2`)
	assertTopDownWithPath(t, compiler, store, "multi2", []string{"ex", "multi2"}, "", `5`)
	assertTopDownWithPath(t, compiler, store, "multi3", []string{"ex", "multi3"}, "", `20`)
	assertTopDownWithPath(t, compiler, store, "multi4", []string{"ex", "multi4"}, "", `"bar"`)
	assertTopDownWithPath(t, compiler, store, "multi cross package", []string{"test", "multi_cross_pkg"}, "", `["bar", 3]`)
	assertTopDownWithPath(t, compiler, store, "skip-functions", []string{"test.l1"}, ``, `{"l2": {"p": true}, "l3": {}}`)
	assertTopDownWithPath(t, compiler, store, "omit result", []string{"test.omit_result.p"}, ``, `true`)
}

func TestTopDownFunctionErrors(t *testing.T) {
	compiler := compileModules([]string{
		`
		package test1

		p(x) = y {
			y = x[_]
		}

		r = y {
			p([1, 2, 3], y)
		}`,
		`
		package test2

		p(1, x) = y {
			y = x
		}

		p(2, x) = y {
			y = x+1
		}

		r = y {
			p(3, 0, y)
		}`,
		`
		package test3

		p(1, x) = y {
			y = x
		}

		p(2, x) = y {
			y = x+1
		}

		p(x, y) = z {
			z = x
		}

		r = y {
			p(1, 0, y)
		}`,
	})

	store := inmem.NewFromObject(loadSmallTestData())
	ctx := context.Background()
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	assertTopDownWithPath(t, compiler, store, "function output conflict single", []string{"test1", "r"}, "", functionConflictErr(nil))
	assertTopDownWithPath(t, compiler, store, "function input no match", []string{"test2", "r"}, "", "")
	assertTopDownWithPath(t, compiler, store, "function output conflict multiple", []string{"test3", "r"}, "", functionConflictErr(nil))
}

func TestTopDownWithKeyword(t *testing.T) {

	tests := []struct {
		note    string
		rules   []string
		modules []string
		input   string
		exp     interface{}
	}{

		{
			note: "with",
			exp:  `true`,
			modules: []string{`package ex
			loopback = input`},
			rules: []string{`p { data.ex.loopback with input as true; data.ex.loopback = false with input as false }`},
		},
		{
			note: "with not",
			exp:  `true`,
			modules: []string{`package ex
			loopback = input`},
			rules: []string{`p = true { not data.ex.loopback with input as false; data.ex.loopback with input as true }`},
		},
		{
			note: "with composite",
			exp:  `[3,4]`,
			modules: []string{`package ex
			composite[x] { input.foo[_] = x; gt(x, 2) }`},
			rules: []string{`p[x] { data.ex.composite[x] with input.foo as [1, 2, 3, 4] }`},
		},
		{
			note: "with vars",
			exp:  `{"foo": "hello", "bar": "world"}`,
			modules: []string{`package ex
			vars = x { y = input.bar; z = input.foo; x = {"bar": y, "foo": z} }`},
			rules: []string{`p = x { foo = "hello"; bar = "world"; x = data.ex.vars with input.foo as foo with input.bar as bar }`},
		},
		{
			note: "with conflict",
			exp:  &Error{Code: ConflictErr, Message: errConflictingDoc.Error()},
			modules: []string{`package ex
			loopback = __local0__ { true; __local0__ = input }`},
			rules: []string{`p = true { data.ex.loopback with input.foo as "x" with input.foo.bar as "y" }`},
		},
		{
			note:  "with stack",
			input: `{"a": {"d": 3}, "e": 4}`,
			exp:   `{"a": {"b": 1, "c": 2, "d": 3}, "e": 4}`,
			rules: []string{
				`r = input { true }`,
				`q = x { r = x with input.a.c as 2 }`,
				`p = x { q = x with input.a.b as 1 }`,
			},
		}, {
			note:  "with not stack",
			input: `{"a": {"d": 3}, "e": 4}`,
			exp:   `{"a": {"b": 1, "c": 2, "d": 3}, "e": 4}`,
			rules: []string{
				`r = input { true }`,
				`q = x { not false with input as {}; r = x with input.a.c as 2 }`,
				`p = x { q = x with input.a.b as 1 }`,
			},
		},
		{
			note: "with stack (data)",
			exp:  `{"a": {"b": 1, "c": 2, "d": 3}, "e": 4}`,
			modules: []string{
				`package test.a
				d = 3`,
				`package test
				e = 4`,
			},
			rules: []string{
				`r = data.test { true }`,
				`q = x { r = x with data.test.a.c as 2 }`,
				`p = x { q = x with data.test.a.b as 1 }`,
			},
		},
		{
			note: "with not stack (data)",
			exp:  `{"a": {"b": 1, "c": 2, "d": 3}, "e": 4}`,
			modules: []string{
				`package test.a
				d = 3`,
				`package test
				e = 4`,
			},
			rules: []string{
				`r = data.test { true }`,
				`n1 { data.test.a.z == 7 }`,
				`n { not n1 } `,
				`q = x { not n with data.test.a.z as 7; r = x with data.test.a.c as 2 }`,
				`p = x { q = x with data.test.a.b as 1 }`,
			},
		},
		{
			note:  "with stack overwrites",
			input: `{"a": {"b": 1, "c": 2}}`,
			exp:   `{"a": {"d": 3}}`,
			rules: []string{
				`q = input { true }`,
				`p = x { q = x with input.a as {"d": 3} }`,
			},
		},
		{
			note: "with stack overwrites (data)",
			exp:  `{"a": {"d": 3}}`,
			modules: []string{
				`package test

				a = {"b": 1, "c": 2}`,
			},
			rules: []string{
				`q = data.test { true }`,
				`p = x { q = x with data.test.a as {"d": 3} }`,
			},
		},
		{
			note: "with invalidate",
			exp:  `[2,3,4]`,
			modules: []string{`package ex
			input_eq = true { input.x = 1 }`},
			rules: []string{`p[x] { data.a[_] = x; not data.ex.input_eq with input.x as x }`},
		},
		{
			note:  "with invalidate input stack",
			exp:   `["a", "b"]`,
			input: `"b"`,
			rules: []string{
				`p = [x, y] { x = input with input as "a"; y = input }`,
			},
		},
		{
			note:  "with invalidate input stack iteration",
			exp:   `[["a", "c"], ["b", "c"]]`,
			input: `"c"`,
			rules: []string{
				`q[x] { input[_] = x }`,
				`p[[x,y]] {
					q[x] with input as ["a", "b"]
					y = input
				}`,
			},
		},
		{
			note:  "with invalidate virtual cache",
			exp:   `["a", "b"]`,
			input: "2",
			rules: []string{
				`q = "a" { input = x; x = 1 }`,
				`q = "b" { input = x; x = 2 }`,
				`p = [x, y] {
					q = x with input as 1
					q = y
				}`},
		},
		{
			note: "with invalidate data stack",
			exp:  `["a", "b"]`,
			rules: []string{
				`q = "b" { true }`,
				`p = [x ,y] {
					q = x with q as "a"
					q = y
				}`,
			},
		},
		{
			note: "with invalidate data stack iteration",
			exp:  `[["a", ["c"]], ["b", ["c"]]]`,
			rules: []string{
				`q["c"] { true }`,
				`p[[x, y]] {
					q[x] with q as {"a", "b"}
					y = q
				}`,
			},
		},
		{
			note: "with basic data",
			exp:  `true`,
			modules: []string{`package ex
			allow_basic = true { data.a = "testdata" }`},
			rules: []string{`p = true { data.ex.allow_basic = true with data.a as "testdata" }`},
		},
		{
			note: "with map data overwrite",
			exp:  `true`,
			modules: []string{`package ex
			allow_merge_1 = true { data.b = {"v1": "hello", "v2": "world"} }`},
			rules: []string{`p = true { data.ex.allow_merge_1 = true with data.b.v2 as "world" }`},
		},
		{
			note: "with map data new key",
			exp:  `true`,
			modules: []string{`package ex
			allow_merge_2 = true { data.b = {"v1": "hello", "v2": "world", "v3": "again"} }`},
			rules: []string{`p = true { data.ex.allow_merge_2 = true with data.b.v2 as "world" with data.b.v3 as "again" }`},
		},
		{
			note: "with data conflict",
			exp:  mergeConflictErr(nil),
			modules: []string{`package ex
			allow_basic = true { data.a = "testdata" }`},
			rules: []string{`p = true { data.ex.allow_basic = true with data.a.b as 5 }`},
		},
		{
			note:  "with base doc exact value",
			exp:   `["c", "e"]`,
			rules: []string{`p[x] { data.a.b[x] = 1 with data.a.b as {"c": 1, "d": 2, "e": 1} }`},
		},
		{
			note:  "with base doc any index",
			exp:   `["c", "d", "e"]`,
			rules: []string{`p[x] { data.a.b[x] with data.a.b as {"c": 1, "d": 2, "e": 1} }`},
		},
		{
			note:  "undefined_1",
			exp:   "",
			rules: []string{`p = true { data.a.b.c with data.a.b as 1 }`},
		},
		{
			note:  "undefined_2",
			exp:   "",
			rules: []string{`p = true { data.l.a with data.l as 1 }`},
		},
		{
			note: "with virtual doc exact value",
			exp:  `[["c", "e"]]`,
			modules: []string{`package ex
			virtual[x] { data.a.b[x] = 1 }`},
			rules: []string{`p[x] { data.ex.virtual = x with data.a.b as {"c": 1, "d": 2, "e": 1} }`},
		},
		{
			note: "with virtual doc any index",
			exp:  `["c", "e"]`,
			modules: []string{`package ex
			virtual[x] { data.a.b[x] = 1 }`},
			rules: []string{`p[x] { data.ex.virtual[x] with data.a.b as {"c": 1, "d": 2, "e": 1} }`},
		},
		{
			note: "with virtual doc specific index",
			exp:  `"c"`,
			modules: []string{`package ex
			virtual[x] { data.a.b[x] = 1 }`},
			rules: []string{`p = y { y = data.ex.virtual.c with data.a.b as {"c": 1, "d": 2, "e": 1} }`},
		},
		{
			note: "with virtual doc not specific index",
			exp:  `true`,
			modules: []string{`package ex
			virtual[x] { data.a.b[x] = 1 }`},
			rules: []string{`p = true { not data.ex.virtual.d with data.a.b as {"c": 1, "d": 2, "e": 1} }`},
		},
		{
			note: "with mock var",
			exp:  `{"c": 1, "d": 2}`,
			modules: []string{`package ex
			mock_var = {"a": 0, "b": 0} { true }`},
			rules: []string{`p = y { y = data.ex.mock_var with data.ex.mock_var as {"c": 1, "d": 2} }`},
		},
		{
			note: "with mock rule",
			exp:  `true`,
			modules: []string{`package ex
			mock_rule = false { 1 = 2 }`},
			rules: []string{`p = true { data.ex.mock_rule with data.ex.mock_rule as true }`},
		},
		{
			note: "with rule chain",
			exp:  `true`,
			modules: []string{`package ex
			allow1 = true { data.label.b.c = [1, 2, 3] }
			allow2 = true { data.label.b.c[x] = 2 }
			allow3 = true { data.label.b[x] = 1 }
			allow4 = true { data.label.b.c.d[x] = 1 }
			allow = true { data.ex.allow1; data.ex.allow2; not data.ex.allow3; not data.ex.allow4 }`},
			rules: []string{`p = true { data.ex.allow with data.label.b.c as [1, 2, 3] }`},
		},
		{
			note: "with mock iteration on sets",
			exp:  `[3,4]`,
			rules: []string{
				`q[1] { true }`,
				`q[2] { true }`,
				`p[x] { q[x] with q as {3,4} }`,
			},
		},
		{
			note: "with mock iteration on objects",
			exp:  `{"a": 3, "c": 4}`,
			rules: []string{
				`q["a"] = 1 { true }`,
				`q["b"] = 2 { true }`,
				`p[x] = y { q[x] = y with q as {"a": 3, "c": 4} }`,
			},
		},
		{
			note: "with mock iteration on arrays",
			exp:  `[3, 4]`,
			rules: []string{
				`q[1] { true }`,
				`q[2] { true }`,
				`p[x] { q[_] = x with q as [3,4] }`,
			},
		},
		{
			note: "bug 1083",
			exp:  ``,
			modules: []string{`package ex
			input_eq = true { input.x = 1 }`},
			rules: []string{`p = true { data.ex.input_eq with data.foo as 1 }`},
		},
		{
			note: "bug 1100",
			exp:  `true`,
			modules: []string{`package ex
			data_eq = true { data.a = x }`},
			rules: []string{`p = true { data.ex.data_eq with input as {} }`},
		},
		{
			note: "set lookup",
			exp:  `true`,
			modules: []string{`package ex
			setl[x] { data.foo[x] }`},
			rules: []string{`p = true { data.ex.setl[1] with data.foo as {1} }`},
		},
		{
			// NOTE(tsandall): This case assumes that partial sets are not memoized.
			// If we change that, it'll be harder to test that the comprehension
			// cache is invalidated.
			note: "invalidate comprehension cache",
			exp:  `[[{"b": ["a", "c"]}], [{"b": ["a"]}]]`,
			modules: []string{`package ex
				s[x] {
					x = {v: ks |
						v = input[i]
						ks = {k | v = input[k]}
					}
				}
			`},
			rules: []string{`p = [x, y] {
				x = data.ex.s with input as {"a": "b", "c": "b"}
				y = data.ex.s with input as {"a": "b"}
			}`},
		},
	}

	for _, tc := range tests {
		runTopDownTestCaseWithModules(t, loadSmallTestData(), tc.note, tc.rules, tc.modules, tc.input, tc.exp)
	}
}

func TestTopDownElseKeyword(t *testing.T) {
	tests := []struct {
		note     string
		path     string
		expected interface{}
	}{
		{"no-op", "ex.no_op", "true"},
		{"trivial", "ex.bool", "true"},
		{"trivial-non-bool", "ex.non_bool", "[100]"},
		{"trivial-3", "ex.triple", `"hello"`},
		{"var-head", "ex.vars", `["hello", "goodbye"]`},
		{"ref-head", "ex.refs", `["hello", "goodbye"]`},
		{"first-match", "ex.multiple_defined", `true`},
		{"default-1", "ex.default_1", "2"},
		{"default-2", "ex.default_2", "2"},
		{"multiple-roots", "ex.multiple_roots", `2`},
		{"indexed", "ex.indexed", "2"},
		{"conflict-1", "ex.conflict_1", completeDocConflictErr(nil)},
		{"conflict-2", "ex.conflict_2", completeDocConflictErr(nil)},
		{"functions", "ex.fn_result", `["large", "small", "medium"]`},
	}

	for _, tc := range tests {

		compiler := compileModules([]string{
			`package ex

			no_op { true } else = false { true }
			bool { false } else { true }
			non_bool = null { false } else = [100] { true }
			triple { false } else { false } else = "hello" { true }
			vars { false } else = ["hello", x] { data.b.v2 = x }
			refs { false } else = ["hello", data.b.v2] { true }
			multiple_defined = false { false } else = true { true } else = false { true }

			default default_1 = 1
			default_1 { false } default_1 = 2 { true }

			default default_2 = 2
			default_2 { false } default_2 = 1 { false }

			multiple_roots {
				false
			} else = 1 {
				false
			} else = 2 {
				true
			} else = 3 {
				true
			}

			multiple_roots = 2

			multiple_roots = 3 {
				false
			} else = 2 {
				true
			}

			indexed {
				data.a[0] = 0
			} else = 2 {
				data.a[0] = 1
			} else = 3 {
				data.a[0] = 1
			}

			indexed {
				data.a[0] = 1
				data.a[2] = 2
			} else {
				false
			} else = 2 {
				data.a[0] = x
				x = 1
				data.a[2] = 3
			}

			conflict_1 { false } else { true }
			conflict_1 = false { true }

			conflict_2 { false } else = false { true }
			conflict_2 { false } else = true { true }

			fn_result = [x,y,z] { fn(101, true, x); fn(100, true, y); fn(100, false, z) }

			fn(x, y) = "large" {
				x > 100
			} else = "small" {
				y = true
			} else = "medium" {
				true
			}
			`,
		})

		store := inmem.NewFromObject(loadSmallTestData())

		assertTopDownWithPath(t, compiler, store, tc.note, strings.Split(tc.path, "."), "", tc.expected)
	}
}

// Test that dynamic dispatch is not broken by the recursion check.
func TestTopdownDynamicDispatch(t *testing.T) {
	compiler := compileModules([]string{`
		package animals

		dog = "woof"
		cat = "meow"
	`, `
		package dynamic

		sound = data.animals[animal]
		animal = "dog" {
			2 > 1
		}
	`})

	data := map[string]interface{}{}
	store := inmem.NewFromObject(data)

	assertTopDownWithPath(t, compiler, store, "dynamic dispatch", []string{}, `{}`, `{
		"animals": {
			"cat": "meow",
			"dog": "woof"
		},
		"dynamic": {
			"animal": "dog",
			"sound": "woof"
		}
	}`)
}

func TestTopDownSystemDocument(t *testing.T) {

	compiler := compileModules([]string{`
		package system.somepolicy

		foo = "hello"
	`, `
		package topdown.system

		bar = "goodbye"
	`})

	data := map[string]interface{}{
		"system": map[string]interface{}{
			"somedata": []interface{}{"a", "b", "c"},
		},
		"com": map[string]interface{}{
			"system": "deadbeef",
		},
	}

	store := inmem.NewFromObject(data)

	assertTopDownWithPath(t, compiler, store, "root query", []string{}, `{}`, `{
		"topdown": {
			"system": {
				"bar": "goodbye"
			}
		},
		"com": {
			"system": "deadbeef"
		}
	}`)

}

func TestExample(t *testing.T) {

	bd := `
        {
            "servers": [
                {"id": "s1", "name": "app", "protocols": ["https", "ssh"], "ports": ["p1", "p2", "p3"]},
                {"id": "s2", "name": "db", "protocols": ["mysql"], "ports": ["p3"]},
                {"id": "s3", "name": "cache", "protocols": ["memcache", "http"], "ports": ["p3"]},
                {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
            ],
            "networks": [
                {"id": "n1", "public": false},
                {"id": "n2", "public": false},
                {"id": "n3", "public": true}
            ],
            "ports": [
                {"id": "p1", "networks": ["n1"]},
                {"id": "p2", "networks": ["n3"]},
                {"id": "p3", "networks": ["n2"]}
            ]
        }
    `

	vd := `package opa.example

import data.servers
import data.networks
import data.ports

public_servers[server] { server = servers[_]; server.ports[_] = ports[i].id; ports[i].networks[_] = networks[j].id; networks[j].public = true }
violations[server] { server = servers[_]; server.protocols[_] = "http"; public_servers[server] }`

	var doc map[string]interface{}

	if err := util.UnmarshalJSON([]byte(bd), &doc); err != nil {
		panic(err)
	}

	compiler := compileModules([]string{vd})

	store := inmem.NewFromObject(doc)

	assertTopDownWithPath(t, compiler, store, "public servers", []string{"opa", "example", "public_servers"}, "{}", `
        [
            {"id": "s1", "name": "app", "protocols": ["https", "ssh"], "ports": ["p1", "p2", "p3"]},
            {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
        ]
    `)

	assertTopDownWithPath(t, compiler, store, "violations", []string{"opa", "example", "violations"}, "{}", `
	    [
	        {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
	    ]
	`)

	assertTopDownWithPath(t, compiler, store, "both", []string{"opa", "example"}, "{}", `
		{
			"public_servers": [
				{"id": "s1", "name": "app", "protocols": ["https", "ssh"], "ports": ["p1", "p2", "p3"]},
				{"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
			],
			"violations": [
				{"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
			]
		}
	`)
}

func TestTopDownUnsupportedBuiltin(t *testing.T) {

	ast.RegisterBuiltin(&ast.Builtin{
		Name: "unsupported_builtin",
	})

	body := ast.MustParseBody(`unsupported_builtin()`)
	ctx := context.Background()
	compiler := ast.NewCompiler()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	q := NewQuery(body).WithCompiler(compiler).WithStore(store).WithTransaction(txn)
	_, err := q.Run(ctx)

	expected := unsupportedBuiltinErr(body[0].Location)

	if !reflect.DeepEqual(err, expected) {
		t.Fatalf("Expected %v but got: %v", expected, err)
	}

}

func TestTopDownQueryCancellation(t *testing.T) {

	ctx := context.Background()

	compiler := compileModules([]string{
		`
		package test

		p { data.arr[_] = _; test.sleep("1ms") }
		`,
	})

	data := map[string]interface{}{
		"arr": make([]interface{}, 1000),
	}

	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	cancel := NewCancel()

	query := NewQuery(ast.MustParseBody("data.test.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithCancel(cancel)

	go func() {
		time.Sleep(time.Millisecond * 50)
		cancel.Cancel()
	}()

	qrs, err := query.Run(ctx)
	if err == nil || err.(*Error).Code != CancelErr {
		t.Fatalf("Expected cancel error but got: %v (err: %v)", qrs, err)
	}

}

type contextPropagationMock struct{}

// contextPropagationStore will accumulate values from the contexts provided to
// read calls so that the test can verify that contexts are being propagated as
// expected.
type contextPropagationStore struct {
	storage.WritesNotSupported
	storage.TriggersNotSupported
	storage.PolicyNotSupported
	calls []interface{}
}

func (m *contextPropagationStore) NewTransaction(context.Context, ...storage.TransactionParams) (storage.Transaction, error) {
	return nil, nil
}

func (m *contextPropagationStore) Commit(context.Context, storage.Transaction) error {
	return nil
}

func (m *contextPropagationStore) Abort(context.Context, storage.Transaction) {
}

func (m *contextPropagationStore) Read(ctx context.Context, txn storage.Transaction, path storage.Path) (interface{}, error) {
	val := ctx.Value(contextPropagationMock{})
	m.calls = append(m.calls, val)
	return nil, nil
}

func TestTopDownContextPropagation(t *testing.T) {

	ctx := context.WithValue(context.Background(), contextPropagationMock{}, "bar")

	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{
		"mod1": ast.MustParseModule(`package ex

p[x] { data.a[i] = x }`,
		),
	})

	mockStore := &contextPropagationStore{}
	txn := storage.NewTransactionOrDie(ctx, mockStore)
	query := NewQuery(ast.MustParseBody("data.ex.p")).
		WithCompiler(compiler).
		WithStore(mockStore).
		WithTransaction(txn)

	_, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("Unexpected query error: %v", err)
	}

	expectedCalls := []interface{}{"bar"}

	if !reflect.DeepEqual(expectedCalls, mockStore.calls) {
		t.Fatalf("Expected %v but got: %v", expectedCalls, mockStore.calls)
	}
}

func TestTopDownCompositeBaseDereference(t *testing.T) {
	tests := []struct {
		note     string
		rule     string
		expected interface{}
	}{
		// Expect that each of these will evaluate without any errors raised
		{"array", `p { not data.a[[0]] }`, "true"},
		{"object", `p { not data.a[{"b": "c"}] }`, "true"},
		{"set", `p { not data.a[["b"]] }`, "true"},
	}

	data := loadSmallTestData()

	for _, tc := range tests {
		runTopDownTestCase(t, data, tc.note, []string{tc.rule}, tc.expected)
	}
}

func compileModules(input []string) *ast.Compiler {

	mods := map[string]*ast.Module{}

	for idx, i := range input {
		id := fmt.Sprintf("testMod%d", idx)
		mods[id] = ast.MustParseModule(i)
	}

	c := ast.NewCompiler()
	if c.Compile(mods); c.Failed() {
		panic(c.Errors)
	}

	return c
}

func compileRules(imports []string, input []string, modules []string) (*ast.Compiler, error) {

	is := []*ast.Import{}
	for _, i := range imports {
		is = append(is, &ast.Import{
			Path: ast.MustParseTerm(i),
		})
	}

	m := &ast.Module{
		Package: ast.MustParsePackage("package generated"),
		Imports: is,
	}

	rules := []*ast.Rule{}
	for i := range input {
		rules = append(rules, ast.MustParseRule(input[i]))
		rules[i].Module = m
	}

	m.Rules = rules

	for i := range rules {
		rules[i].Module = m
	}

	mods := map[string]*ast.Module{"testMod": m}

	for i, s := range modules {
		mods[fmt.Sprintf("testMod%d", i)] = ast.MustParseModule(s)
	}

	c := ast.NewCompiler()

	if c.Compile(mods); c.Failed() {
		return nil, c.Errors
	}

	return c, nil
}

// loadSmallTestData returns base documents that are referenced
// throughout the topdown test suite.
//
// Avoid the following top-level keys: i, j, k, p, q, r, v, x, y, z.
// These are used for rule names, local variables, etc.
//
func loadSmallTestData() map[string]interface{} {
	var data map[string]interface{}
	err := util.UnmarshalJSON([]byte(`{
        "a": [1,2,3,4],
        "b": {
            "v1": "hello",
            "v2": "goodbye"
        },
        "c": [{
            "x": [true, false, "foo"],
            "y": [null, 3.14159],
            "z": {"p": true, "q": false}
        }],
        "d": {
            "e": ["bar", "baz"]
        },
        "f": [
            {"xs": [1.0], "ys": [2.0]},
            {"xs": [2.0], "ys": [3.0]}
        ],
        "g": {
            "a": [1, 0, 0, 0],
            "b": [0, 2, 0, 0],
            "c": [0, 0, 0, 4]
        },
        "h": [
            [1,2,3],
            [2,3,4]
        ],
        "l": [
            {
                "a": "bob",
                "b": -1,
                "c": [1,2,3,4]
            },
            {
                "a": "alice",
                "b": 1,
                "c": [2,3,4,5],
                "d": null
            }
        ],
		"strings": {
			"foo": 1,
			"bar": 2,
			"baz": 3
		},
		"three": 3,
        "m": [],
		"numbers": [
			"1",
			"2",
			"3",
			"4"
		]
    }`), &data)
	if err != nil {
		panic(err)
	}
	return data
}

func runTopDownTestCase(t *testing.T, data map[string]interface{}, note string, rules []string, expected interface{}) {
	t.Helper()

	runTopDownTestCaseWithContext(context.Background(), t, data, note, rules, nil, "", expected)
}

func runTopDownTestCaseWithModules(t *testing.T, data map[string]interface{}, note string, rules []string, modules []string, input string, expected interface{}) {
	t.Helper()

	runTopDownTestCaseWithContext(context.Background(), t, data, note, rules, modules, input, expected)
}

func runTopDownTestCaseWithContext(ctx context.Context, t *testing.T, data map[string]interface{}, note string, rules []string, modules []string, input string, expected interface{}) {
	t.Helper()

	imports := []string{}
	for k := range data {
		imports = append(imports, "data."+k)
	}

	compiler, err := compileRules(imports, rules, modules)
	if err != nil {
		if _, ok := expected.(error); ok {
			assertError(t, expected, err)
		} else {
			t.Errorf("%v: Compiler error: %v", note, err)
		}
		return
	}

	store := inmem.NewFromObject(data)

	assertTopDownWithPathAndContext(ctx, t, compiler, store, note, []string{"generated", "p"}, input, expected)
}

func assertTopDownWithPath(t *testing.T, compiler *ast.Compiler, store storage.Store, note string, path []string, input string, expected interface{}) {
	t.Helper()

	assertTopDownWithPathAndContext(context.Background(), t, compiler, store, note, path, input, expected)
}

func assertTopDownWithPathAndContext(ctx context.Context, t *testing.T, compiler *ast.Compiler, store storage.Store, note string, path []string, input string, expected interface{}) {
	t.Helper()

	var inputTerm *ast.Term

	if len(input) > 0 {
		inputTerm = ast.MustParseTerm(input)
	}

	txn := storage.NewTransactionOrDie(ctx, store)

	defer store.Abort(ctx, txn)

	var lhs *ast.Term
	if len(path) == 0 {
		lhs = ast.NewTerm(ast.DefaultRootRef)
	} else {
		lhs = ast.MustParseTerm("data." + strings.Join(path, "."))
	}

	rhs := ast.VarTerm(ast.WildcardPrefix + "result")
	body := ast.NewBody(ast.Equality.Expr(lhs, rhs))

	var requiresSort bool

	if rules := compiler.GetRulesExact(lhs.Value.(ast.Ref)); len(rules) > 0 && rules[0].Head.DocKind() == ast.PartialSetDoc {
		requiresSort = true
	}

	if os.Getenv("OPA_DUMP_TEST") != "" {

		data, err := store.Read(ctx, txn, storage.MustParsePath("/"))
		if err != nil {
			t.Fatal(err)
		}

		dump(note, compiler.Modules, data, path, inputTerm, expected, requiresSort)
	}

	// add an inter-query cache
	config, _ := iCache.ParseCachingConfig(nil)
	interQueryCache := iCache.NewInterQueryCache(config)

	query := NewQuery(body).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(inputTerm).
		WithInterQueryBuiltinCache(interQueryCache)

	var tracer BufferTracer

	if os.Getenv("OPA_TRACE_TEST") != "" {
		query = query.WithTracer(&tracer)
	}

	testutil.Subtest(t, note, func(t *testing.T) {
		t.Helper()

		switch e := expected.(type) {
		case *Error, error:
			_, err := query.Run(ctx)
			assertError(t, expected, err)
		case string:
			qrs, err := query.Run(ctx)

			if tracer != nil {
				PrettyTrace(os.Stdout, tracer)
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(e) == 0 {
				if len(qrs) != 0 {
					t.Fatalf("Expected undefined result but got: %v", qrs)
				}
				return
			}

			if len(qrs) == 0 {
				t.Fatalf("Expected %v but got undefined", e)
			}

			result, err := ast.JSON(qrs[0][rhs.Value.(ast.Var)].Value)
			if err != nil {
				t.Fatal(err)
			}

			expected := util.MustUnmarshalJSON([]byte(e))

			if requiresSort {
				sort.Sort(resultSet(result.([]interface{})))
				if sl, ok := expected.([]interface{}); ok {
					sort.Sort(resultSet(sl))
				}
			}

			if util.Compare(expected, result) != 0 {
				t.Fatalf("Unexpected result:\nGot: %+v\nExp:\n%+v", result, expected)
			}

			// If the test case involved the input document, re-run it with partial
			// evaluation enabled and input marked as unknown. Then replay the query and
			// verify the partial evaluation result is the same. Note, we cannot evaluate
			// the result of a query against `data` because the queries need to be
			// converted into rules (which would result in recursion.)
			if len(path) > 0 {
				runTopDownPartialTestCase(ctx, t, compiler, store, txn, inputTerm, rhs, body, requiresSort, expected)
			}
		default:
			t.Fatalf("Unexpected expected value type: %+v", e)
		}
	})
}

func runTopDownPartialTestCase(ctx context.Context, t *testing.T, compiler *ast.Compiler, store storage.Store, txn storage.Transaction, input *ast.Term, output *ast.Term, body ast.Body, requiresSort bool, expected interface{}) {
	t.Helper()

	// add an inter-query cache
	config, _ := iCache.ParseCachingConfig(nil)
	interQueryCache := iCache.NewInterQueryCache(config)

	partialQuery := NewQuery(body).
		WithCompiler(compiler).
		WithStore(store).
		WithUnknowns([]*ast.Term{ast.MustParseTerm("input")}).
		WithTransaction(txn).
		WithInterQueryBuiltinCache(interQueryCache)

	partials, support, err := partialQuery.PartialRun(ctx)

	if err != nil {
		t.Fatal("Unexpected error on partial evaluation comparison:", err)
	}

	module := ast.MustParseModule("package topdown_test_partial")
	module.Rules = make([]*ast.Rule, len(partials))
	for i, body := range partials {
		module.Rules[i] = &ast.Rule{
			Head:   ast.NewHead(ast.Var("__result__"), nil, output),
			Body:   body,
			Module: module,
		}
	}

	compiler.Modules["topdown_test_partial"] = module
	for i, module := range support {
		compiler.Modules[fmt.Sprintf("topdown_test_support_%d", i)] = module
	}

	compiler.Compile(compiler.Modules)
	if compiler.Failed() {
		t.Fatal("Unexpected error on partial evaluation result compile:", compiler.Errors)
	}

	query := NewQuery(ast.MustParseBody("data.topdown_test_partial.__result__ = x")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(input).
		WithInterQueryBuiltinCache(interQueryCache)

	qrs, err := query.Run(ctx)
	if err != nil {
		t.Fatal("Unexpected error on query after partial evaluation:", err)
	}

	if len(qrs) == 0 {
		t.Fatalf("Expected %v but got undefined from query after partial evaluation", expected)
	}

	result, err := ast.JSON(qrs[0][ast.Var("x")].Value)
	if err != nil {
		t.Fatal(err)
	}

	if requiresSort {
		sort.Sort(resultSet(result.([]interface{})))
		if sl, ok := expected.([]interface{}); ok {
			sort.Sort(resultSet(sl))
		}
	}

	if util.Compare(expected, result) != 0 {
		t.Fatalf("Unexpected result after partial evaluation:\nGot:\n%v\nExp:\n%v", result, expected)
	}
}

type resultSet []interface{}

func (rs resultSet) Less(i, j int) bool {
	return util.Compare(rs[i], rs[j]) < 0
}

func (rs resultSet) Swap(i, j int) {
	tmp := rs[i]
	rs[i] = rs[j]
	rs[j] = tmp
}

func (rs resultSet) Len() int {
	return len(rs)
}

func init() {

	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.sleep",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.NewNull(),
		),
	})

	RegisterFunctionalBuiltin1("test.sleep", func(a ast.Value) (ast.Value, error) {
		d, _ := time.ParseDuration(string(a.(ast.String)))
		time.Sleep(d)
		return ast.Null{}, nil
	})

}

var testID = 0
var testIDMutex sync.Mutex

func getTestNamespace() string {
	programCounters := make([]uintptr, 20)
	n := runtime.Callers(0, programCounters)
	if n > 0 {
		frames := runtime.CallersFrames(programCounters[:n])
		for more := true; more; {
			var f runtime.Frame
			f, more = frames.Next()
			if strings.HasPrefix(f.Function, "github.com/open-policy-agent/opa/topdown.Test") {
				return strings.TrimPrefix(strings.ToLower(strings.TrimPrefix(strings.TrimPrefix(f.Function, "github.com/open-policy-agent/opa/topdown.Test"), "TopDown")), "builtin")
			}
		}
	}
	return ""
}

func dump(note string, modules map[string]*ast.Module, data interface{}, docpath []string, input *ast.Term, exp interface{}, requiresSort bool) {

	moduleSet := []string{}
	for _, module := range modules {
		moduleSet = append(moduleSet, string(bytes.ReplaceAll(format.MustAst(module), []byte("\t"), []byte("  "))))
	}

	namespace := getTestNamespace()

	test := map[string]interface{}{
		"note":    namespace + "/" + note,
		"data":    data,
		"modules": moduleSet,
		"query":   strings.Join(append([]string{"data"}, docpath...), ".") + " = x",
	}

	if input != nil {
		test["input_term"] = input.String()
	}

	switch e := exp.(type) {
	case string:
		rs := []map[string]interface{}{}
		if len(e) > 0 {
			exp := util.MustUnmarshalJSON([]byte(e))
			if requiresSort {
				sl := exp.([]interface{})
				sort.Sort(resultSet(sl))
			}
			rs = append(rs, map[string]interface{}{"x": exp})
		}
		test["want_result"] = rs
		if requiresSort {
			test["sort_bindings"] = true
		}
	case error:
		test["want_error_code"] = e.(*Error).Code
		test["want_error"] = e.(*Error).Message
	default:
		panic("Unexpected test expectation. Cowardly refusing to generate test cases.")
	}

	bs, err := yaml.Marshal(map[string]interface{}{"cases": []interface{}{test}})
	if err != nil {
		panic(err)
	}

	dir := path.Join(os.Getenv("OPA_DUMP_TEST"), namespace)

	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(err)
	}

	testIDMutex.Lock()
	testID++
	c := testID
	testIDMutex.Unlock()

	filename := fmt.Sprintf("test-%v-%04d.yaml", namespace, c)

	if err := ioutil.WriteFile(filepath.Join(dir, filename), bs, 0644); err != nil {
		panic(err)
	}

}

func assertError(t *testing.T, expected interface{}, actual error) {
	t.Helper()
	if actual == nil {
		t.Errorf("Expected error but got: %v", actual)
		return
	}

	errString := actual.Error()

	if reflect.TypeOf(expected) != reflect.TypeOf(actual) {
		t.Errorf("Expected error of type '%T', got '%T'", expected, actual)
	}

	switch e := expected.(type) {
	case Error:
		assertErrorContains(t, errString, e.Code)
		assertErrorContains(t, errString, e.Message)
	case *Error:
		assertErrorContains(t, errString, e.Code)
		assertErrorContains(t, errString, e.Message)
	case *ast.Error:
		assertErrorContains(t, errString, e.Code)
		assertErrorContains(t, errString, e.Message)
	case ast.Errors:
		for _, astErr := range e {
			assertErrorContains(t, errString, astErr.Code)
			assertErrorContains(t, errString, astErr.Message)
		}
	case error:
		assertErrorContains(t, errString, e.Error())
	}
}

func assertErrorContains(t *testing.T, actualErrMsg string, expected string) {
	t.Helper()
	if !strings.Contains(actualErrMsg, expected) {
		t.Errorf("Expected error '%v' but got: '%v'", expected, actualErrMsg)
	}
}
