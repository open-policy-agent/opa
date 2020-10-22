// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestMergeTermWithValues(t *testing.T) {

	tests := []struct {
		note     string
		exist    string
		input    [][2]string
		expected interface{}
	}{
		{
			note:     "var",
			input:    [][2]string{{`input.hello`, `"world"`}},
			expected: `{"hello": "world"}`,
		},
		{
			note:     "multiple vars",
			input:    [][2]string{{`input.a`, `"a"`}, {`input.b`, `"b"`}},
			expected: `{"a": "a", "b": "b"}`,
		},
		{
			note:     "multiple overlapping vars",
			input:    [][2]string{{`input.a.b.c`, `"c"`}, {`input.a.b.d`, `"d"`}, {`input.x.y`, `[]`}},
			expected: `{"a": {"b": {"c": "c", "d": "d"}}, "x": {"y": []}}`,
		},
		{
			note:     "ref value",
			input:    [][2]string{{"input.foo.bar", "data.com.example.widgets[i]"}},
			expected: `{"foo": {"bar": data.com.example.widgets[i]}}`,
		},
		{
			note:     "non-object",
			input:    [][2]string{{"input", "[1,2,3]"}},
			expected: "[1,2,3]",
		},
		{
			note:     "conflicting value",
			input:    [][2]string{{"input", "[1,2,3]"}, {"input.a.b", "true"}},
			expected: errConflictingDoc,
		},
		{
			note:     "conflicting merge",
			input:    [][2]string{{`input.a.b`, `"c"`}, {`input.a.b.d`, `"d"`}},
			expected: errConflictingDoc,
		},
		{
			note:     "ordered roots",
			input:    [][2]string{{"input", `"a"`}, {"input", `"b"`}},
			expected: `"b"`,
		},
		{
			note:     "bad import path",
			input:    [][2]string{{`input.a[1]`, `1`}},
			expected: errBadPath,
		},
		{
			note:     "existing merge",
			exist:    `{"foo": {"bar": 1}}`,
			input:    [][2]string{{"input.foo.baz", "2"}},
			expected: `{"foo": {"bar": 1, "baz": 2}}`,
		},
		{
			note:     "existing overwrite",
			exist:    `{"a": {"b": 1, "c": 2}}`,
			input:    [][2]string{{"input.a", `{"d": 3}`}},
			expected: `{"a": {"d": 3}}`,
		},
	}

	for i, tc := range tests {

		t.Run(tc.note, func(t *testing.T) {

			pairs := make([][2]*ast.Term, len(tc.input))

			for j := range tc.input {
				var k *ast.Term
				k = ast.MustParseTerm(tc.input[j][0])
				v := ast.MustParseTerm(tc.input[j][1])
				pairs[j] = [...]*ast.Term{k, v}
			}

			var exist *ast.Term

			if tc.exist != "" {
				exist = ast.MustParseTerm(tc.exist)
			}

			input, err := mergeTermWithValues(exist, pairs)

			switch e := tc.expected.(type) {
			case error:
				if err == nil {
					t.Fatalf("%v (#%d): Expected error %v but got: %v", tc.note, i+1, e, input)
				}
				if err.Error() != e.Error() {
					t.Fatalf("%v (#%d): Expected error %v but got: %v", tc.note, i+1, e, err)
				}
			case string:
				if err != nil {
					t.Fatalf("%v (#%d): Unexpected error: %v", tc.note, i+1, err)
				}
				expected := ast.MustParseTerm(e)
				if expected.Value.Compare(input.Value) != 0 {
					t.Fatalf("%v (#%d): Expected input to equal %v but got: %v", tc.note, i+1, expected, input)
				}
			}
		})
	}
}

func TestMergeTermWithValuesInputsShouldBeImmutable(t *testing.T) {

	initial := ast.MustParseTerm(`{"foo": 1}`)
	expInitial := initial.Copy()
	two := ast.MustParseTerm(`2`)

	result, err := mergeTermWithValues(nil, [][2]*ast.Term{
		{ast.MustParseTerm("input"), initial},
		{ast.MustParseTerm("input.foo"), two},
	})

	if err != nil {
		t.Fatal(err)
	}

	exp := ast.MustParseTerm(`{"foo": 2}`)

	if !result.Equal(exp) {
		t.Fatalf("expected %v but got %v", exp, result)
	}

	if !initial.Equal(expInitial) {
		t.Fatalf("expected input value to be unchanged but got %v (expected: %v)", initial, expInitial)
	}
}
