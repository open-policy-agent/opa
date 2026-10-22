// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"encoding/json"
	"testing"
)

func TestJSONByteSizeMatchesMarshal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note  string
		value string
	}{
		{note: "null", value: `null`},
		{note: "true", value: `true`},
		{note: "false", value: `false`},
		{note: "numbers", value: `[0, -1, 1.5, 1e100, 123456789012345678901234567890]`},
		{note: "empty string", value: `""`},
		{note: "ascii string", value: `"hello world"`},
		{note: "multibyte string", value: `"héllo wörld ✓ 日本語 😀"`},
		{note: "escapes", value: `"quote\" slash\\ nl\n cr\r tab\t bs\b ff\f"`},
		{note: "control characters", value: `"\u0000\u0001\u001f\u007f"`},
		{note: "html characters", value: `"<script>a && b</script>"`},
		{note: "empty array", value: `[]`},
		{note: "empty object", value: `{}`},
		{note: "single element array", value: `[1]`},
		{note: "nested", value: `{"a": [1, 2, {"b": null}], "ключ<>": {"c": "d"}, "": []}`},
		{note: "set", value: `{1, "two", [3]}`},
		{note: "empty set", value: `set()`},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			assertJSONByteSize(t, MustParseTerm(tc.value).Value)
		})
	}

	t.Run("invalid utf-8", func(t *testing.T) {
		t.Parallel()

		assertJSONByteSize(t, String("a\xffb\xc3"))
	})

	t.Run("line separators", func(t *testing.T) {
		t.Parallel()

		assertJSONByteSize(t, String("a\u2028b\u2029c"))
	})
}

func TestJSONByteSizeLazyObject(t *testing.T) {
	t.Parallel()

	// The native values are marshaled as is, so numbers must be sized as Go formats
	// them rather than as their AST counterparts.
	native := map[string]any{
		"small":  1e-7,
		"large":  12345678901234567890.0,
		"number": json.Number("1.50"),
		"html":   "<x>é",
		"list":   []any{1.0, "x", nil},
	}

	assertJSONByteSize(t, LazyObject(native))

	t.Run("nested in object", func(t *testing.T) {
		t.Parallel()

		assertJSONByteSize(t, NewObject(Item(StringTerm("k"), NewTerm(LazyObject(native)))))
	})

	t.Run("nested in array", func(t *testing.T) {
		t.Parallel()

		assertJSONByteSize(t, NewArray(NewTerm(LazyObject(native))))
	})
}

func TestJSONByteSizeDeclines(t *testing.T) {
	t.Parallel()

	for _, v := range []Value{
		MustParseTerm(`{1: "a", "1": "b", [1]: "c"}`).Value,
		MustParseTerm(`[{1: "a"}]`).Value,
		Var("x"),
	} {
		if _, ok := JSONByteSize(v); ok {
			t.Errorf("expected size walk to decline %v", v)
		}
	}
}

func assertJSONByteSize(t *testing.T, v Value) {
	t.Helper()

	got, ok := JSONByteSize(v)
	if !ok {
		t.Fatalf("size walk declined %v", v)
	}

	asJSON, err := JSON(v)
	if err != nil {
		t.Fatal(err)
	}
	bs, err := json.Marshal(asJSON)
	if err != nil {
		t.Fatal(err)
	}

	if got != len(bs) {
		t.Fatalf("expected size %d for %s, got %d", len(bs), bs, got)
	}
}
