// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestMergeTermWithValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note     string
		exist    string
		input    [][2]string
		expected any
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
			input:    [][2]string{{"input", "[1,2,3]"}, {"input.a", "true"}},
			expected: `{"a": true}`,
		},
		{
			note:     "conflicting value, nested trailing terms",
			input:    [][2]string{{"input", "[1,2,3]"}, {"input.a.b", "true"}},
			expected: `{"a": {"b": true}}`,
		},
		{
			note:     "conflicting merge",
			input:    [][2]string{{`input.a.b`, `"c"`}, {`input.a.b.d`, `"d"`}},
			expected: `{"a": {"b": {"d": "d"}}}`,
		},
		{
			note:     "ordered roots",
			input:    [][2]string{{"input", `"a"`}, {"input", `"b"`}},
			expected: `"b"`,
		},
		{
			note:     "bad path, var element",
			input:    [][2]string{{`input.a[x]`, `1`}},
			expected: errBadPath,
		},
		{
			note:     "array index, no existing array",
			input:    [][2]string{{`input.a[1]`, `1`}},
			expected: errNumberNotArray,
		},
		{
			note:     "array index, array from previous with",
			input:    [][2]string{{"input", `["foo", "bar", "baz"]`}, {"input[1]", `"foo"`}, {"input[2]", `"foo"`}},
			expected: `["foo", "foo", "foo"]`,
		},
		{
			note:     "array index, existing array",
			exist:    `{"a": [1, 2, 3]}`,
			input:    [][2]string{{"input.a[0]", "9"}},
			expected: `{"a": [9, 2, 3]}`,
		},
		{
			note:     "array index, nested object in array",
			exist:    `{"a": [{"b": 1, "c": 2}, {"b": 3}]}`,
			input:    [][2]string{{"input.a[0].b", "9"}, {"input.a[1].d", "4"}},
			expected: `{"a": [{"b": 9, "c": 2}, {"b": 3, "d": 4}]}`,
		},
		{
			note:     "array index, nested arrays",
			exist:    `[[1, 2], [3, 4]]`,
			input:    [][2]string{{"input[1][0]", "9"}},
			expected: `[[1, 2], [9, 4]]`,
		},
		{
			note:     "array index, element replaced by object tree",
			exist:    `[1, 2]`,
			input:    [][2]string{{"input[0].x.y", "9"}},
			expected: `[{"x": {"y": 9}}, 2]`,
		},
		{
			note:     "array index, out of bounds",
			exist:    `[1, 2, 3]`,
			input:    [][2]string{{"input[3]", "9"}},
			expected: errBadArrayIndex,
		},
		{
			note:     "array index, negative",
			exist:    `[1, 2, 3]`,
			input:    [][2]string{{"input[-1]", "9"}},
			expected: errBadArrayIndex,
		},
		{
			note:     "array index, not an integer",
			exist:    `[1, 2, 3]`,
			input:    [][2]string{{"input[1.5]", "9"}},
			expected: errBadArrayIndex,
		},
		{
			note:     "array index on object",
			exist:    `{"a": {"b": 1}}`,
			input:    [][2]string{{"input.a[0]", "9"}},
			expected: errNumberNotArray,
		},
		{
			note:     "array index below missing key",
			exist:    `{"a": 1}`,
			input:    [][2]string{{"input.b[0]", "9"}},
			expected: errNumberNotArray,
		},
		{
			note:     "string key on array still replaces array",
			exist:    `{"a": [1, 2]}`,
			input:    [][2]string{{"input.a.b", "9"}},
			expected: `{"a": {"b": 9}}`,
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
			t.Parallel()

			pairs := make([][2]*ast.Term, len(tc.input))

			for j := range tc.input {
				k := ast.MustParseTerm(tc.input[j][0])
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
	t.Parallel()

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

func TestMergeTermWithValuesArrayInputsShouldBeImmutable(t *testing.T) {
	t.Parallel()

	initial := ast.MustParseTerm(`["foo", "bar"]`)
	expInitial := initial.Copy()
	exist := ast.MustParseTerm(`{"a": [1, 2]}`)
	expExist := exist.Copy()

	result, err := mergeTermWithValues(exist, [][2]*ast.Term{
		{ast.MustParseTerm("input.a[0]"), ast.MustParseTerm("9")},
		{ast.MustParseTerm("input.b"), initial},
		{ast.MustParseTerm("input.b[1]"), ast.MustParseTerm(`"foo"`)},
	})
	if err != nil {
		t.Fatal(err)
	}

	exp := ast.MustParseTerm(`{"a": [9, 2], "b": ["foo", "foo"]}`)
	if !result.Equal(exp) {
		t.Fatalf("expected %v but got %v", exp, result)
	}
	if !initial.Equal(expInitial) {
		t.Fatalf("expected with value to be unchanged but got %v (expected: %v)", initial, expInitial)
	}
	if !exist.Equal(expExist) {
		t.Fatalf("expected existing document to be unchanged but got %v (expected: %v)", exist, expExist)
	}
}
