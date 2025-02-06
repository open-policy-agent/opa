// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
)

func TestGlobBuiltinCache(t *testing.T) {
	t.Parallel()

	ctx := BuiltinContext{}
	iter := func(*ast.Term) error { return nil }

	// A novel glob pattern is cached.
	glob1 := "foo/*"
	operands := []*ast.Term{
		ast.NewTerm(ast.String(glob1)),
		ast.NullTerm(),
		ast.NewTerm(ast.String("foo/bar")),
	}
	err := builtinGlobMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// the glob id will have a trailing '-' rune.
	if _, ok := globCache[glob1+"-"]; !ok {
		t.Fatalf("Expected glob to be cached: %v", glob1)
	}

	// Fill up the cache.
	for i := range regexCacheMaxSize - 1 {
		operands := []*ast.Term{
			ast.NewTerm(ast.String(fmt.Sprintf("foo/%d/*", i))),
			ast.NullTerm(),
			ast.NewTerm(ast.String(fmt.Sprintf("foo/%d/bar", i))),
		}
		err := builtinGlobMatch(ctx, operands, iter)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	if len(globCache) != regexCacheMaxSize {
		t.Fatalf("Expected cache to be full")
	}

	// A new glob pattern is cached and a random pattern is evicted.
	glob2 := "bar/*"
	operands = []*ast.Term{
		ast.NewTerm(ast.String(glob2)),
		ast.NullTerm(),
		ast.NewTerm(ast.String("bar/baz")),
	}
	err = builtinGlobMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(globCache) != regexCacheMaxSize {
		t.Fatalf("Expected cache be capped at %d, was %d", regexCacheMaxSize, len(globCache))
	}

	if _, ok := globCache[glob2+"-"]; !ok {
		t.Fatalf("Expected glob to be cached: %v", glob2)
	}
}

func TestGlobBuiltinInterQueryValueCache(t *testing.T) {
	t.Parallel()

	ip := []byte(`{"inter_query_builtin_value_cache": {"max_num_entries": "10"},}`)
	config, _ := cache.ParseCachingConfig(ip)
	interQueryValueCache := cache.NewInterQueryValueCache(context.Background(), config)

	ctx := BuiltinContext{InterQueryBuiltinValueCache: interQueryValueCache}
	iter := func(*ast.Term) error { return nil }

	// A novel glob pattern is cached.
	glob1 := "foo/*"
	operands := []*ast.Term{
		ast.NewTerm(ast.String(glob1)),
		ast.NullTerm(),
		ast.NewTerm(ast.String("foo/bar")),
	}
	err := builtinGlobMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// the glob id will have a trailing '-' rune.
	if _, ok := ctx.InterQueryBuiltinValueCache.Get(ast.StringTerm(glob1 + "-").Value); !ok {
		t.Fatalf("Expected glob to be cached: %v", glob1)
	}

	// Fill up the cache.
	for i := range 9 {
		operands := []*ast.Term{
			ast.NewTerm(ast.String(fmt.Sprintf("foo/%d/*", i))),
			ast.NullTerm(),
			ast.NewTerm(ast.String(fmt.Sprintf("foo/%d/bar", i))),
		}
		err := builtinGlobMatch(ctx, operands, iter)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	// A new glob pattern is cached and a random pattern is evicted.
	glob2 := "bar/*"
	operands = []*ast.Term{
		ast.NewTerm(ast.String(glob2)),
		ast.NullTerm(),
		ast.NewTerm(ast.String("bar/baz")),
	}
	err = builtinGlobMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if _, ok := ctx.InterQueryBuiltinValueCache.Get(ast.StringTerm(glob2 + "-").Value); !ok {
		t.Fatalf("Expected glob to be cached: %v", glob2)
	}
}

func TestGlobBuiltinInterQueryValueCacheTypeMismatch(t *testing.T) {
	t.Parallel()

	ip := []byte(`{"inter_query_builtin_value_cache": {"max_num_entries": "10"},}`)
	config, _ := cache.ParseCachingConfig(ip)
	interQueryValueCache := cache.NewInterQueryValueCache(context.Background(), config)

	ctx := BuiltinContext{InterQueryBuiltinValueCache: interQueryValueCache}
	iter := func(*ast.Term) error { return nil }

	key := "foo.*"

	operands := []*ast.Term{
		ast.NewTerm(ast.String(key)),
		ast.NullTerm(),
		ast.NewTerm(ast.String("foo/bar")),
	}
	err := builtinGlobMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// the glob id will have a trailing '-' rune.
	if _, ok := ctx.InterQueryBuiltinValueCache.Get(ast.StringTerm(key + "-").Value); !ok {
		t.Fatalf("Expected glob to be cached: %v", key)
	}

	// update the cache entry
	ctx.InterQueryBuiltinValueCache.Insert(ast.StringTerm(key+"-").Value, "bar")

	err = builtinGlobMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// verify the cache entry is unchanged
	value, ok := ctx.InterQueryBuiltinValueCache.Get(ast.StringTerm(key + "-").Value)
	if !ok {
		t.Fatal("Expected key \"foo.*-\" in cache")
	}

	actual, ok := value.(string)
	if !ok {
		t.Fatal("Expected string value")
	}

	if actual != "bar" {
		t.Fatalf("Expected value \"bar\" but got %v", actual)
	}
}
