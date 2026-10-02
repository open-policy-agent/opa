// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/open-policy-agent/opa/internal/prefixtrie"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
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

func TestBuiltinAnyPrefixMatch(t *testing.T) {
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
		{note: "nested prefixes", strs: []string{"abd"}, prefixes: []string{"abc", "a"}, exp: true},
		{note: "byte-wise comparison", strs: []string{"é", "e"}, prefixes: []string{"\xc3", "z"}, exp: true},
		{note: "duplicates", strs: []string{"ab", "ab"}, prefixes: []string{"ab", "ab"}, exp: true},
	}
	for _, tc := range tests {
		for _, cached := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cached=%v", tc.note, cached), func(t *testing.T) {
				bctx := affixBuiltinContext(cached)
				// A cached run records the collection, then builds its trie, then
				// answers from the cache.
				for range 3 {
					if got := callAffixBuiltin(t, bctx, builtinAnyPrefixMatch, tc.strs, tc.prefixes); got != tc.exp {
						t.Errorf("expected %v, got %v", tc.exp, got)
					}
				}
			})
		}
	}
}

func TestBuiltinAnySuffixMatch(t *testing.T) {
	tests := []struct {
		note     string
		strs     []string
		suffixes []string
		exp      bool
	}{
		{note: "no strings", suffixes: []string{"a"}},
		{note: "no suffixes", strs: []string{"a"}},
		{note: "empty suffix", strs: []string{"a", "b"}, suffixes: []string{""}, exp: true},
		{note: "suffix equals string", strs: []string{"abc", "x"}, suffixes: []string{"q", "abc"}, exp: true},
		{note: "suffix longer than string", strs: []string{"bc", "x"}, suffixes: []string{"abc", "y"}},
		{note: "nested suffixes", strs: []string{"xbc"}, suffixes: []string{"abc", "c"}, exp: true},
		{note: "multi-byte rune", strs: []string{"café"}, suffixes: []string{"é"}, exp: true},
		{note: "byte-wise comparison", strs: []string{"é"}, suffixes: []string{"\xa9"}, exp: true},
		// A search string that isn't valid UTF-8 is compared byte by byte like
		// any other, rather than having its runes decoded.
		{note: "invalid UTF-8", strs: []string{"a\xff"}, suffixes: []string{"\xff", "z"}, exp: true},
	}
	for _, tc := range tests {
		for _, cached := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/cached=%v", tc.note, cached), func(t *testing.T) {
				bctx := affixBuiltinContext(cached)
				for range 3 {
					if got := callAffixBuiltin(t, bctx, builtinAnySuffixMatch, tc.strs, tc.suffixes); got != tc.exp {
						t.Errorf("expected %v, got %v", tc.exp, got)
					}
				}
			})
		}
	}
}

func TestBuiltinAnyAffixMatchMatchesBruteForce(t *testing.T) {
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

	// One cache for every case, so that collections are both looked up and
	// evicted while the results are checked.
	bctx := affixBuiltinContext(true)
	for range 10000 {
		strs, bases := randStrings(), randStrings()

		expPrefix := slices.ContainsFunc(strs, func(s string) bool {
			return slices.ContainsFunc(bases, func(p string) bool { return strings.HasPrefix(s, p) })
		})
		// Twice, so that the second call matches with a trie.
		for range 2 {
			if got := callAffixBuiltin(t, bctx, builtinAnyPrefixMatch, strs, bases); got != expPrefix {
				t.Fatalf("any_prefix_match(%q, %q): expected %v, got %v", strs, bases, expPrefix, got)
			}
		}

		expSuffix := slices.ContainsFunc(strs, func(s string) bool {
			return slices.ContainsFunc(bases, func(p string) bool { return strings.HasSuffix(s, p) })
		})
		for range 2 {
			if got := callAffixBuiltin(t, bctx, builtinAnySuffixMatch, strs, bases); got != expSuffix {
				t.Fatalf("any_suffix_match(%q, %q): expected %v, got %v", strs, bases, expSuffix, got)
			}
		}

		// A single search string is matched differently, with and without a trie.
		for _, s := range strs {
			for _, tc := range []struct {
				fn    BuiltinFunc
				match func(string, string) bool
			}{
				{builtinAnyPrefixMatch, strings.HasPrefix},
				{builtinAnySuffixMatch, strings.HasSuffix},
			} {
				exp := slices.ContainsFunc(bases, func(b string) bool { return tc.match(s, b) })
				for range 2 {
					var got bool
					if err := tc.fn(bctx, []*ast.Term{ast.StringTerm(s), stringsArray(bases)}, func(r *ast.Term) error {
						got = bool(r.Value.(ast.Boolean))
						return nil
					}); err != nil {
						t.Fatal(err)
					}
					if got != exp {
						t.Fatalf("matching %q against %q: expected %v, got %v", s, bases, exp, got)
					}
				}
			}
		}
	}
}

