// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestMakeInput(t *testing.T) {

	tests := []struct {
		note     string
		input    [][2]string
		expected interface{}
	}{
		{"var",
			[][2]string{{`input.hello`, `"world"`}},
			`{"hello": "world"}`},
		{"multiple vars",
			[][2]string{{`input.a`, `"a"`}, {`input.b`, `"b"`}},
			`{"a": "a", "b": "b"}`},
		{"multiple overlapping vars",
			[][2]string{{`input.a.b.c`, `"c"`}, {`input.a.b.d`, `"d"`}, {`input.x.y`, `[]`}},
			`{"a": {"b": {"c": "c", "d": "d"}}, "x": {"y": []}}`},
		{"ref value",
			[][2]string{{"input.foo.bar", "data.com.example.widgets[i]"}},
			`{"foo": {"bar": data.com.example.widgets[i]}}`},
		{"non-object",
			[][2]string{{"input", "[1,2,3]"}},
			"[1,2,3]"},
		{"conflicting value",
			[][2]string{{"input", "[1,2,3]"}, {"input.a.b", "true"}},
			errConflictingInputDoc},
		{"conflicting merge",
			[][2]string{{`input.a.b`, `"c"`}, {`input.a.b.d`, `"d"`}},
			errConflictingInputDoc},
		{"conflicting roots",
			[][2]string{{"input", `"a"`}, {"input", `"b"`}},
			errConflictingInputDoc},
		{"bad import path",
			[][2]string{{`input.a[1]`, `1`}},
			errBadInputPath,
		},
	}

	for i, tc := range tests {

		pairs := make([][2]*ast.Term, len(tc.input))

		for j := range tc.input {
			var k *ast.Term
			k = ast.MustParseTerm(tc.input[j][0])
			v := ast.MustParseTerm(tc.input[j][1])
			pairs[j] = [...]*ast.Term{k, v}
		}

		input, err := makeInput(pairs)

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
			if expected.Value.Compare(input) != 0 {
				t.Errorf("%v (#%d): Expected input to equal %v but got: %v", tc.note, i+1, expected, input)
			}
		}
	}
}
