// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strings"
	"testing"
)

// prefixGrid is the shape of the ruleset the prefix benchmarks build: `rules`
// rules holding `perRule` prefixes each, so rules*perRule prefixes in total.
// The two vary independently because the index scales differently in each --
// see BenchmarkBuildPrefixIndex.
type prefixGrid struct {
	rules   int
	perRule int
}

func (g prefixGrid) String() string {
	return fmt.Sprintf("rules=%d/prefixes=%d", g.rules, g.perRule)
}

func (g prefixGrid) total() int { return g.rules * g.perRule }

var prefixGrids = []prefixGrid{
	{1, 100}, {1, 1000}, {1, 10000},
	{10, 100}, {10, 1000}, {10, 10000},
	{50, 100}, {50, 1000}, {50, 10000},
	{250, 100}, {250, 1000}, {250, 10000},
}

// BenchmarkBuildPrefixIndex is the cost of putting rules*perRule prefixes into
// the index, once as rules matching perRule prefixes each
// (strings.any_prefix_match) and once as one rule per prefix (startswith).
// Both are linear in the total number of prefixes and indifferent to how those
// prefixes are divided into rules -- nothing here is quadratic in either
// dimension.
//
//	                          any_prefix_match              startswith
//	rules=1/prefixes=10000       1907915 ns    4.7 MB     5029604 ns    6.1 MB
//	rules=10/prefixes=1000       1850371 ns    4.7 MB     5052827 ns    6.1 MB
//	rules=250/prefixes=10000   480645292 ns   1186 MB  2247822750 ns   1531 MB
//
// The first two rows hold 10000 prefixes each, split two ways, and cost the
// same; the third holds 250 times as many and costs 250 times as much.
//
// any_prefix_match used to be quadratic in perRule -- 302ms and 4.6MB for a
// single rule of 10000 prefixes -- because refindices.insert rescans the rule's
// indices on every call. See insertPrefixes.
//
// The B/op column is close to what the trie costs to hold, since nothing it
// allocates is released. At the top of the grid the startswith ruleset is 2.5M
// rules and takes gigabytes; that ruleset is built once, outside the timer.
func BenchmarkBuildPrefixIndex(b *testing.B) {
	for _, tc := range prefixShapes {
		b.Run(tc.note, func(b *testing.B) {
			for _, g := range prefixGrids {
				b.Run(g.String(), func(b *testing.B) {
					rules := tc.rules(g)
					b.ReportAllocs()
					b.ResetTimer()

					for b.Loop() {
						index := newBaseDocEqIndex(isVirtual)
						if !index.Build(rules) {
							b.Fatal("failed to build index")
						}
					}
				})
			}
		})
	}
}

// BenchmarkLookupPrefixIndex is the point of the radix trie: a lookup walks the
// value once, so its cost follows the length of the value and not the number of
// prefixes indexed -- flat from 100 prefixes to 2.5M, in either dimension.
//
//	                          any_prefix_match          startswith
//	rules=1/prefixes=100         172.0 ns  1 alloc    171.3 ns  1 alloc
//	rules=250/prefixes=10000     223.5 ns  1 alloc    222.7 ns  1 alloc
func BenchmarkLookupPrefixIndex(b *testing.B) {
	for _, tc := range prefixShapes {
		b.Run(tc.note, func(b *testing.B) {
			for _, g := range prefixGrids {
				b.Run(g.String(), func(b *testing.B) {
					index := newBaseDocEqIndex(isVirtual)
					if !index.Build(tc.rules(g)) {
						b.Fatal("failed to build index")
					}

					// A value under the last prefix, so the walk goes the whole
					// way down rather than failing at the first byte.
					input := inputResolver{
						input: MustParseTerm(fmt.Sprintf(`{"path": %q}`, prefixAt(g.total()-1)+"/and/some/more")).Value,
					}

					b.ReportAllocs()
					b.ResetTimer()

					for b.Loop() {
						res, err := index.Lookup(input)
						if err != nil {
							b.Fatal(err)
						} else if len(res.Rules) != 1 {
							b.Fatalf("expected 1 rule, got %d", len(res.Rules))
						}
						IndexResultPool.Put(res)
					}
				})
			}
		})
	}
}

