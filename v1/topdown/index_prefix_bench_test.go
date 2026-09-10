// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

// prefixMatchShape is a way of writing "the path starts with one of these".
type prefixMatchShape struct {
	note string
	// rule renders one rule matching the given prefixes.
	rule func(sb *strings.Builder, prefixes []string)
}

var prefixMatchShapes = []prefixMatchShape{
	{
		// Indexable: the base strings are known at compile time, so they go
		// into the rule index and the rule is only evaluated for a path that
		// starts with one of them.
		note: "any_prefix_match",
		rule: func(sb *strings.Builder, prefixes []string) {
			sb.WriteString("allow if strings.any_prefix_match(input.path, [")
			writePrefixList(sb, prefixes)
			sb.WriteString("])\n\n")
		},
	},
	{
		// The same thing spelled out as iteration, which is what
		// strings.any_prefix_match replaces. The base is a variable, so there
		// is nothing for the index to record and every rule runs its whole
		// loop on every query.
		note: "iteration",
		rule: func(sb *strings.Builder, prefixes []string) {
			sb.WriteString("allow if {\n\tsome prefix in [")
			writePrefixList(sb, prefixes)
			sb.WriteString("]\n\tstartswith(input.path, prefix)\n}\n\n")
		},
	},
	{
		// One rule per prefix. Indexable like any_prefix_match, but it costs a
		// rule -- and a trie leaf -- per prefix rather than per rule, which is
		// what makes it worth comparing the two.
		note: "startswith",
		rule: func(sb *strings.Builder, prefixes []string) {
			for _, prefix := range prefixes {
				fmt.Fprintf(sb, "allow if startswith(input.path, %q)\n\n", prefix)
			}
		},
	},
}

// BenchmarkRuleIndexPrefixMatch measures what the rule index does for policies
// that admit a request by the prefix of its path -- the shape of a routing or
// authorization policy with a list of allowed path prefixes per rule.
//
// The input path matches nothing, which is the case the index is for: without
// it every rule has to be evaluated to find that out, and with it the trie
// walks the path once and returns nothing. Rules and prefixes-per-rule vary
// independently, up to 250 rules of 10000 prefixes each -- 2.5M prefixes.
//
// At the top of that grid:
//
//	                      indexing=true                indexing=false
//	any_prefix_match        597 ns      1096 B/op     63295856 ns   41216594 B/op
//	startswith              613 ns      1096 B/op   1592790750 ns 1558489144 B/op
//	iteration         850229396 ns 504413952 B/op    846111188 ns  504412388 B/op
//
// The indexed rows are flat across both dimensions: the whole ruleset costs one
// walk of the path. 597 ns at 2.5M prefixes is the same 600-700 ns the index
// costs at 100. `iteration` is the same policy written as a loop over the
// prefixes, which is what strings.any_prefix_match exists to replace -- there is
// nothing there for the index to record, so it stays at the cost of evaluating
// every rule.
//
// The top of the grid is a large policy to build: 2.5M prefixes is 74MB of
// source for any_prefix_match and iteration, and 156MB and ~8GB of heap for
// startswith, which spends a rule on each prefix. That is setup rather than
// measured time, and the two indexing settings of a grid point share one
// compile (see compilePrefixPolicy).
func BenchmarkRuleIndexPrefixMatch(b *testing.B) {
	for _, shape := range prefixMatchShapes {
		b.Run(shape.note, func(b *testing.B) {
			for _, rules := range []int{1, 10, 50, 250} {
				for _, perRule := range []int{100, 1000, 10000} {
					b.Run(fmt.Sprintf("rules=%d/prefixes=%d", rules, perRule), func(b *testing.B) {
						for _, indexing := range []bool{true, false} {
							b.Run(fmt.Sprintf("indexing=%v", indexing), func(b *testing.B) {
								benchmarkPrefixMatch(b, shape, rules, perRule, indexing, "/no/such/path/at/all", false)
							})
						}
					})
				}
			}
		})
	}
}

