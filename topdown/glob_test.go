// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestGlobBuiltinCache(t *testing.T) {
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
	if _, ok := globCache[fmt.Sprintf("%s-", glob1)]; !ok {
		t.Fatalf("Expected glob to be cached: %v", glob1)
	}

	// Fill up the cache.
	for i := 0; i < regexCacheMaxSize-1; i++ {
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

	if _, ok := globCache[fmt.Sprintf("%s-", glob2)]; !ok {
		t.Fatalf("Expected glob to be cached: %v", glob2)
	}
}
