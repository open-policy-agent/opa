// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
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

	if _, ok := globCache[globCacheKey(glob1, nil)]; !ok {
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
		t.Fatal("Expected cache to be full")
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

	if _, ok := globCache[globCacheKey(glob2, nil)]; !ok {
		t.Fatalf("Expected glob to be cached: %v", glob2)
	}
}

// TestGlobMatchCacheKeyCollision pins that two different (pattern, delimiters)
// inputs do not share a cache entry. Before the key carried the pattern length,
// glob.match("qux", ["-","x"], ...) and glob.match("qux-", ["x"], ...) produced
// the same key ("qux--x"), so the second call reused the first call's compiled
// pattern and returned the wrong result.
func TestGlobMatchCacheKeyCollision(t *testing.T) {
	t.Parallel()

	call := func(pattern string, delimiters []*ast.Term, match string) bool {
		var got bool
		iter := func(tm *ast.Term) error {
			got = bool(tm.Value.(ast.Boolean))
			return nil
		}
		operands := []*ast.Term{
			ast.NewTerm(ast.String(pattern)),
			ast.ArrayTerm(delimiters...),
			ast.NewTerm(ast.String(match)),
		}
		if err := builtinGlobMatch(BuiltinContext{}, operands, iter); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		return got
	}

	// Pattern "qux" with delimiters '-' and 'x' matches the literal "qux".
	if got := call("qux", []*ast.Term{ast.StringTerm("-"), ast.StringTerm("x")}, "qux"); !got {
		t.Fatalf(`glob.match("qux", ["-","x"], "qux") = false, want true`)
	}

	// Pattern "qux-" with delimiter 'x' must not match "qux": the trailing '-'
	// is a required literal. This previously collided with the entry above.
	if got := call("qux-", []*ast.Term{ast.StringTerm("x")}, "qux"); got {
		t.Fatalf(`glob.match("qux-", ["x"], "qux") = true, want false (cache key collision)`)
	}
}

// TestGlobMatchCompileError pins the patterns glob.match rejects, and the
// message it rejects them with: both are visible to policy authors, and both
// have changed with a gobwas/glob upgrade before. Note that a malformed
// pattern is not cached, so this test does not disturb the cache assertions
// above.
func TestGlobMatchCompileError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note    string
		pattern string
		wantErr string
	}{
		{
			note:    "unclosed pattern-alternatives list",
			pattern: "{a,b",
			wantErr: "glob: syntax error at 4: unclosed `{`",
		},
		{
			note:    "trailing backslash",
			pattern: `a\`,
			wantErr: "glob: syntax error at 2: trailing backslash",
		},
		{
			note:    "lone backslash",
			pattern: `\`,
			wantErr: "glob: syntax error at 1: trailing backslash",
		},
		{
			note:    "empty character-list",
			pattern: "[]",
			wantErr: "glob: syntax error at 2: could not parse range",
		},
		{
			note:    "unclosed character-list",
			pattern: "[abc",
			wantErr: "glob: syntax error at 4: unexpected end of input",
		},
		{
			note:    "reversed character-range",
			pattern: "[c-a]",
			wantErr: "glob: syntax error at 5: range hi character is less than lo",
		},
		{
			note:    "invalid UTF-8",
			pattern: "a\xffb",
			wantErr: "glob: syntax error at 1: invalid UTF-8 sequence",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			operands := []*ast.Term{
				ast.NewTerm(ast.String(tc.pattern)),
				ast.NullTerm(),
				ast.NewTerm(ast.String("abc")),
			}

			err := builtinGlobMatch(BuiltinContext{}, operands, func(*ast.Term) error { return nil })
			if err == nil {
				t.Fatalf("Expected error for pattern %q", tc.pattern)
			}

			if err.Error() != tc.wantErr {
				t.Errorf("Expected error %q but got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func TestGlobBuiltinInterQueryValueCache(t *testing.T) {
	t.Parallel()

	ip := []byte(`{"inter_query_builtin_value_cache": {"max_num_entries": "10"},}`)
	config, _ := cache.ParseCachingConfig(ip)
	interQueryValueCache := cache.NewInterQueryValueCache(t.Context(), config)

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

	if _, ok := ctx.InterQueryBuiltinValueCache.Get(ast.String(globCacheKey(glob1, nil))); !ok {
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

	if _, ok := ctx.InterQueryBuiltinValueCache.Get(ast.String(globCacheKey(glob2, nil))); !ok {
		t.Fatalf("Expected glob to be cached: %v", glob2)
	}
}

func TestGlobBuiltinInterQueryValueCacheTypeMismatch(t *testing.T) {
	t.Parallel()

	ip := []byte(`{"inter_query_builtin_value_cache": {"max_num_entries": "10"},}`)
	config, _ := cache.ParseCachingConfig(ip)
	interQueryValueCache := cache.NewInterQueryValueCache(t.Context(), config)

	ctx := BuiltinContext{InterQueryBuiltinValueCache: interQueryValueCache}
	iter := func(*ast.Term) error { return nil }

	key := "foo.*"
	cacheKey := ast.String(globCacheKey(key, nil))

	operands := []*ast.Term{
		ast.NewTerm(ast.String(key)),
		ast.NullTerm(),
		ast.NewTerm(ast.String("foo/bar")),
	}
	err := builtinGlobMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if _, ok := ctx.InterQueryBuiltinValueCache.Get(cacheKey); !ok {
		t.Fatalf("Expected glob to be cached: %v", key)
	}

	// update the cache entry
	ctx.InterQueryBuiltinValueCache.Insert(cacheKey, "bar")

	err = builtinGlobMatch(ctx, operands, iter)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// verify the cache entry is unchanged
	value, ok := ctx.InterQueryBuiltinValueCache.Get(cacheKey)
	if !ok {
		t.Fatalf("Expected key %q in cache", string(cacheKey))
	}

	actual, ok := value.(string)
	if !ok {
		t.Fatal("Expected string value")
	}

	if actual != "bar" {
		t.Fatalf("Expected value \"bar\" but got %v", actual)
	}
}