// BenchmarkRuleIndexPrefixMatchHit is the same policy queried with a path that
// one rule admits. Complete-rule evaluation stops at the first rule that holds,
// so the matching prefix is put in the last rule to keep the two shapes
// comparable.
//
// At 250 rules of 10000 prefixes each -- 2.5M prefixes:
//
//	                      indexing=true                indexing=false
//	any_prefix_match     159350 ns    167258 B/op     80579479 ns   41301049 B/op
//	startswith             1956 ns      2672 B/op   1563445458 ns 1558490072 B/op
//	iteration         811024271 ns 504414304 B/op    806562979 ns  504413028 B/op
//
// This is where the two indexable shapes come apart, and it is the reason to
// vary prefixes per rule rather than in total. The index narrows 250 rules to
// the one that can hold either way, but that rule still has to be evaluated:
// for startswith it is a single comparison, while for any_prefix_match the
// builtin scans the rule's own 10000-element array. So any_prefix_match pays
// per prefix in the rule it selects, not per prefix in the policy -- 159 us
// here against 597 ns for the miss, which returns no rule to evaluate. Both
// still beat the unindexed policy by two to five orders of magnitude.
func BenchmarkRuleIndexPrefixMatchHit(b *testing.B) {
	const (
		rules   = 250
		perRule = 10000
	)

	for _, shape := range prefixMatchShapes {
		b.Run(shape.note, func(b *testing.B) {
			for _, indexing := range []bool{true, false} {
				b.Run(fmt.Sprintf("indexing=%v", indexing), func(b *testing.B) {
					benchmarkPrefixMatch(b, shape, rules, perRule, indexing,
						benchPrefix(rules*perRule-1)+"/some/request", true)
				})
			}
		})
	}
}

