// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

func TestEvalRef(t *testing.T) {

	var tests = []struct {
		ref      string
		expected interface{}
	}{
		{"data.c[i][j]", `[
            {"i": 0, "j": "x"},
            {"i": 0, "j": "y"},
            {"i": 0, "j": "z"}
         ]`},
		{"data.c[i][j][k]", `[
            {"i": 0, "j": "x", "k": 0},
            {"i": 0, "j": "x", "k": 1},
            {"i": 0, "j": "x", "k": 2},
            {"i": 0, "j": "y", "k": 0},
            {"i": 0, "j": "y", "k": 1},
            {"i": 0, "j": "z", "k": "p"},
            {"i": 0, "j": "z", "k": "q"}
        ]`},
		{"data.d[x][y]", `[
            {"x": "e", "y": 0},
            {"x": "e", "y": 1}
        ]`},
		{`data.c[i]["x"][k]`, `[
            {"i": 0, "k": 0},
            {"i": 0, "k": 1},
            {"i": 0, "k": 2}
        ]`},
		{"data.c[i][j][i]", `[
            {"i": 0, "j": "x"},
            {"i": 0, "j": "y"}
        ]`},
		{`data.c[i]["deadbeef"][k]`, nil},
		{`data.c[999]`, nil},
	}

	data := loadSmallTestData()

	ctx := &Context{
		DataStore: storage.NewDataStoreFromJSONObject(data),
		Globals:   storage.NewBindings(),
		Locals:    storage.NewBindings(),
	}

	for i, tc := range tests {

		switch e := tc.expected.(type) {
		case nil:
			var tmp *Context
			err := evalRef(ctx, ast.MustParseRef(tc.ref), func(ctx *Context) error {
				tmp = ctx
				return nil
			})
			if err != nil {
				t.Errorf("Test case (%d): unexpected error: %v", i+1, err)
				continue
			}
			if tmp != nil {
				t.Errorf("Test case (%d): expected no bindings (nil) but got: %v", i+1, tmp)
			}
		case string:
			expected := loadExpectedBindings(e)
			err := evalRef(ctx, ast.MustParseRef(tc.ref), func(ctx *Context) error {
				if len(expected) > 0 {
					for j, exp := range expected {
						if exp.Equal(ctx.Locals) {
							tmp := expected[:j]
							expected = append(tmp, expected[j+1:]...)
							return nil
						}
					}
				}
				// If there was not a matching expected binding, treat this case as a failure.
				return fmt.Errorf("unexpected bindings: %v", ctx.Locals)
			})
			if err != nil {
				t.Errorf("Test case %d: expected success but got error: %v", i+1, err)
				continue
			}
			if len(expected) > 0 {
				t.Errorf("Test case %d: missing expected bindings: %v", i+1, expected)
			}
		}

	}
}

func TestEvalTerms(t *testing.T) {

	tests := []struct {
		body     string
		expected string
	}{
		{"data.c[i][j][k] = x", `[
            {"i": 0, "j": "x", "k": 0},
            {"i": 0, "j": "x", "k": 1},
            {"i": 0, "j": "x", "k": 2},
            {"i": 0, "j": "y", "k": 0},
            {"i": 0, "j": "y", "k": 1},
            {"i": 0, "j": "z", "k": "p"},
            {"i": 0, "j": "z", "k": "q"}
        ]`},
		{"data.a[i] = data.h[j][k]", `[
            {"i": 0, "j": 0, "k": 0},
            {"i": 1, "j": 0, "k": 1},
            {"i": 1, "j": 1, "k": 0},
            {"i": 2, "j": 0, "k": 2},
            {"i": 2, "j": 1, "k": 1},
            {"i": 3, "j": 1, "k": 2}
        ]`},
		{`data.d[x][y] = "baz"`, `[
            {"x": "e", "y": 1}
        ]`},
		{"data.d[x][y] = data.d[x][y]", `[
            {"x": "e", "y": 0},
            {"x": "e", "y": 1}
        ]`},
		{"data.d[x][y] = data.z[i]", `[]`},
	}

	data := loadSmallTestData()

	for i, tc := range tests {

		ctx := &Context{
			Query:     ast.MustParseBody(tc.body),
			DataStore: storage.NewDataStoreFromJSONObject(data),
			Globals:   storage.NewBindings(),
			Locals:    storage.NewBindings(),
		}

		expected := loadExpectedBindings(tc.expected)

		err := evalTerms(ctx, func(ctx *Context) error {
			if len(expected) > 0 {
				for j, exp := range expected {
					if exp.Equal(ctx.Locals) {
						tmp := expected[:j]
						expected = append(tmp, expected[j+1:]...)
						return nil
					}
				}
			}
			// If there was not a matching expected binding, treat this case as a failure.
			return fmt.Errorf("unexpected bindings: %v", ctx.Locals)
		})
		if err != nil {
			t.Errorf("Test case %d: expected success but got error: %v", i+1, err)
			continue
		}
		if len(expected) > 0 {
			t.Errorf("Test case %d: missing expected bindings: %v", i+1, expected)
		}
	}
}