// BenchmarkLookupPrefixIndexMiss is the other half of it: a value sharing no
// first byte with any prefix costs one binary search, whatever the ruleset is
// -- 55 ns flat from 100 prefixes to 2.5M.
func BenchmarkLookupPrefixIndexMiss(b *testing.B) {
	for _, g := range prefixGrids {
		b.Run(g.String(), func(b *testing.B) {
			index := newBaseDocEqIndex(isVirtual)
			if !index.Build(anyPrefixMatchRules(g)) {
				b.Fatal("failed to build index")
			}
			input := inputResolver{input: MustParseTerm(`{"path": "no/prefix/starts/like/this"}`).Value}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				res, err := index.Lookup(input)
				if err != nil {
					b.Fatal(err)
				} else if len(res.Rules) != 0 {
					b.Fatalf("expected no rules, got %d", len(res.Rules))
				}
				IndexResultPool.Put(res)
			}
		})
	}
}

// BenchmarkLookupPrefixIndexDepth separates the two things a lookup could scale
// with. The ruleset is held at 250 rules of 10000 prefixes each while the value
// grows past the longest prefix. The walk stops where the trie runs out of
// edges, so the tail of the value costs nothing: 222 ns for a 16-byte tail and
// 222 ns for a 1024-byte one.
func BenchmarkLookupPrefixIndexDepth(b *testing.B) {
	g := prefixGrid{rules: 250, perRule: 10000}

	index := newBaseDocEqIndex(isVirtual)
	if !index.Build(anyPrefixMatchRules(g)) {
		b.Fatal("failed to build index")
	}

	for _, length := range []int{16, 64, 256, 1024} {
		b.Run(fmt.Sprintf("tail=%d", length), func(b *testing.B) {
			path := prefixAt(g.total()-1) + strings.Repeat("x", length)
			input := inputResolver{input: MustParseTerm(fmt.Sprintf(`{"path": %q}`, path)).Value}

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				res, err := index.Lookup(input)
				if err != nil {
					b.Fatal(err)
				} else if len(res.Rules) != 1 {
					b.Fatalf("expected 1 rule, got %d", len(res.Rules))
				}
				IndexResultPool.Put(res)
			}
		})
	}
}

var prefixShapes = []struct {
	note  string
	rules func(prefixGrid) []*Rule
}{
	{note: "any_prefix_match", rules: anyPrefixMatchRules},
	{note: "startswith", rules: startsWithRules},
}

// prefixAt returns the i'th prefix of the benchmark set. They share a long stem
// so that the trie has edges to split and walk rather than one-byte edges off
// the root. The counter is zero-padded to a fixed width so that no prefix in
// the set is a prefix of another, whatever the size of the set.
func prefixAt(i int) string {
	return fmt.Sprintf("/service/v1/tenant/%07d", i)
}

// anyPrefixMatchRules returns g.rules rules matching g.perRule prefixes each.
func anyPrefixMatchRules(g prefixGrid) []*Rule {
	var sb strings.Builder
	sb.WriteString("package p\n\n")

	next := 0
	for range g.rules {
		sb.WriteString("allow if strings.any_prefix_match(__local0__, [")
		for i := range g.perRule {
			if i > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%q", prefixAt(next))
			next++
		}
		sb.WriteString("])\n")
	}

	return prefixRules(sb.String())
}

// startsWithRules returns the same prefixes as one rule each.
func startsWithRules(g prefixGrid) []*Rule {
	var sb strings.Builder
	sb.WriteString("package p\n\n")
	for i := range g.total() {
		fmt.Fprintf(&sb, "allow if startswith(__local0__, %q)\n", prefixAt(i))
	}

	return prefixRules(sb.String())
}

// prefixRules parses src and prepends the assignment the compiler would emit
// for the hoisted call operand, so the indexer sees the shape it sees in
// practice without paying for a full compile in the benchmark.
func prefixRules(src string) []*Rule {
	rules := MustParseModule(src).Rules
	assign := MustParseExpr("__local0__ = input.path")

	for _, rule := range rules {
		rule.Body = append(Body{assign}, rule.Body...)
	}

	return rules
}
