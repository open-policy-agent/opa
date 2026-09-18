// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// BenchmarkBuildMembershipIndex is the cost of putting `input.x in [k
// literals]` into the index. Each element is a value the rule may reach
// input.x by, and refindices.insert rescans the rule's indices on every one, so
// recording k of them used to be O(k^2):
//
//	         rescanning    recorded together
//	  100        121 us               20 us
//	 1000       11.8 ms             0.22 ms
//	10000       1128 ms             2.69 ms
//
// A decade of k cost 97x rescanning and costs 11x now. strings.any_prefix_match
// had this fixed for its own base collection when it landed; insertAll is that
// fix, shared.
func BenchmarkBuildMembershipIndex(b *testing.B) {
	for _, k := range []int{100, 1000, 10000} {
		b.Run(strconv.Itoa(k), func(b *testing.B) {
			c := MustCompileModules(map[string]string{"test.rego": membershipPolicy(k)})
			rules := c.Modules["test.rego"].Rules

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				index := newBaseDocEqIndex(func(Ref) bool { return false })
				if !index.Build(rules) {
					b.Fatal("expected index build to succeed")
				}
			}
		})
	}
}

// membershipPolicy is one rule reaching input.x by k values, with a second
// constraint below so that the rule has a path to lose if the k values are
// mishandled (see refindices.alternated).
func membershipPolicy(k int) string {
	var sb strings.Builder

	sb.WriteString("package test\n\np if {\n\tinput.x in [")
	for i := range k {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%d", i)
	}
	sb.WriteString("]\n\tinput.y == 1\n}\n")

	return sb.String()
}

// alternatingPolicy is n rules, each reaching two references by several values:
// a subject that every rule's collection holds, and an action that only one
// does. The query names a subject and an action, so exactly one rule can hold.
//
// insertPath used to stop a rule's path at the first such reference, so the
// second was not indexed and every rule stayed a candidate. Ranking those
// references last cannot help here -- both are ranked last, and one of them is
// still first of the two.
func alternatingPolicy(n int) (string, *Term) {
	var sb strings.Builder
	sb.WriteString("package test\n\n")

	for i := range n {
		fmt.Fprintf(&sb, "p if {\n\tinput.subject in [\"alice\", \"u%d\"]\n\tinput.action in [\"act%d\", \"x%d\"]\n}\n", i, i, i)
	}

	return sb.String(), MustParseTerm(`{"subject": "alice", "action": "act0"}`)
}

// BenchmarkLookupAlternatingIndex is the cost of a lookup against those rules.
// What it measures is how many of them the traversal has to report: with only
// the first reference indexed, the subject admits all n and the caller
// evaluates them; with both, the action leaves one.
func BenchmarkLookupAlternatingIndex(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			policy, input := alternatingPolicy(n)
			c := MustCompileModules(map[string]string{"test.rego": policy})

			index := newBaseDocEqIndex(func(Ref) bool { return false })
			if !index.Build(c.Modules["test.rego"].Rules) {
				b.Fatal("expected index build to succeed")
			}
			resolver := testResolver{input: input}

			res, err := index.Lookup(resolver)
			if err != nil {
				b.Fatal(err)
			}
			candidates := float64(len(res.Rules))

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if _, err := index.Lookup(resolver); err != nil {
					b.Fatal(err)
				}
			}

			// After ResetTimer, which drops user metrics reported before it.
			b.ReportMetric(candidates, "candidates")
		})
	}
}

// BenchmarkBuildAlternatingIndex is what indexing the second reference costs to
// build: one node per rule that the alternatives of the first converge on.
func BenchmarkBuildAlternatingIndex(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			policy, _ := alternatingPolicy(n)
			c := MustCompileModules(map[string]string{"test.rego": policy})
			rules := c.Modules["test.rego"].Rules

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				index := newBaseDocEqIndex(func(Ref) bool { return false })
				if !index.Build(rules) {
					b.Fatal("expected index build to succeed")
				}
			}
		})
	}
}
