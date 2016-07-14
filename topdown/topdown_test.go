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
			err := evalRef(ctx, ast.MustParseRef(tc.ref), ast.Ref{}, func(ctx *Context) error {
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
			err := evalRef(ctx, ast.MustParseRef(tc.ref), ast.Ref{}, func(ctx *Context) error {
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
		{"data.a[data.a[i]] = 3", `[
			{"i": 0, "data.a[i]": 1},
			{"i": 1, "data.a[i]": 2},
			{"i": 2, "data.a[i]": 3}
		]`},
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

	ctx1 := &Context{Locals: storage.NewBindings(), Globals: storage.NewBindings()}
	ctx1.Bind(a, b, nil)
	ctx1.Bind(b, cs, nil)
	ctx1.Bind(c, ks, nil)
	ctx1.Bind(k, hello, nil)

	ctx2 := &Context{Locals: storage.NewBindings(), Globals: storage.NewBindings()}
	ctx2.Bind(a, b, nil)
	ctx2.Bind(b, cs, nil)
	ctx2.Bind(c, vs, nil)
	ctx2.Bind(v, world, nil)

	expected := ast.MustParseTerm(`[{"hello": "world"}]`).Value

	r1 := PlugValue(a, ctx1)

	if !expected.Equal(r1) {
		t.Errorf("Expected %v but got %v", expected, r1)
		return
	}

	r2 := PlugValue(a, ctx2)

	if !expected.Equal(r2) {
		t.Errorf("Expected %v but got %v", expected, r2)
	}

	n := ast.MustParseTerm("a.b[x.y[i]]").Value

	ctx3 := &Context{Locals: storage.NewBindings(), Globals: storage.NewBindings()}
	ctx3.Bind(ast.Var("i"), ast.Number(1), nil)
	ctx3.Bind(ast.MustParseTerm("x.y[i]").Value, ast.Number(1), nil)

	expected = ast.MustParseTerm("a.b[1]").Value

	r3 := PlugValue(n, ctx3)

	if !expected.Equal(r3) {
		t.Errorf("Expected %v but got: %v", expected, r3)
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
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, []string{tc.rule}, tc.expected)
	}
}

func TestTopDownEvalTermExpr(t *testing.T) {

	tests := []struct {
		note     string
		rule     string
		expected string
	}{
		{"true", "p :- true", "true"},
		{"false", "p :- false", ""},
		{"number non-zero", "p :- -3.14", "true"},
		{"number zero", "p :- null", "true"},
		{"null", "p :- null", "true"},
		{"string non-empty", `p :- "abc"`, "true"},
		{"string empty", `p :- ""`, "true"},
		{"array non-empty", "p :- [1,2,3]", "true"},
		{"array empty", "p :- []", "true"},
		{"object non-empty", `p :- {"a": 1}`, "true"},
		{"object empty", `p :- {}`, "true"},
		{"ref", "p :- a[i]", "true"},
		{"ref undefined", "p :- data.deadbeef[i]", ""},
		{"array comprehension", "p :- [x | x = 1]", "true"},
		{"array comprehension empty", "p :- [x | x = 1, x = 2]", "true"},
		{"arbitrary position", "p :- a[i] = x, x, i", "true"},
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
		{"output: set var binding", []string{"p[x] :- q[x]", "q[y] :- y = [i, j], i = 1, j = 2"}, `[[1,2]]`},
		{"output: object key", []string{"p[x] :- q[x] = 4", "q[i] = x :- a[i] = x"}, "[3]"},
		{"output: object value", []string{"p[x] = y :- q[x] = y", "q[k] = v :- b[k] = v"}, `{"v1": "hello", "v2": "goodbye"}`},
		{"output: object embedded", []string{"p[k] = v :- {k: [q[k]]} = {k: [v]}", `q[x] = y :- b[x] = y`}, `{"v1": "hello", "v2": "goodbye"}`},
		{"output: object dereference ground", []string{`p[i] :- q[i]["x"][1] = false`, `q[i] = x :- x = c[i]`}, "[0]"},
		{"output: object defererence non-ground", []string{
			`p[r] :- q[x][y][z] = false, r = [x, y, z]`,
			`q[i] = x :- x = c[i]`},
			`[[0, "x", 1], [0, "z", "q"]]`},
		{"output: object dereference array of refs", []string{
			"p[x] :- q[_][0].c[_] = x",
			"q[k] = v :- d.e[_] = k, v = [ r | r = l[_] ]",
		}, "[1,1,2,2,3,3,4,4]"},
		{"output: object dereference array of refs within object", []string{
			"p[x] :- q[_].x[0].c[_] = x",
			`q[k] = v :- d.e[_] = k, v = {"x": [r | r = l[_]]}`,
		}, "[1,1,2,2,3,3,4,4]"},
		{"output: object dereference object with key refs", []string{
			"p :- q.bar[1].alice[0] = 1",
			"q[k] = v :- d.e[_] = k, v = [x | x = {l[_].a: [1]}]",
		}, "true"},
		{"output: object var binding", []string{
			"p[z] :- q[x] = y, z = [x, y]",
			`q[k] = v :- v = [x, y], x = "a", y = "b", k = "foo"`},
			`[["foo", ["a", "b"]]]`},
		{"output: object key var binding", []string{
			"p[z] :- q[x] = y, z = [x, y]",
			`q[k] = v :- k = y, y = x, x = "a", v = "foo"`},
			`[["a", "foo"]]`},

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

		// no dereferencing
		{"no suffix: complete", []string{"p = true :- q", "q = true :- true"}, "true"},
		{"no suffix: complete incr (error)", []string{"p = true :- q", "q = false :- true", "q = true :- true"}, fmt.Errorf("evaluation error (code: 2): multiple values for data.q: rules must produce exactly one value for complete documents: check rule definition(s): q")},
		{"no suffix: complete incr", []string{"p = true :- not q", "q = true :- false", "q = false :- true"}, "true"},
		{"no suffix: object", []string{"p[x] = y :- q = o, o[x] = y", "q[x] = y :- b[x] = y"}, `{"v1": "hello", "v2": "goodbye"}`},
		{"no suffix: object incr", []string{"p[x] = y :- q = o, o[x] = y", "q[x] = y :- b[x] = y", "q[x1] = y1 :- b[x1] = y1"}, fmt.Errorf("evaluation error (code: 2): multiple values for data.q: rules must produce exactly one value for object document keys: check rule definition(s): q")},
		{"no suffix: object incr", []string{
			"p[x] = y :- q = o, o[x] = y",
			"q[x] = y :- b[x] = y",
			`q[x1] = y1 :- d["e"][y1] = x1`},
			`{"v1": "hello", "v2": "goodbye", "bar": 0, "baz": 1}`},
		{"no suffix: chained", []string{
			"p = true :- q = x, x[i] = 4",
			"q[k] = v :- r = x, x[k] = v",
			"r[k] = v :- s = x, x[k] = v",
			"r[k] = v :- t = x, x[v] = k",
			`s = {"a": 1, "b": 2, "c": 4} :- true`,
			`t = ["d", "e", "g"] :- true`},
			"true"},
		{"no suffix: object var binding", []string{
			"p[x] :- q = x",
			`q[k] = v :- v = [i, j], k = i, i = "a", j = 1`},
			`[{"a": ["a", 1]}]`},

		// TODO(tsandall): this requires set literals; "s" must be bound to a "set" type.
		// {"no deref: set", []string{"p[x] :- q = s, s[x]", "q[x] :- a[i] = x"}, "[1,2,3,4]"},

		// undefined
		// {"undefined: dereference set", []string{"p = true :- q[x].foo = 100", "q[x] :- x = a[i]"}, ""},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownNestedReferences(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		// nested base document references
		{"ground ref", []string{"p :- a[h[0][0]] = 2"}, "true"},
		{"non-ground ref", []string{"p[x] :- x = a[h[i][j]]"}, "[2,3,4,3,4]"},
		{"two deep", []string{"p[x] :- x = a[a[a[i]]]"}, "[3,4]"},
		{"two deep", []string{"p[x] :- x = a[h[i][a[j]]]"}, "[3,4,4]"},
		{"two deep repeated var", []string{"p[x] :- x = a[h[i][a[i]]]"}, "[3]"},
		{"no suffix", []string{"p :- 4 = a[three]"}, "true"},
		{"var ref", []string{"p[y] :- x = [1,2,3], y = a[x[_]]"}, "[2,3,4]"},
		{"undefined", []string{"p :- a[three.deadbeef] = x"}, ""},

		// nested virtual document references
		{"vdoc ref: complete", []string{"p[x] :- x = a[q[_]]", "q = [2,3] :- true"}, "[3,4]"},
		{"vdoc ref: complete: ground", []string{"p[x] :- x = a[q[1]]", "q = [2,3] :- true"}, "[4]"},
		{"vdoc ref: complete: no suffix", []string{"p :- 2 = a[q]", "q = 1 :- true"}, "true"},
		{"vdoc ref: partial object", []string{
			"p[x] :- x = a[q[_]]",
			`q[k] = v :- o = {"a": 2, "b": 3, "c": 100}, o[k] = v`},
			"[3,4]"},
		{"vdoc ref: partial object: ground", []string{
			`p[x] :- x = a[q["b"]]`,
			`q[k] = v :- o = {"a": 2, "b": 3, "c": 100}, o[k] = v`},
			"[4]"},

		// mixed cases
		{"vdoc ref: complete: nested bdoc ref", []string{
			"p[x] :- x = a[q[b[_]]]",
			`q = {"hello": 1, "goodbye": 3, "deadbeef": 1000} :- true`}, "[2,4]"},
		{"vdoc ref: partial object: nested bdoc ref", []string{
			"p[x] :- x = a[q[b[_]]]",
			// bind to value
			`q[k] = v :- o = {"hello": 1, "goodbye": 3, "deadbeef": 1000}, o[k] = v`}, "[2,4]"},
		{"vdoc ref: partial object: nested bdoc ref-2", []string{
			"p[x] :- x = a[q[d.e[_]]]",
			// bind to reference
			`q[k] = v :- strings[k] = v`}, "[3,4]"},
		{"vdoc ref: multiple", []string{
			"p[x] :- x = q[a[_]].v[r[a[_]]]",
			`q = [{"v": {}}, {"v": [0,0,1,2]}, {"v": [0,0,3,4]}, {"v": [0,0]}, {}] :- true`,
			"r = [1,2,3,4] :- true"}, "[1,2,3,4]"},
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
		{"complete: undefined", []string{"p :- false", "p :- false"}, ""},
		{"complete: error", []string{"p :- true", "p = false :- true"}, fmt.Errorf("evaluation error (code: 2): multiple values for [p]: rules must produce exactly one value for complete documents: check rule definition(s): p")},
		{"complete: valid", []string{"p :- true", "p = true :- true"}, "true"},
		{"complete: valid-2", []string{"p :- true", "p = false :- false"}, "true"},
		{"complete: reference error", []string{"p :- q", "q :- true", "q = false :- true"}, fmt.Errorf("evaluation error (code: 2): multiple values for data.q: rules must produce exactly one value for complete documents: check rule definition(s): q")},
		{"complete: reference valid", []string{"p :- q", "q :- true", "q = true :- true"}, "true"},
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
		{"neg: set contains undefined", []string{`p = true :- not q["v2"]`, `q[x] :- b[x] = v`}, ""},

		// TODO(tsandall): 'not g[j][k] ...' is not valid.
		// {"neg: multiple exprs", []string{"p[x] :- a[x] = i, not g[j][k] = x, f[v][y][z] = x"}, "[3]"},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownComprehensions(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"simple", []string{"p[i] :- xs = [x | x = a[_]], xs[i] > 1"}, "[1,2,3]"},
		{"nested", []string{"p[i] :- ys = [y | y = x[_], x = [z | z = a[_]]], ys[i] > 1"}, "[1,2,3]"},
		{"embedded array", []string{"p[i] :- xs = [[x | x = a[_]]], xs[0][i] > 1"}, "[1,2,3]"},
		{"embedded object", []string{`p[i] :- xs = {"a": [x | x = a[_]]}, xs["a"][i] > 1`}, "[1,2,3]"},
		{"closure", []string{"p[x] :- y = 1, x = [y | y = 1]"}, "[[1]]"},
		{"dereference embedded", []string{
			"p[x] :- q.a[2][i] = x",
			`q[k] = v :- k = "a", v = [y | i[_] = _, i = y, i = [ z | z = a[_]] ]`,
		}, "[1,2,3,4]"},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownAggregates(t *testing.T) {

	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"count", []string{"p[x] :- count(a, x)"}, "[4]"},
		{"count virtual", []string{"p[x] :- count([y | q[y]], x)", "q[x] :- x = a[_]"}, "[4]"},
		{"count keys", []string{"p[x] :- count(b, x)"}, "[2]"},
		{"count keys virtual", []string{"p[x] :- count([k | q[k] = _], x)", "q[k] = v :- b[k] = v"}, "[2]"},
		{"sum", []string{"p[x] :- sum([1,2,3,4], x)"}, "[10]"},
		{"sum virtual", []string{"p[x] :- sum([y | q[y]], x)", "q[x] :- a[_] = x"}, "[10]"},
		{"max", []string{"p[x] :- max([1,2,3,4], x)"}, "[4]"},
		{"max virtual", []string{"p[x] :- max([y | q[y]], x)", "q[x] :- a[_] = x"}, "[4]"},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownArithmetic(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"plus", []string{"p[y] :- a[i] = x, plus(i, x, y)"}, "[1,3,5,7]"},
		{"minus", []string{"p[y] :- a[i] = x, minus(i, x, y)"}, "[-1,-1,-1,-1]"},
		{"multiply", []string{"p[y] :- a[i] = x, mul(i, x, y)"}, "[0,2,6,12]"},
		{"divide+round", []string{"p[z] :- a[i] = x, div(i, x, y), round(y, z)"}, "[0,1,1,1]"},
		{"divide+error", []string{"p[y] :- a[i] = x, div(x, i, y)"}, fmt.Errorf("divide: by zero")},
		{"abs", []string{"p :- abs(-10, x), x = 10"}, "true"},
	}

	data := loadSmallTestData()

	for i, tc := range tests {
		runTopDownTestCase(t, data, i, tc.note, tc.rules, tc.expected)
	}
}

func TestTopDownCasts(t *testing.T) {
	tests := []struct {
		note     string
		rules    []string
		expected interface{}
	}{
		{"to_number", []string{`p[x] :- to_number("-42.0", y), to_number(false, z), x = [y, z]`}, "[[-42.0, 0]]"},
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
		 import req3.a.b
		 import req4.a.b as req4as
		 p = true :- a[i] = x, req1.foo = x, req2as.bar = x, q[x]
		 q[x] :- req1.foo = x, req2as.bar = x, r[x]
		 r[x] :- {"foo": req2as.bar, "bar": [x]} = {"foo": x, "bar": [req1.foo]}
		 s :- b.x[0] = 1
		 t :- req4as.x[0] = 1`})

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

	assertTopDown(t, store, 1, "global vars (missing)", []string{"z", "p"}, `{
		req1: {"foo": 4}
	}`, unboundGlobalVarErr(ast.MustParseRef("req2.bar")))

	assertTopDown(t, store, 2, "global vars (namespaced)", []string{"z", "s"}, `{
		req3: {
			"a": {
				"b": {
					"x": [1,2,3,4]
				}
			}
		}
	}`, "true")

	assertTopDown(t, store, 2, "global vars (namespaced w/ alias)", []string{"z", "t"}, `{
		req4: {
			"a": {
				"b": {
					"x": [1,2,3,4]
				}
			}
		}
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
            server = servers[_],
            server.ports[_] = ports[i].id,
            ports[i].networks[_] = networks[j].id,
            networks[j].public = true

        violations[server] :-
            server = servers[_],
            server.protocols[_] = "http",
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
				buf.Put(ast.MustParseTerm(k).Value, ast.String(v))
			case float64:
				buf.Put(ast.MustParseTerm(k).Value, ast.Number(v))
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
		"strings": {
			"foo": 1,
			"bar": 2,
			"baz": 3
		},
		"three": 3,
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
