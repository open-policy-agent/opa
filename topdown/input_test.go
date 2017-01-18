// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestMakeInput(t *testing.T) {

	tests := []struct {
		note     string
		input    [][2]string
		expected interface{}
	}{
		{"var", [][2]string{{`hello`, `"world"`}}, `{"hello": "world"}`},
		{"multiple vars", [][2]string{{`a`, `"a"`}, {`b`, `"b"`}}, `{"a": "a", "b": "b"}`},
		{"multiple overlapping vars",
			[][2]string{{`a.b.c`, `"c"`}, {`a.b.d`, `"d"`}, {`x.y`, `[]`}},
			`{"a": {"b": {"c": "c", "d": "d"}}, "x": {"y": []}}`},
		{"ref value",
			[][2]string{{"foo.bar", "data.com.example.widgets[i]"}},
			`{"foo": {"bar": data.com.example.widgets[i]}}`},
		{"non-object", [][2]string{{"", "[1,2,3]"}}, "[1,2,3]"},
		{"non-object conflict",
			[][2]string{{"", "[1,2,3]"}, {"a.b", "true"}},
			fmt.Errorf("conflicting input values: check input parameters")},
		{"conflicting vars",
			[][2]string{{`a.b`, `"c"`}, {`a.b.d`, `"d"`}},
			fmt.Errorf("conflicting input value input.a.b.d: check input parameters")},
		{"conflicting vars-2",
			[][2]string{{`a.b`, `{"c":[]}`}, {`a.b.c`, `["d"]`}},
			fmt.Errorf("conflicting input value input.a.b.c: check input parameters")},
		{"conflicting vars-3",
			[][2]string{{"a", "100"}, {`a.b`, `"c"`}},
			fmt.Errorf("conflicting input value input.a.b: check input parameters")},
		{"conflicting vars-4",
			[][2]string{{`a.b`, `"c"`}, {`a`, `100`}},
			fmt.Errorf("conflicting input value input.a: check input parameters")},
		{"bad path",
			[][2]string{{`a[1]`, `1`}},
			fmt.Errorf("invalid input path: invalid path input.a[1]: path elements must be strings"),
		},
	}

	for i, tc := range tests {

		pairs := make([][2]*ast.Term, len(tc.input))

		for j := range tc.input {
			var k *ast.Term
			if len(tc.input[j][0]) == 0 {
				k = ast.NewTerm(ast.EmptyRef())
			} else {
				k = ast.MustParseTerm("input." + tc.input[j][0])
			}
			v := ast.MustParseTerm(tc.input[j][1])
			pairs[j] = [...]*ast.Term{k, v}
		}

		input, err := MakeInput(pairs)

		switch e := tc.expected.(type) {
		case error:
			if err == nil {
				t.Errorf("%v (#%d): Expected error %v but got: %v", tc.note, i+1, e, input)
				continue
			}
			if err.Error() != e.Error() {
				t.Errorf("%v (#%d): Expected error %v but got: %v", tc.note, i+1, e, err)
			}
		case string:
			if err != nil {
				t.Errorf("%v (#%d): Unexpected error: %v", tc.note, i+1, err)
				continue
			}
			expected := ast.MustParseTerm(e)
			if !expected.Value.Equal(input) {
				t.Errorf("%v (#%d): Expected input to equal %v but got: %v", tc.note, i+1, expected, input)
			}
		}
	}
}