func benchmarkPrefixMatch(b *testing.B, shape prefixMatchShape, rules, perRule int, indexing bool, path string, defined bool) {
	b.Helper()

	ctx := b.Context()

	compiler := compilePrefixPolicy(b, shape, rules, perRule)

	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	input := ast.NewTerm(ast.MustInterfaceToValue(map[string]any{"path": path}))

	query := NewQuery(ast.MustParseBody("data.bench.allow = x")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(input).
		WithIndexing(indexing)

	// Whether the query is defined is a property of the policy and the path,
	// not of indexing, so check it once up front rather than in the loop.
	rs, err := query.Run(ctx)
	if err != nil {
		b.Fatalf("unexpected topdown query error: %v", err)
	}
	if (len(rs) == 1) != defined {
		b.Fatalf("expected defined=%v for %q, got %v", defined, path, rs)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := query.Run(ctx); err != nil {
			b.Fatalf("unexpected topdown query error: %v", err)
		}
	}
}

// prefixPolicyCache holds the most recently compiled policy, so that the
// indexing=true and indexing=false runs of one grid point do not each pay for
// it. Only one is kept: at the top of the grid a compiled policy is gigabytes,
// and the previous one has to be released before the next is built.
var prefixPolicyCache struct {
	key      string
	compiler *ast.Compiler
}

func compilePrefixPolicy(b *testing.B, shape prefixMatchShape, rules, perRule int) *ast.Compiler {
	b.Helper()

	key := fmt.Sprintf("%s/%d/%d", shape.note, rules, perRule)
	if prefixPolicyCache.key == key {
		return prefixPolicyCache.compiler
	}
	prefixPolicyCache.key, prefixPolicyCache.compiler = "", nil

	compiler := ast.NewCompiler()
	mods := map[string]*ast.Module{"policy": ast.MustParseModule(prefixMatchPolicy(shape, rules, perRule))}
	if compiler.Compile(mods); compiler.Failed() {
		b.Fatalf("unexpected compiler error: %v", compiler.Errors)
	}

	prefixPolicyCache.key, prefixPolicyCache.compiler = key, compiler

	return compiler
}

// prefixMatchPolicy renders a policy of the given shape holding `rules` rules
// of `perRule` prefixes each, so rules*perRule prefixes in total. The prefixes
// are distinct across the whole policy.
func prefixMatchPolicy(shape prefixMatchShape, rules, perRule int) string {
	var sb strings.Builder
	sb.WriteString("package bench\n\nimport rego.v1\n\n")

	batch := make([]string, perRule)
	next := 0

	for range rules {
		for i := range batch {
			batch[i] = benchPrefix(next)
			next++
		}

		shape.rule(&sb, batch)
	}

	return sb.String()
}

func writePrefixList(sb *strings.Builder, prefixes []string) {
	for i, prefix := range prefixes {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(sb, "%q", prefix)
	}
}

// benchPrefix returns the i'th path prefix. They share a stem, the way the
// paths of one service do, so the index cannot tell them apart on the first
// byte and has to walk. The counter is zero-padded to a fixed width so that no
// prefix in the set is a prefix of another, whatever the size of the set.
func benchPrefix(i int) string {
	return fmt.Sprintf("/service/v1/tenant/%07d", i)
}

// siblingShapes write the rule's second, single-valued constraint either side
// of its prefix set. Which comes first decides what a candidate costs when the
// index hands topdown a rule that cannot hold: the method comparison fails on
// the first expression, the prefix scan walks the rule's whole base first.
var siblingShapes = []struct {
	note        string
	prefixFirst bool
}{
	{"method-first", false},
	{"prefix-first", true},
}

// BenchmarkRuleIndexPrefixMatchSibling is the shape the rest of this file does
// not have: a rule that constrains something besides its prefix set. Every rule
// here admits a request by (method, path prefix), the way an authorization
// policy does, and the query presents a path one rule admits under a method
// none of them do.
//
// The prefix set gives the rule several values for input.path, which is where
// insertPath stops building its path (see refindices.alternated) -- so whether
// input.method is indexed at all depends on the two being ordered the right way
// round. Where BenchmarkRuleIndexPrefixMatch measures the trie walk, this
// measures what that ordering is worth: the rule topdown does not evaluate.
//
//	                 ranked last            by frequency
//	method-first     527 ns  1097 B     1152 ns  2235 B
//	prefix-first     532 ns  1097 B     2345 ns  4936 B
//
// Both shapes hold the same policy and return the same answer. That they cost
// the same is the point: with input.method indexed the rule is not a candidate,
// so it no longer matters which of its two constraints the author wrote first.
// 1097 B and 20 allocs is what a query that reaches no rule at all costs -- the
// same as the indexing=true rows of BenchmarkRuleIndexPrefixMatch, whose rules
// have nothing besides their prefix set to be excluded on.
//
// Ranked by frequency the prefix reference comes first, because insertPrefixes
// records one value per base string, so the rule is reached by its prefixes
// alone and input.method never joins its path. What that costs is then down to
// which constraint topdown evaluates first: the method comparison fails
// immediately, the prefix scan walks all 100 base strings before it does.
func BenchmarkRuleIndexPrefixMatchSibling(b *testing.B) {
	const (
		rules   = 250
		perRule = 100
	)

	for _, shape := range siblingShapes {
		b.Run(shape.note, func(b *testing.B) {
			benchmarkPrefixMatchSibling(b, rules, perRule, shape.prefixFirst)
		})
	}
}

func benchmarkPrefixMatchSibling(b *testing.B, rules, perRule int, prefixFirst bool) {
	b.Helper()

	ctx := b.Context()

	compiler := ast.NewCompiler()
	mods := map[string]*ast.Module{
		"policy": ast.MustParseModule(siblingPolicy(rules, perRule, prefixFirst)),
	}
	if compiler.Compile(mods); compiler.Failed() {
		b.Fatalf("unexpected compiler error: %v", compiler.Errors)
	}

	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)

	// A path the fourth rule admits, under a method no rule admits.
	input := ast.NewTerm(ast.MustInterfaceToValue(map[string]any{
		"path":   benchPrefix(3*perRule+7) + "/some/request",
		"method": "NO_SUCH_METHOD",
	}))

	query := NewQuery(ast.MustParseBody("data.bench.allow = x")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(input).
		WithIndexing(true)

	rs, err := query.Run(ctx)
	if err != nil {
		b.Fatalf("unexpected topdown query error: %v", err)
	}
	if len(rs) != 0 {
		b.Fatalf("expected the query to be undefined, got %v", rs)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		if _, err := query.Run(ctx); err != nil {
			b.Fatalf("unexpected topdown query error: %v", err)
		}
	}
}

// siblingPolicy renders a rule per (method, prefix set). The prefixes are
// distinct across the whole policy, as in prefixMatchPolicy, and each rule
// takes a method of its own so that a query naming none of them is a miss on
// every rule.
func siblingPolicy(rules, perRule int, prefixFirst bool) string {
	var sb strings.Builder
	sb.WriteString("package bench\n\nimport rego.v1\n\n")

	batch := make([]string, perRule)
	next := 0

	for r := range rules {
		for i := range batch {
			batch[i] = benchPrefix(next)
			next++
		}

		var prefix strings.Builder
		prefix.WriteString("strings.any_prefix_match(input.path, [")
		writePrefixList(&prefix, batch)
		prefix.WriteString("])")

		method := fmt.Sprintf("input.method == %q", fmt.Sprintf("METHOD_%d", r))

		first, second := method, prefix.String()
		if prefixFirst {
			first, second = prefix.String(), method
		}

		fmt.Fprintf(&sb, "allow if {\n\t%s\n\t%s\n}\n\n", first, second)
	}

	return sb.String()
}
