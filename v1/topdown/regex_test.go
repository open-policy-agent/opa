// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
)

func TestRegexBuiltinCache(t *testing.T) {
	// Not parallel: regexpCache is shared by the whole package, and parallel
	// tests calling regex built-ins would evict the patterns asserted on here.
	regexpCacheLock.Lock()
	regexpCache = make(map[string]*regexp.Regexp)
	regexpCacheLock.Unlock()

	ctx := BuiltinContext{}
	iter := func(*ast.Term) error { return nil }

	// A novel regex pattern is cached.
	regex1 := "foo.*"
	operands := []*ast.Term{
		ast.NewTerm(ast.String(regex1)),
		ast.NewTerm(ast.String("foobar")),
	}
	err := builtinRegexMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	assertRegexCached(t, regex1)

	// Fill up the cache.
	for i := range regexCacheMaxSize - 1 {
		operands := []*ast.Term{
			ast.NewTerm(ast.String(fmt.Sprintf("foo%d.*", i))),
			ast.NewTerm(ast.String(fmt.Sprintf("foo%dbar", i))),
		}
		err := builtinRegexMatch(ctx, operands, iter)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	assertRegexCacheLen(t, regexCacheMaxSize)

	// A new regex pattern is cached and a random pattern is evicted. Any
	// pattern can be evicted by the next insert, so each one is asserted on
	// right after it is cached.
	regex2 := "bar.*"
	operands = []*ast.Term{
		ast.NewTerm(ast.String(regex2)),
		ast.NewTerm(ast.String("barbaz")),
	}
	err = builtinRegexMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	assertRegexCached(t, regex2)
	assertRegexCacheLen(t, regexCacheMaxSize)

	// Both builtins which interact with the cache should correctly evict
	// cache items.
	regex3 := "ba<[zx]>.*"
	operands = []*ast.Term{
		ast.NewTerm(ast.String(regex3)),
		ast.NewTerm(ast.String("barbaz")),
		ast.NewTerm(ast.String("<")),
		ast.NewTerm(ast.String(">")),
	}
	err = builtinRegexMatchTemplate(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Templates are cached under the pattern generated from them.
	gen, err := generateRegexTemplate(regex3, '<', '>')
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	assertRegexCached(t, gen)
	assertRegexCacheLen(t, regexCacheMaxSize)
}

func assertRegexCached(t *testing.T, pat string) {
	t.Helper()

	regexpCacheLock.RLock()
	_, ok := regexpCache[pat]
	regexpCacheLock.RUnlock()

	if !ok {
		t.Fatalf("Expected regex to be cached: %v", pat)
	}
}

func assertRegexCacheLen(t *testing.T, exp int) {
	t.Helper()

	regexpCacheLock.RLock()
	n := len(regexpCache)
	regexpCacheLock.RUnlock()

	if n != exp {
		t.Fatalf("Expected cache to hold %d entries, was %d", exp, n)
	}
}

func TestRegexBuiltinInterQueryValueCache(t *testing.T) {
	t.Parallel()

	ip := []byte(`{"inter_query_builtin_value_cache": {"max_num_entries": "10"},}`)
	config, _ := cache.ParseCachingConfig(ip)
	interQueryValueCache := cache.NewInterQueryValueCache(t.Context(), config)

	ctx := BuiltinContext{InterQueryBuiltinValueCache: interQueryValueCache}
	iter := func(*ast.Term) error { return nil }

	// A novel regex pattern is cached.
	regex1 := "foo.*"
	operands := []*ast.Term{
		ast.NewTerm(ast.String(regex1)),
		ast.NewTerm(ast.String("foobar")),
	}
	err := builtinRegexMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if _, ok := ctx.InterQueryBuiltinValueCache.Get(ast.StringTerm(regex1).Value); !ok {
		t.Fatalf("Expected regex to be cached: %v", regex1)
	}

	// Fill up the cache.
	for i := range 9 {
		operands := []*ast.Term{
			ast.NewTerm(ast.String(fmt.Sprintf("foo%d.*", i))),
			ast.NewTerm(ast.String(fmt.Sprintf("foo%dbar", i))),
		}
		err := builtinRegexMatch(ctx, operands, iter)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	// A new regex pattern is cached and a random pattern is evicted.
	regex2 := "bar.*"
	operands = []*ast.Term{
		ast.NewTerm(ast.String(regex2)),
		ast.NewTerm(ast.String("barbaz")),
	}
	err = builtinRegexMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if _, ok := ctx.InterQueryBuiltinValueCache.Get(ast.StringTerm(regex2).Value); !ok {
		t.Fatalf("Expected regex to be cached: %v", regex2)
	}
}

func TestRegexBuiltinInterQueryValueCacheTypeMismatch(t *testing.T) {
	t.Parallel()

	ip := []byte(`{"inter_query_builtin_value_cache": {"max_num_entries": "10"},}`)
	config, _ := cache.ParseCachingConfig(ip)
	interQueryValueCache := cache.NewInterQueryValueCache(t.Context(), config)

	ctx := BuiltinContext{InterQueryBuiltinValueCache: interQueryValueCache}
	iter := func(*ast.Term) error { return nil }

	key := "foo.*"

	ctx.InterQueryBuiltinValueCache.Insert(ast.StringTerm(key).Value, "bar")

	operands := []*ast.Term{
		ast.NewTerm(ast.String(key)),
		ast.NewTerm(ast.String("foobar")),
	}
	err := builtinRegexMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// verify the original cache entry is unchanged
	value, ok := ctx.InterQueryBuiltinValueCache.Get(ast.StringTerm(key).Value)
	if !ok {
		t.Fatal("Expected key \"foo.*\" in cache")
	}

	actual, ok := value.(string)
	if !ok {
		t.Fatal("Expected string value")
	}

	if actual != "bar" {
		t.Fatalf("Expected value \"bar\" but got %v", actual)
	}
}