func TestBuiltinAnyAffixMatchCache(t *testing.T) {
	bctx := affixBuiltinContext(true)
	bases := ast.NewArray(ast.StringTerm("/api/"), ast.StringTerm(".rego"))

	call := func(fn BuiltinFunc, s string, bases *ast.Array) bool {
		t.Helper()
		var result bool
		if err := fn(bctx, []*ast.Term{ast.StringTerm(s), ast.NewTerm(bases)}, func(r *ast.Term) error {
			result = bool(r.Value.(ast.Boolean))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		return result
	}
	prefixes := bctx.InterQueryBuiltinValueCache.GetCache(anyPrefixMatchCacheName)
	seen := bctx.InterQueryBuiltinValueCache.GetCache(anyPrefixMatchSeenCacheName)

	// The first time, the collection is only recorded as seen, by hash.
	if !call(builtinAnyPrefixMatch, "/api/x", bases) {
		t.Fatal("expected a prefix match")
	}
	if v, ok := seen.Get(affixSeenKey(bases)); !ok {
		t.Fatal("expected the collection to be recorded as seen")
	} else if _, ok := v.(*affixSeen); !ok {
		t.Fatalf("expected a seen marker, got %v", v)
	}
	if v, ok := prefixes.Get(bases); ok {
		t.Fatalf("expected no trie yet, got %v", v)
	}

	// The second time, its trie is built and cached.
	if call(builtinAnyPrefixMatch, "/nope", bases) {
		t.Fatal("expected no prefix match")
	}
	cached, _ := prefixes.Get(bases)
	if _, ok := cached.(*prefixtrie.Trie[struct{}]); !ok {
		t.Fatalf("expected the prefix trie to be cached, got %v", cached)
	}

	// An equal collection that isn't the same value finds the same trie.
	equal := ast.NewArray(ast.StringTerm("/api/"), ast.StringTerm(".rego"))
	if !call(builtinAnyPrefixMatch, "/api/y", equal) {
		t.Fatal("expected a prefix match")
	}
	if again, _ := prefixes.Get(equal); again != cached {
		t.Error("expected an equal collection to reuse the cached trie")
	}

	// The same collection as suffixes is a different trie, in a cache of its own.
	if call(builtinAnySuffixMatch, "/api/x", bases) {
		t.Fatal("expected no suffix match")
	}
	if !call(builtinAnySuffixMatch, "x.rego", bases) {
		t.Fatal("expected a suffix match")
	}
	suffixes := bctx.InterQueryBuiltinValueCache.GetCache(anySuffixMatchCacheName)
	if v, _ := suffixes.Get(bases); v == cached {
		t.Error("expected a separately cached suffix trie")
	} else if _, ok := v.(*prefixtrie.Trie[struct{}]); !ok {
		t.Errorf("expected the suffix trie to be cached, got %v", v)
	}

	// A collection with a non-string member is an error, and isn't recorded.
	invalid := ast.NewArray(ast.StringTerm("a"), ast.NumberTerm("1"))
	for range 2 {
		err := builtinAnyPrefixMatch(bctx, []*ast.Term{ast.StringTerm("a"), ast.NewTerm(invalid)}, func(*ast.Term) error { return nil })
		if err == nil {
			t.Fatal("expected an error for a non-string base")
		}
	}
	if _, ok := prefixes.Get(invalid); ok {
		t.Error("expected an invalid collection not to be cached")
	}
	if _, ok := seen.Get(affixSeenKey(invalid)); ok {
		t.Error("expected an invalid collection not to be recorded as seen")
	}
}

func TestBuiltinAnyAffixMatchCacheMarkersDontEvictTries(t *testing.T) {
	bctx := affixBuiltinContext(true)
	static := []string{"/api/", "/admin/"}

	for range 2 {
		callAffixBuiltin(t, bctx, builtinAnyPrefixMatch, []string{"/api/x"}, static)
	}
	prefixes := bctx.InterQueryBuiltinValueCache.GetCache(anyPrefixMatchCacheName)
	trie, ok := prefixes.Get(stringsArray(static).Value)
	if !ok {
		t.Fatal("expected the static collection's trie to be cached")
	}

	// Each query's own collection, seen once, many times over the trie cache's size.
	for i := range 100 {
		callAffixBuiltin(t, bctx, builtinAnyPrefixMatch, []string{"/api/x"}, []string{fmt.Sprintf("/user/%d/", i)})
	}
	if again, ok := prefixes.Get(stringsArray(static).Value); !ok || again != trie {
		t.Error("expected the static collection's trie to survive collections seen only once")
	}
}

func TestBuiltinAnyAffixMatchCacheRebuildsEvictedTrie(t *testing.T) {
	bctx := affixBuiltinContext(true)
	bases := []string{"/api/", "/admin/"}
	key := stringsArray(bases).Value
	prefixes := bctx.InterQueryBuiltinValueCache.GetCache(anyPrefixMatchCacheName)

	for range 2 {
		callAffixBuiltin(t, bctx, builtinAnyPrefixMatch, []string{"/api/x"}, bases)
	}
	prefixes.Delete(key)

	// Matched without a trie, and marked to be built again on the next call.
	if !callAffixBuiltin(t, bctx, builtinAnyPrefixMatch, []string{"/api/x"}, bases) {
		t.Fatal("expected a prefix match")
	}
	if _, ok := prefixes.Get(key); ok {
		t.Fatal("expected the trie not to be rebuilt straight away")
	}
	if callAffixBuiltin(t, bctx, builtinAnyPrefixMatch, []string{"/nope"}, bases) {
		t.Fatal("expected no prefix match")
	}
	if _, ok := prefixes.Get(key); !ok {
		t.Error("expected the trie to be rebuilt")
	}
}

func TestBuiltinAnyAffixMatchCacheConcurrentBuild(t *testing.T) {
	inner := cache.NewInterQueryValueCache(t.Context(), &cache.Config{})
	counting := &countingInsertsCache{InterQueryValueCache: inner, name: anyPrefixMatchCacheName}
	bctx := BuiltinContext{InterQueryBuiltinValueCache: counting}
	bases := stringsArray([]string{"/api/", "/admin/"})

	call := func() (bool, error) {
		var result bool
		err := builtinAnyPrefixMatch(bctx, []*ast.Term{ast.StringTerm("/api/x"), bases}, func(r *ast.Term) error {
			result = bool(r.Value.(ast.Boolean))
			return nil
		})
		return result, err
	}

	if _, err := call(); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for range 50 {
		wg.Go(func() {
			if found, err := call(); err != nil {
				errs <- err
			} else if !found {
				errs <- errors.New("expected a prefix match")
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	if n := counting.inserts.Load(); n != 1 {
		t.Errorf("expected the trie to be built and cached once, got %d", n)
	}
}

func TestBuiltinAnyAffixMatchCacheSeenDisabled(t *testing.T) {
	disabled := true
	config := &cache.Config{InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
		NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
			anyPrefixMatchSeenCacheName: {Disabled: &disabled},
		},
	}}
	bctx := BuiltinContext{InterQueryBuiltinValueCache: cache.NewInterQueryValueCache(t.Context(), config)}
	bases := []string{"/api/", "/admin/"}

	for range 3 {
		if !callAffixBuiltin(t, bctx, builtinAnyPrefixMatch, []string{"/api/x"}, bases) {
			t.Fatal("expected a prefix match")
		}
	}
	if v, ok := bctx.InterQueryBuiltinValueCache.GetCache(anyPrefixMatchCacheName).Get(stringsArray(bases).Value); ok {
		t.Errorf("expected no trie without the %s cache, got %v", anyPrefixMatchSeenCacheName, v)
	}
}

func affixSeenKey(bases ast.Value) ast.Value {
	return ast.Number(strconv.Itoa(bases.Hash()))
}

// countingInsertsCache counts the inserts into one of its named caches.
type countingInsertsCache struct {
	cache.InterQueryValueCache
	name    string
	inserts atomic.Int64
}

func (c *countingInsertsCache) GetCache(name string) cache.InterQueryValueCacheBucket {
	b := c.InterQueryValueCache.GetCache(name)
	if name != c.name || b == nil {
		return b
	}
	return countingInsertsBucket{b, &c.inserts}
}

type countingInsertsBucket struct {
	cache.InterQueryValueCacheBucket
	inserts *atomic.Int64
}

func (b countingInsertsBucket) Insert(k ast.Value, v any) int {
	b.inserts.Add(1)
	return b.InterQueryValueCacheBucket.Insert(k, v)
}

func TestBuiltinAnyAffixMatchCacheDisabled(t *testing.T) {
	disabled := true
	config := &cache.Config{InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
		NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
			anyPrefixMatchCacheName: {Disabled: &disabled},
		},
	}}
	bctx := BuiltinContext{InterQueryBuiltinValueCache: cache.NewInterQueryValueCache(t.Context(), config)}

	if !callAffixBuiltin(t, bctx, builtinAnyPrefixMatch, []string{"/api/x"}, []string{"/api/", "/admin/"}) {
		t.Fatal("expected a prefix match")
	}
	if c := bctx.InterQueryBuiltinValueCache.GetCache(anyPrefixMatchCacheName); c != nil {
		t.Errorf("expected the %s cache to be disabled, got %v", anyPrefixMatchCacheName, c)
	}
}

func affixBuiltinContext(cached bool) BuiltinContext {
	if !cached {
		return BuiltinContext{}
	}
	return BuiltinContext{InterQueryBuiltinValueCache: cache.NewInterQueryValueCache(context.Background(), &cache.Config{})}
}

func callAffixBuiltin(t *testing.T, bctx BuiltinContext, fn BuiltinFunc, strs, bases []string) bool {
	t.Helper()
	var result bool
	err := fn(bctx, []*ast.Term{stringsArray(strs), stringsArray(bases)}, func(r *ast.Term) error {
		result = bool(r.Value.(ast.Boolean))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func stringsArray(strs []string) *ast.Term {
	terms := make([]*ast.Term, len(strs))
	for i, s := range strs {
		terms[i] = ast.StringTerm(s)
	}
	return ast.ArrayTerm(terms...)
}
