// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestMakeGlobals(t *testing.T) {

	tests := []struct {
		note     string
		globals  [][2]string
		expected interface{}
	}{
		{"var", [][2]string{{`hello`, `"world"`}}, `{hello: "world"}`},
		{"multiple vars", [][2]string{{`a`, `"a"`}, {`b`, `"b"`}}, `{a: "a", b: "b"}`},
		{"multiple overlapping vars",
			[][2]string{{`a.b.c`, `"c"`}, {`a.b.d`, `"d"`}, {`x.y`, `[]`}},
			`{a: {"b": {"c": "c", "d": "d"}}, x: {"y": []}}`},
		{"ref value",
			[][2]string{{"foo.bar", "data.com.example.widgets[i]"}},
			`{foo: {"bar": data.com.example.widgets[i]}}`},
		{"conflicting vars",
			[][2]string{{`a.b`, `"c"`}, {`a.b.d`, `"d"`}},
			globalConflictErr(ast.MustParseRef("a.b.d"))},
		{"conflicting vars-2",
			[][2]string{{`a.b`, `{"c":[]}`}, {`a.b.c`, `["d"]`}},
			globalConflictErr(ast.MustParseRef("a.b.c"))},
		{"conflicting vars-3",
			[][2]string{{"a", "100"}, {`a.b`, `"c"`}},
			globalConflictErr(ast.MustParseRef("a.b"))},
		{"conflicting vars-4",
			[][2]string{{`a.b`, `"c"`}, {`a`, `100`}},
			globalConflictErr(ast.MustParseTerm("a").Value)},
		{"bad path",
			[][2]string{{`"hello"`, `1`}},
			fmt.Errorf(`invalid global: "hello": path must be a variable or a reference`)},
	}

	for i, tc := range tests {

		pairs := make([][2]*ast.Term, len(tc.globals))

		for j := range tc.globals {
			k := ast.MustParseTerm(tc.globals[j][0])
			v := ast.MustParseTerm(tc.globals[j][1])
			pairs[j] = [...]*ast.Term{k, v}
		}

		bindings, err := MakeGlobals(pairs)

		switch e := tc.expected.(type) {
		case error:
			if err == nil {
				t.Errorf("%v (#%d): Expected error %v but got: %v", tc.note, i+1, e, bindings)
				continue
			}
			if !reflect.DeepEqual(e, err) {
				t.Errorf("%v (#%d): Expected error %v but got: %v", tc.note, i+1, e, err)
			}
		case string:
			if err != nil {
				t.Errorf("%v (#%d): Unexpected error: %v", tc.note, i+1, err)
				continue
			}
			exp := ast.NewValueMap()
			for _, i := range ast.MustParseTerm(e).Value.(ast.Object) {
				exp.Put(i[0].Value, i[1].Value)
			}
			if !exp.Equal(bindings) {
				t.Errorf("%v (#%d): Expected bindings to equal %v but got: %v", tc.note, i+1, exp, bindings)
			}
		}
	}
}
