// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestBuiltinSprintf(t *testing.T) {
	tests := []struct {
		note   string
		format string
		args   *ast.Array
		exp    string
	}{
		{
			note:   "integer",
			format: "%d",
			args:   ast.NewArray(ast.NumberTerm("42")),
			exp:    "42",
		},
		{
			note:   "integer, multiple args",
			format: "%d-%d",
			args:   ast.NewArray(ast.NumberTerm("42"), ast.NumberTerm("-1")),
			exp:    "42--1",
		},
		{
			note:   "integer too large for int64",
			format: "%d",
			args:   ast.NewArray(ast.NumberTerm("1208925819614629174706175")),
			exp:    "1208925819614629174706175",
		},
		{
			note:   "float",
			format: "%f",
			args:   ast.NewArray(ast.NumberTerm("0.1")),
			exp:    "0.100000",
		},
		{
			// https://github.com/open-policy-agent/opa/issues/9187
			note:   "float with zero fraction",
			format: "float: %f, %3.1f, %.3f, %f, %3.1f, %.3f",
			args: ast.NewArray(
				ast.NumberTerm("0.1"), ast.NumberTerm("10.2"), ast.NumberTerm(".5"),
				ast.NumberTerm("0.0"), ast.NumberTerm("100.0"), ast.NumberTerm(".0"),
			),
			exp: "float: 0.100000, 10.2, 0.500, 0.000000, 100.0, 0.000",
		},
		{
			note:   "float in exponent notation",
			format: "%f",
			args:   ast.NewArray(ast.NumberTerm("1e2")),
			exp:    "100.000000",
		},
		{
			note:   "float too large for float64",
			format: "%s",
			args:   ast.NewArray(ast.NumberTerm("1e400")),
			exp:    "1e400",
		},
		{
			// The single argument case is served by an optimized path, and must
			// agree with the general one.
			note:   "float with integer verb",
			format: "%d",
			args:   ast.NewArray(ast.NumberTerm("1.0")),
			exp:    "%!d(float64=1)",
		},
		{
			note:   "float with integer verb, multiple args",
			format: "%d-%d",
			args:   ast.NewArray(ast.NumberTerm("1.0"), ast.NumberTerm("2")),
			exp:    "%!d(float64=1)-2",
		},
		{
			note:   "string",
			format: "%s",
			args:   ast.NewArray(ast.StringTerm("foo")),
			exp:    "foo",
		},
		{
			note:   "composite value",
			format: "%v",
			args:   ast.NewArray(ast.ArrayTerm(ast.NumberTerm("1"), ast.StringTerm("foo"))),
			exp:    `[1, "foo"]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var result *ast.Term

			operands := []*ast.Term{ast.StringTerm(tc.format), ast.NewTerm(tc.args)}
			err := builtinSprintf(BuiltinContext{}, operands, func(t *ast.Term) error {
				result = t
				return nil
			})
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if exp := ast.StringTerm(tc.exp); ast.Compare(exp, result) != 0 {
				t.Fatalf("Expected result:\n\n%s\n\ngot:\n\n%s", exp, result)
			}
		})
	}
}

func TestAnyStartsWithAny(t *testing.T) {
	tests := []struct {
		note     string
		strs     []string
		prefixes []string
		exp      bool
	}{
		{note: "no strings", prefixes: []string{"a"}},
		{note: "no prefixes", strs: []string{"a"}},
		{note: "empty prefix", strs: []string{"a", "b"}, prefixes: []string{""}, exp: true},
		{note: "empty string only matches empty prefix", strs: []string{"", "b"}, prefixes: []string{"a", "c"}},
		{note: "prefix equals string", strs: []string{"abc", "x"}, prefixes: []string{"q", "abc"}, exp: true},
		{note: "prefix longer than string", strs: []string{"ab", "x"}, prefixes: []string{"abc", "y"}},
		{note: "match is not the nearest string", strs: []string{"ab", "abd", "abc"}, prefixes: []string{"q", "abc"}, exp: true},
		{note: "prefix sorts after every string", strs: []string{"a", "b"}, prefixes: []string{"c", "d"}},
		{note: "byte-wise comparison", strs: []string{"é", "e"}, prefixes: []string{"\xc3", "z"}, exp: true},
		{note: "duplicates", strs: []string{"ab", "ab"}, prefixes: []string{"ab", "ab"}, exp: true},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			if got := anyStartsWithAny(tc.strs, tc.prefixes); got != tc.exp {
				t.Errorf("expected %v, got %v", tc.exp, got)
			}
		})
	}
}

func TestAnyStartsWithAnyMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	randStrings := func() []string {
		strs := make([]string, rng.IntN(8))
		for i := range strs {
			b := make([]byte, rng.IntN(4))
			for j := range b {
				b[j] = "abc"[rng.IntN(3)]
			}
			strs[i] = string(b)
		}
		return strs
	}

	for range 10000 {
		strs, prefixes := randStrings(), randStrings()
		exp := slices.ContainsFunc(strs, func(s string) bool {
			return slices.ContainsFunc(prefixes, func(p string) bool { return strings.HasPrefix(s, p) })
		})
		if got := anyStartsWithAny(slices.Clone(strs), prefixes); got != exp {
			t.Fatalf("anyStartsWithAny(%q, %q): expected %v, got %v", strs, prefixes, exp, got)
		}
	}
}

func TestBuiltinReverse(t *testing.T) {
	long := strings.Repeat("ab\xffé", 100) // past the 255-byte stack buffer
	tests := []struct {
		note string
		s    string
		exp  string
	}{
		{note: "empty", s: "", exp: ""},
		{note: "ascii", s: "abc", exp: "cba"},
		{note: "multi-byte runes", s: "añb€", exp: "€bña"},
		{note: "invalid byte first", s: "\xffab", exp: "ba\xff"},
		{note: "invalid byte last", s: "ab\xff", exp: "\xffba"},
		{note: "invalid byte between runes", s: "é\xff€", exp: "€\xffé"},
		{note: "truncated rune", s: "a\xe2\x82", exp: "\x82\xe2a"},
		{note: "encoded replacement character", s: "a�b", exp: "b�a"},
		{note: "long", s: long, exp: strings.Repeat("é\xffba", 100)},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var got string
			if err := builtinReverse(BuiltinContext{}, []*ast.Term{ast.StringTerm(tc.s)}, func(r *ast.Term) error {
				got = string(r.Value.(ast.String))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if got != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, got)
			}
		})
	}
}

func TestReverseStringArbitraryBytes(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for range 10000 {
		b := make([]byte, rng.IntN(300))
		for i := range b {
			b[i] = byte(rng.Uint32())
		}
		s := string(b)

		got := reverseString(s)

		// Whatever the input, the output holds the same bytes.
		if !slices.Equal(slices.Sorted(slices.Values([]byte(got))), slices.Sorted(slices.Values(b))) {
			t.Fatalf("reverseString(%q) = %q: not a rearrangement of the input", s, got)
		}
		// And a valid string comes back out as its runes in reverse.
		if valid := strings.ToValidUTF8(s, ""); reverseString(reverseString(valid)) != valid {
			t.Fatalf("reverseString is not its own inverse for %q", valid)
		}
	}
}

func TestBuiltinAnySuffixMatchInvalidUTF8(t *testing.T) {
	// Suffix matching reverses the strings, which used to panic on bytes that
	// aren't valid UTF-8.
	var got bool
	err := builtinAnySuffixMatch(BuiltinContext{}, []*ast.Term{ast.StringTerm("a\xff"), ast.StringTerm("\xff")}, func(r *ast.Term) error {
		got = bool(r.Value.(ast.Boolean))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected a match")
	}
}