func TestPlugValue(t *testing.T) {

	a := ast.Var("a")
	b := ast.Var("b")
	c := ast.Var("c")
	k := ast.Var("k")
	v := ast.Var("v")
	cs := ast.MustParseTerm("[c]").Value
	ks := ast.MustParseTerm(`{k: "world"}`).Value
	vs := ast.MustParseTerm(`{"hello": v}`).Value
	hello := ast.String("hello")
	world := ast.String("world")

	ctx1 := &Context{Locals: storage.NewBindings()}
	ctx1 = ctx1.BindVar(a, b)
	ctx1 = ctx1.BindVar(b, cs)
	ctx1 = ctx1.BindVar(c, ks)
	ctx1 = ctx1.BindVar(k, hello)

	ctx2 := &Context{Locals: storage.NewBindings()}
	ctx2 = ctx2.BindVar(a, b)
	ctx2 = ctx2.BindVar(b, cs)
	ctx2 = ctx2.BindVar(c, vs)
	ctx2 = ctx2.BindVar(v, world)

	expected := ast.MustParseTerm(`[{"hello": "world"}]`).Value

	r1 := plugValue(a, ctx1)

	if !expected.Equal(r1) {
		t.Errorf("Expected %v but got %v", expected, r1)
		return
	}

	r2 := plugValue(a, ctx2)

	if !expected.Equal(r2) {
		t.Errorf("Expected %v but got %v", expected, r2)
	}
}

