// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestRegexBuiltinCache(t *testing.T) {
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

	if _, ok := regexpCache[regex1]; !ok {
		t.Fatalf("Expected regex to be cached: %v", regex1)
	}

	// Fill up the cache.
	for i := 0; i < regexCacheMaxSize-1; i++ {
		operands := []*ast.Term{
			ast.NewTerm(ast.String(fmt.Sprintf("foo%d.*", i))),
			ast.NewTerm(ast.String(fmt.Sprintf("foo%dbar", i))),
		}
		err := builtinRegexMatch(ctx, operands, iter)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	}

	if len(regexpCache) != regexCacheMaxSize {
		t.Fatalf("Expected cache to be full")
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

	if len(regexpCache) != regexCacheMaxSize {
		t.Fatalf("Expected cache be capped at %d, was %d", regexCacheMaxSize, len(regexpCache))
	}

	if _, ok := regexpCache[regex2]; !ok {
		t.Fatalf("Expected regex to be cached: %v", regex2)
	}
}