func TestTopDownCompleteDoc(t *testing.T) {
	tests := []struct {
		note     string
		rule     string
		expected string
	}{
		{"undefined", "p = null :- false", ""}, // "" will be converted to Undefined
		{"null", "p = null :- true", "null"},
		{"bool: true", "p = true :- true", "true"},
		{"bool: false", "p = false :- true", "false"},
		{"number: 3", "p = 3 :- true", "3"},
		{"number: 3.0", "p = 3.0 :- true", "3.0"},
		{"number: 66.66667", "p = 66.66667 :- true", "66.66667"},
		{`string: "hello"`, `p = "hello" :- true`, `"hello"`},
		{`string: ""`, `p = "" :- true`, `""`},
		{"array: [1,2,3,4]", "p = [1,2,3,4] :- true", "[1,2,3,4]"},
		{"array: []", "p = [] :- true", "[]"},
		{`object/nested composites: {"a": [1], "b": [2], "c": [3]}`,
			`p = {"a": [1], "b": [2], "c": [3]} :- true`,
			`{"a": [1], "b": [2], "c": [3]}`},
		{"var/var", "p = true :- x = y", ""},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownPartialSetDoc(t *testing.T) {

	tests := []struct {
		note     string
		rule     string
		expected string
	}{
		{"array values", "p[x] :- a[i] = x", `[1, 2, 3, 4]`},
		{"array indices", "p[x] :- a[x] = _", `[0, 1, 2, 3]`},
		{"object keys", "p[x] :- b[x] = _", `["v1", "v2"]`},
		{"object values", "p[x] :- b[i] = x", `["hello", "goodbye"]`},
		{"nested composites", "p[x] :- f[i] = x", `[{"xs": [1.0], "ys": [2.0]}, {"xs": [2.0], "ys": [3.0]}]`},
		{"deep ref/heterogeneous", "p[x] :- c[i][j][k] = x", `[null, 3.14159, true, false, true, false, "foo"]`},
		{"composite var value", "p[x] :- x = [i, a[i]]", "[[0,1],[1,2],[2,3],[3,4]]"},
		{"var/var", "p[x] :- x = y", "[]"},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownPartialObjectDoc(t *testing.T) {
	tests := []struct {
		note     string
		rule     string
		expected interface{}
	}{
		{"identity", "p[k] = v :- b[k] = v", `{"v1": "hello", "v2": "goodbye"}`},
		{"composites", "p[k] = v :- d[k] = v", `{"e": ["bar", "baz"]}`},
		{"non-string key", "p[k] = v :- a[k] = v", fmt.Errorf("illegal object key type float64: 0")},
		{"body/join var", "p[k] = v :- a[i] = v, g[k][i] = v", `{"a": 1, "b": 2, "c": 4}`},
		{"var/var key", "p[k] = v :- v = 1, k = x", "{}"},
		{"var/var val", `p[k] = v :- k = "x", v = x`, "{}"},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownEqExpr(t *testing.T) {

	tests := []struct {
		note     string
		rule     string
		expected interface{}
	}{
		// undefined cases
		{"undefined: same type", `p = true :- true = false`, ""},
		{"undefined: diff type", `p = true :- 42 = "hello"`, ""},
		{"undefined: array order", `p = true :- [1,2,3] = [1,3,2]`, ""},
		{"undefined: ref value", "p = true :- a[3] = 9999", ""},
		{"undefined: ref values", "p = true :- a[i] = 9999", ""},
		{"undefined: ground var", "p = true :- a[3] = x, x = 3", ""},
		{"undefined: array var 1", "p = true :- [1,x,x] = [1,2,3]", ""},
		{"undefined: array var 2", "p = true :- [1,x,3] = [1,2,x]", ""},
		{"undefined: object var 1", `p = true :- {"a": 1, "b": 2} = {"a": a, "b": a}`, ""},
		{"undefined: array deep var 1", "p = true :- [[1,x],[3,x]] = [[1,2],[3,4]]", ""},
		{"undefined: array deep var 2", "p = true :- [[1,x],[3,4]] = [[1,2],[x,4]]", ""},
		{"undefined: array uneven", `p = true :- [true, false, "foo", "deadbeef"] = c[i][j]`, ""},
		{"undefined: object uneven", `p = true :- {"a": 1, "b": 2} = {"a": 1}`, ""},
		{"undefined: occurs 1", "p = true :- [y,x] = [[x],y]", ""},
		{"undefined: occurs 2", "p = true :- [y,x] = [{\"a\": x}, y]", ""},

		// ground terms
		{"ground: bool", `p = true :- true = true`, "true"},
		{"ground: string", `p = true :- "string" = "string"`, "true"},
		{"ground: number", `p = true :- 17 = 17`, "true"},
		{"ground: null", `p = true :- null = null`, "true"},
		{"ground: array", `p = true :- [1,2,3] = [1,2,3]`, "true"},
		{"ground: object", `p = true :- {"b": false, "a": [1,2,3]} = {"a": [1,2,3], "b": false}`, "true"},
		{"ground: ref 1", `p = true :- a[2] = 3`, "true"},
		{"ground: ref 2", `p = true :- b["v2"] = "goodbye"`, "true"},
		{"ground: ref 3", `p = true :- d["e"] = ["bar", "baz"]`, "true"},
		{"ground: ref 4", `p = true :- c[0].x[1] = c[0].z["q"]`, "true"},

		// variables
		{"var: x=y=z", "p[x] :- x = y, z = 42, y = z", "[42]"},
		{"var: ref value", "p = true :- a[3] = x, x = 4", "true"},
		{"var: ref values", "p = true :- a[i] = x, x = 2", "true"},
		{"var: ref key", "p = true :- a[i] = 4, x = 3", "true"},
		{"var: ref keys", "p = true :- a[i] = x, i = 2", "true"},
		{"var: ref ground var", "p[x] :- i = 2, a[i] = x", "[3]"},
		{"var: ref ref", "p[x] :- c[0].x[i] = c[0].z[j], x = [i, j]", `[[0, "p"], [1, "q"]]`},

		// arrays and variables
		{"pattern: array", "p[x] :- [1,x,3] = [1,2,3]", "[2]"},
		{"pattern: array 2", "p[x] :- [[1,x],[3,4]] = [[1,2],[3,4]]", "[2]"},
		{"pattern: array same var", "p[x] :- [2,x,3] = [x,2,3]", "[2]"},
		{"pattern: array multiple vars", "p[z] :- [1,x,y] = [1,2,3], z = [x, y]", "[[2, 3]]"},
		{"pattern: array multiple vars 2", "p[z] :- [1,x,3] = [y,2,3], z = [x, y]", "[[2, 1]]"},
		{"pattern: array ref", "p[x] :- [1,2,3,x] = [a[0], a[1], a[2], a[3]]", "[4]"},
		{"pattern: array non-ground ref", "p[x] :- [1,2,3,x] = [a[0], a[1], a[2], a[i]]", "[1,2,3,4]"},
		{"pattern: array = ref", "p[x] :- [true, false, x] = c[i][j]", `["foo"]`},
		{"pattern: array = ref (reversed)", "p[x] :-  c[i][j] = [true, false, x]", `["foo"]`},
		{"pattern: array = var", "p[y] :- [1,2,x] = y, x = 3", "[[1,2,3]]"},

		// objects and variables
		{"pattern: object val", `p[y] :- {"x": y} = {"x": "y"}`, `["y"]`},
		{"pattern: object same var", `p[x] :- {"x": x, "y": x} = {"x": 1, "y": 1}`, "[1]"},
		{"pattern: object multiple vars", `p[z] :- {"x": x, "y": y} = {"x": 1, "y": 2}, z = [x, y]`, "[[1, 2]]"},
		{"pattern: object multiple vars 2", `p[z] :- {"x": x, "y": 2} = {"x": 1, "y": y}, z = [x, y]`, "[[1, 2]]"},
		{"pattern: object ref", `p[x] :- {"p": c[0].x[0], "q": x} = c[i][j]`, `[false]`},
		{"pattern: object non-ground ref", `p[x] :- {"a": 1, "b": x} = {"a": 1, "b": c[0].x[i]}`, `[true, false, "foo"]`},
		{"pattern: object = ref", `p[x] :- {"p": y, "q": z} = c[i][j], x = [i, j, y, z]`, `[[0, "z", true, false]]`},
		{"pattern: object = ref (reversed)", `p[x] :- c[i][j] = {"p": y, "q": z}, x = [i, j, y, z]`, `[[0, "z", true, false]]`},
		{"pattern: object = var", `p[x] :- {"a": 1, "b": y} = x, y = 2`, `[{"a": 1, "b": 2}]`},
		{"pattern: object/array nested", `p[ys] :- f[i] = {"xs": [2.0], "ys": ys}`, `[[3.0]]`},
		{"pattern: object/array nested 2", `p[v] :- f[i] = {"xs": [x], "ys": [y]}, v = [x, y]`, `[[1.0, 2.0], [2.0, 3.0]]`},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownIneqExpr(t *testing.T) {

	tests := []struct {
		note     string
		rule     string
		expected interface{}
	}{
		{"noteq", "p = true :- 0 != 1, a[i] = x, x != 2", "true"},
		{"gt", "p = true :- 1 > 0, a[i] = x, x > 2", "true"},
		{"gteq", "p = true :- 1 >= 1, a[i] = x, x >= 4", "true"},
		{"lt", "p = true :- -1 < 0, a[i] = x, x < 5", "true"},
		{"lteq", "p = true :- -1 <= 0, a[i] = x, x <= 1", "true"},
		{"undefined: noteq", "p = true :- 0 != 0", ""},
		{"undefined: gt", "p = true :- 1 > 2", ""},
		{"undefined: gteq", "p = true :- 1 >= 2", ""},
		{"undefined: lt", "p = true :- 1 < -1", ""},
		{"undefined: lteq", "p = true :- 1 < -1", ""},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownVirtualDocs(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		// input to partial set and object docs
		{"input: set 1", []string{"p = true :- q[1]", "q[x] :- a[i] = x"}, "true"},
		{"input: set 2", []string{"p[x] :- q[1] = x", "q[x] :- a[i] = x"}, "[true]"},
		{"input: set embedded", []string{`p[x] :- x = {"b": [q[2]]}`, `q[x] :- a[i] = x`}, `[{"b": [true]}]`},
		{"input: set undefined", []string{"p = true :- q[1000]", "q[x] :- a[x] = y"}, ""},
		{"input: set ground var", []string{"p[x] :- x = 1, q[x]", "q[y] :- a = [1,2,3,4], a[y] = i"}, "[1]"},
		{"input: object 1", []string{"p = true :- q[1] = 2", "q[i] = x :- a[i] = x"}, "true"},
		{"input: object 2", []string{"p = true :- q[1] = 0", "q[x] = i :- a[i] = x"}, "true"},
		{"input: object embedded 1", []string{"p[x] :- x = [1, q[3], q[2]]", "q[i] = x :- a[i] = x"}, "[[1,4,3]]"},
		{"input: object embedded 2", []string{`p[x] :- x = {"a": [q[3]], "b": [q[2]]}`, `q[i] = x :- a[i] = x`}, `[{"a": [4], "b": [3]}]`},
		{"input: object undefined val", []string{`p = true :- q[1] = 9999`, `q[i] = x :- a[i] = x`}, ""},
		{"input: object undefined key 1", []string{`p = true :- q[9999] = 2`, `q[i] = x :- a[i] = x`}, ""},
		{"input: object undefined key 2", []string{`p = true :- q["foo"] = 2`, `q[i] = x :- a[i] = x`}, ""},
		{"input: object dereference ground", []string{`p = true :- q[0]["x"][1] = false`, `q[i] = x :- x = c[i]`}, "true"},
		{"input: object defererence non-ground", []string{`p = true :- q[0][x][y] = false`, `q[i] = x :- x = c[i]`}, "true"},
		{"input: object ground var key", []string{`p[y] :- x = "b", q[x] = y`, `q[k] = v :- x = {"a": 1, "b": 2}, x[k] = v`}, "[2]"},
		{"input: variable binding substitution", []string{
			"p[x] = y :- r[z] = y, q[x] = z",
			`r[k] = v :- x = {"a": 1, "b": 2, "c": 3, "d": 4}, x[k] = v`,
			`q[y] = x :- z = {"a": "a", "b": "b", "d": "d"}, z[y] = x`},
			`{"a": 1, "b": 2, "d": 4}`},

		// output from partial set and object docs
		{"output: set", []string{"p[x] :- q[x]", "q[y] :- a[i] = y"}, "[1,2,3,4]"},
		{"output: set embedded", []string{`p[i] :- {i: [true]} = {i: [q[i]]}`, `q[x] :- d.e[i] = x`}, `["bar", "baz"]`},
		{"output: object key", []string{"p[x] :- q[x] = 4", "q[i] = x :- a[i] = x"}, "[3]"},
		{"output: object value", []string{"p[x] = y :- q[x] = y", "q[k] = v :- b[k] = v"}, `{"v1": "hello", "v2": "goodbye"}`},
		{"output: object embedded", []string{"p[k] = v :- {k: [q[k]]} = {k: [v]}", `q[x] = y :- b[x] = y`}, `{"v1": "hello", "v2": "goodbye"}`},
		{"output: object dereference ground", []string{`p[i] :- q[i]["x"][1] = false`, `q[i] = x :- x = c[i]`}, "[0]"},
		{"output: object defererence non-ground", []string{`p[r] :- q[x][y][z] = false, r = [x, y, z]`, `q[i] = x :- x = c[i]`}, `[[0, "x", 1], [0, "z", "q"]]`},

		// input+output from partial set/object docs
		{"i/o: objects", []string{
			"p[x] :- q[x] = r[x]",
			`q[x] = y :- z = {"a": 1, "b": 2, "d": 4}, z[x] = y`,
			`r[k] = v :- x = {"a": 1, "b": 2, "c": 4, "d": 3}, x[k] = v`},
			`["a", "b"]`},

		{"i/o: undefined keys", []string{
			"p[y] :- q[x], r[x] = y",
			`q[x] :- z = ["a", "b", "c", "d"], z[y] = x`,
			`r[k] = v :- x = {"a": 1, "b": 2, "d": 4}, x[k] = v`},
			`[1, 2, 4]`},

		// input/output to/from complete docs
		{"input: complete array", []string{"p = true :- q[1] = 2", "q = [1,2,3,4] :- true"}, "true"},
		{"input: complete object", []string{`p = true :- q["b"] = 2`, `q = {"a": 1, "b": 2} :- true`}, "true"},
		{"input: complete array dereference ground", []string{"p = true :- q[1][1] = 3", "q = [[0,1], [2,3]] :- true"}, "true"},
		{"input: complete object dereference ground", []string{`p = true :- q["b"][1] = 4`, `q = {"a": [1, 2], "b": [3, 4]} :- true`}, "true"},
		{"input: complete array ground index", []string{"p[x] :- z = [1, 2], z[i] = y, q[y] = x", "q = [1,2,3,4] :- true"}, "[2,3]"},
		{"input: complete object ground key", []string{`p[x] :- z = ["b", "c"], z[i] = y, q[y] = x`, `q = {"a":1,"b":2,"c":3,"d":4} :- true`}, "[2,3]"},
		{"output: complete array", []string{"p[x] :- q[i] = e, x = [i,e]", "q = [1,2,3,4] :- true"}, "[[0,1],[1,2],[2,3],[3,4]]"},
		{"output: complete object", []string{"p[x] :- q[i] = e, x = [i,e]", `q = {"a": 1, "b": 2} :- true`}, `[["a", 1], ["b", 2]]`},
		{"output: complete array dereference non-ground", []string{"p[r] :- q[i][j] = 2, r = [i, j]", "q = [[1,2], [3,2]] :- true"}, "[[0, 1], [1, 1]]"},
		{"output: complete object defererence non-ground", []string{`p[r] :- q[x][y] = 2, r = [x, y]`, `q = {"a": {"x": 1}, "b": {"y": 2}, "c": {"z": 2}} :- true`}, `[["b", "y"], ["c", "z"]]`},

		// undefined
		{"undefined: dereference set", []string{"p = true :- q[x].foo = 100", "q[x] :- x = a[i]"}, ""},

		// TODO(tsandall): cover non-dereferenced cases, e.g., "p[x] :- q = [1,2,3]" and "q = [1,2,3] :- true"
		// once the parser supports modules this will be easier to generate test cases for...
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownVarReferences(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		// TODO(tsandall) dereferenced variables must be bound beforehand (safety check)
		{"ground", []string{"p[x] :- v = [[1,2],[2,3],[3,4]], x = v[2][1]"}, "[4]"},
		{"non-ground", []string{"p[x] :- v = [[1,2],[2,3],[3,4]], x = v[i][j]"}, "[1,2,2,3,3,4]"},
		{"mixed", []string{`p[x] = y :- v = [{"a": 1, "b": 2}, {"c": 3, "z": [4]}], y = v[i][x][j]`}, `{"z": 4}`},
		{"ref binding", []string{"p[x] :- v = c[i][j], x = v[k], x = true"}, "[true, true]"},
		{"embedded", []string{`p[x] :- v = [1,2,3], x = [{"a": v[i]}]`}, `[[{"a": 1}], [{"a": 2}], [{"a": 3}]]`},
		{"embedded ref binding", []string{"p[x] :- v = c[i][j], w = [v[0], v[1]], x = w[y]"}, "[null, false, true, 3.14159]"},
		{"array: ground var", []string{"p[x] :- i = [1,2,3,4], j = [1,2,999], j[k] = y, i[y] = x"}, "[2,3]"},
		{"object: ground var", []string{`p[x] :- i = {"a": 1, "b": 2, "c": 3}, j = ["a", "c", "deadbeef"], j[k] = y, i[y] = x`}, "[1, 3]"},
		{"avoids indexer", []string{"p = true :- somevar = [1,2,3], somevar[i] = 2"}, "true"},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownDisjunction(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"incr: query set", []string{"p[x] :- a[i] = x", "p[y] :- b[j] = y"}, `[1,2,3,4,"hello","goodbye"]`},
		{"incr: query object", []string{"p[k] = v :- b[v] = k", "p[k] = v :- a[i] = v, g[k][j] = v"}, `{"b": 2, "c": 4, "hello": "v1", "goodbye": "v2", "a": 1}`},
		{"incr: eval set", []string{"p[x] :- q[x]", "q[x] :- a[i] = x", "q[y] :- b[j] = y"}, `[1,2,3,4,"hello","goodbye"]`},
		{"incr: eval object", []string{"p[k] = v :- q[k] = v", "q[k] = v :- b[v] = k", "q[k] = v :- a[i] = v, g[k][j] = v"}, `{"b": 2, "c": 4, "hello": "v1", "goodbye": "v2", "a": 1}`},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownNegation(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"neg: constants", []string{"p = true :- not true = false"}, "true"},
		{"neg: constants", []string{"p = true :- not true = true"}, ""},

		// TODO(tsandall): re-enable these once we have support for mangling "_"

		// {"neg: array contains", []string{"p = true :- not a[_] = 9999"}, "true"},
		// {"neg: array diff", []string{"p = true :- not a[_] = 4"}, ""},
		// {"neg: object values contains", []string{`p = true :- not b[_] = "deadbeef"`}, "true"},
		// {"neg: object values diff", []string{`p = true :- not b[_] = "goodbye"`}, ""},
		{"neg: set contains", []string{`p = true :- not q["v0"]`, `q[x] :- b[x] = v`}, "true"},
		// {"neg: set diff", []string{`p = true :- not q["v2"]`, `q[x] :- b[x] = v`}, ""},

		// TODO(tsandall): 'not g[j][k] ...' is not valid.
		// {"neg: multiple exprs", []string{"p[x] :- a[x] = i, not g[j][k] = x, f[v][y][z] = x"}, "[3]"},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownEmbeddedVirtualDoc(t *testing.T) {

	mods := compileModules([]string{
		`package b.c.d

         import data.a
         import data.g

         p[x] :- a[i] = x, q[x]
         q[x] :- g[j][k] = x`})

	data := loadSmallTestData()

	store := storage.NewDataStoreFromJSONObject(data)
	policyStore := storage.NewPolicyStore(store, "")

	for id, mod := range mods {
		err := policyStore.Add(id, mod, []byte(""), false)
		if err != nil {
			panic(err)
		}
	}

	assertTopDown(t, store, 0, "deep embedded vdoc", []string{"b", "c", "d", "p"}, "{}", "[1, 2, 4]")
}

func TestTopDownGlobalVars(t *testing.T) {
	mods := compileModules([]string{
		`package z
		 import data.a
		 import req1
		 import req2 as req2as
		 p = true :- a[i] = x, req1.foo = x, req2as.bar = x, q[x]
		 q[x] :- req1.foo = x, req2as.bar = x, r[x]
		 r[x] :- {"foo": req2as.bar, "bar": [x]} = {"foo": x, "bar": [req1.foo]}`})

	data := loadSmallTestData()
	store := storage.NewDataStoreFromJSONObject(data)
	policyStore := storage.NewPolicyStore(store, "")

	for id, mod := range mods {
		err := policyStore.Add(id, mod, []byte(""), false)
		if err != nil {
			panic(err)
		}
	}

	assertTopDown(t, store, 0, "global vars", []string{"z", "p"}, `{
		req1: {"foo": 4},
		req2: {"bar": 4}
	}`, "true")
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

	vd := `
        package opa.example

        import data.servers
        import data.networks
        import data.ports

        public_servers[server] :-
            server = servers[i],
            server.ports[j] = ports[k].id,
            ports[k].networks[l] = networks[m].id,
            networks[m].public = true

        violations[server] :-
            server = servers[i],
            server.protocols[j] = "http",
            public_servers[server]
    `

	var doc map[string]interface{}

	if err := json.Unmarshal([]byte(bd), &doc); err != nil {
		panic(err)
	}

	mods := compileModules([]string{vd})

	store := storage.NewDataStoreFromJSONObject(doc)
	policyStore := storage.NewPolicyStore(store, "")

	for id, mod := range mods {
		err := policyStore.Add(id, mod, []byte(""), false)
		if err != nil {
			panic(err)
		}
	}

	assertTopDown(t, store, 0, "public servers", []string{"opa", "example", "public_servers"}, "{}", `
        [
            {"id": "s1", "name": "app", "protocols": ["https", "ssh"], "ports": ["p1", "p2", "p3"]},
            {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
        ]
    `)

	assertTopDown(t, store, 0, "violations", []string{"opa", "example", "violations"}, "{}", `
        [
            {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
        ]
    `)
}

func compileModules(input []string) map[string]*ast.Module {

	mods := map[string]*ast.Module{}

	for idx, i := range input {
		id := fmt.Sprintf("testMod%d", idx)
		mods[id] = ast.MustParseModule(i)
	}

	c := ast.NewCompiler()
	if c.Compile(mods); c.Failed() {
		panic(c.FlattenErrors())
	}

	return c.Modules
}

func compileRules(imports []string, input []string) map[string]*ast.Module {

	rules := []*ast.Rule{}
	for _, i := range input {
		rules = append(rules, ast.MustParseRule(i))
	}

	is := []*ast.Import{}
	for _, i := range imports {
		is = append(is, &ast.Import{
			Path: ast.MustParseTerm(i),
		})
	}

	p := ast.Ref{ast.DefaultRootDocument}
	m := &ast.Module{
		Package: &ast.Package{
			Path: p,
		},
		Imports: is,
		Rules:   rules,
	}

	c := ast.NewCompiler()
	if c.Compile(map[string]*ast.Module{"testMod": m}); c.Failed() {
		panic(c.FlattenErrors())
	}

	return c.Modules
}

func loadExpectedBindings(input string) []*storage.Bindings {
	var data []map[string]interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		panic(err)
	}
	var expected []*storage.Bindings
	for _, bindings := range data {
		buf := storage.NewBindings()
		for k, v := range bindings {
			switch v := v.(type) {
			case string:
				buf.Put(ast.Var(k), ast.String(v))
			case float64:
				buf.Put(ast.Var(k), ast.Number(v))
			default:
				panic("unreachable")
			}
		}
		expected = append(expected, buf)
	}

	return expected
}

func loadExpectedResult(input string) interface{} {
	if len(input) == 0 {
		return Undefined{}
	}
	var data interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		panic(err)
	}
	return data
}

func loadExpectedSortedResult(input string) interface{} {
	data := loadExpectedResult(input)
	switch data := data.(type) {
	case []interface{}:
		sort.Sort(resultSet(data))
		return data
	default:
		return data
	}
}

// loadSmallTestData returns base documents that are referenced
// throughout the topdown test suite.
//
// Avoid the following top-level keys: i, j, k, p, q, r, v, x, y, z.
// These are used for rule names, local variables, etc.
//
func loadSmallTestData() map[string]interface{} {
	var data map[string]interface{}
	err := json.Unmarshal([]byte(`{
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
        "m": []
    }`), &data)
	if err != nil {
		panic(err)
	}
	return data
}

func runTopDownTestCase(t *testing.T, data map[string]interface{}, i int, note string, rules []string, expected interface{}) {
	imports := []string{}
	for k := range data {
		imports = append(imports, "data."+k)
	}

	mods := compileRules(imports, rules)

	store := storage.NewDataStoreFromJSONObject(data)
	policyStore := storage.NewPolicyStore(store, "")

	for id, mod := range mods {
		err := policyStore.Add(id, mod, []byte(""), false)
		if err != nil {
			panic(err)
		}
	}

	assertTopDown(t, store, i, note, []string{"p"}, "{}", expected)
}

func assertTopDown(t *testing.T, store *storage.DataStore, i int, note string, path []string, globals string, expected interface{}) {

	g := storage.NewBindings()
	for _, i := range ast.MustParseTerm(globals).Value.(ast.Object) {
		g.Put(i[0].Value, i[1].Value)
	}

	p := []interface{}{}
	for _, x := range path {
		p = append(p, x)
	}

	switch e := expected.(type) {

	case error:
		result, err := Query(&QueryParams{DataStore: store, Path: p, Globals: g})
		if err == nil {
			t.Errorf("Test case %d (%v): expected error but got: %v", i+1, note, result)
			return
		}
		if err.Error() != e.Error() {
			t.Errorf("Test case %d (%v): expected error %v but got: %v", i+1, note, e, err)
		}

	case string:
		expected := loadExpectedSortedResult(e)
		result, err := Query(&QueryParams{DataStore: store, Path: p, Globals: g})
		if err != nil {
			t.Errorf("Test case %d (%v): unexpected error: %v", i+1, note, err)
			return
		}
		p := []interface{}{}
		for _, x := range path {
			p = append(p, x)
		}
		switch store.MustGet(p).([]*ast.Rule)[0].DocKind() {
		case ast.PartialSetDoc:
			sort.Sort(resultSet(result.([]interface{})))
		}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("Test case %d (%v): expected %v but got: %v", i+1, note, expected, result)
		}
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
