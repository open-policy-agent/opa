// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// BenchmarkLookupGatedIndex is every rule testing a few shared refs and one of
// its own, the shape that makes the trie fan out under the shared levels.
func BenchmarkLookupGatedIndex(b *testing.B) {
	for _, tc := range []struct{ n, gates, base int }{{100, 4, 3}, {1000, 2, 3}, {1000, 4, 3}, {1000, 4, 2}} {
		mod, input := orderingBenchModule(tc.n, tc.gates, tc.base)
		idx := newBaseDocEqIndex(func(Ref) bool { return false })
		if !idx.Build(mod.Rules) {
			b.Fatal("expected index build to succeed")
		}
		b.Run(fmt.Sprintf("rules=%d/gates=%d/base=%d", tc.n, tc.gates, tc.base), func(b *testing.B) {
			r := &countingResolver{input: input}
			res, err := idx.Lookup(r)
			if err != nil {
				b.Fatal(err)
			}
			resolver := testResolver{input: input}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := idx.Lookup(resolver); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(r.calls), "resolves/op")
			b.ReportMetric(float64(len(res.Rules)), "rules/op")
		})
	}
}

// BenchmarkBuildIndexShapes builds the index for several shapes, and reports how
// much of the heap the built index keeps.
func BenchmarkBuildIndexShapes(b *testing.B) {
	compiled := func(policy string) []*Rule {
		return MustCompileModules(map[string]string{"test.rego": policy}).Modules["test.rego"].Rules
	}
	gated, _ := orderingBenchModule(1000, 4, 3)
	alternating, _ := alternatingPolicy(1000)
	shapes := []struct {
		note  string
		rules []*Rule
	}{
		{"eq/10000", eqIndexRules(10000)},
		{"gated/1000", gated.Rules},
		{"alternating/1000", compiled(alternating)},
		{"membership/10000", compiled(membershipPolicy(10000))},
		{"startswith/10x1000", startsWithRules(prefixGrid{rules: 10, perRule: 1000})},
		{"any_prefix_match/10x1000", anyPrefixMatchRules(prefixGrid{rules: 10, perRule: 1000})},
	}
	for _, tc := range shapes {
		b.Run(tc.note, func(b *testing.B) {
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			index := newBaseDocEqIndex(isVirtual)
			index.Build(tc.rules)
			runtime.GC()
			runtime.ReadMemStats(&after)
			runtime.KeepAlive(index)

			b.ReportAllocs()
			for b.Loop() {
				newBaseDocEqIndex(isVirtual).Build(tc.rules)
			}
			b.ReportMetric(float64(after.HeapAlloc-before.HeapAlloc), "retained-B")
		})
	}
}

// orderingBenchModule builds n rules, each testing d shared "gate" refs
// (input.h0..h{d-1}, the digits of the rule's number in base) and one ref of
// its own (input.cN). The input sets every ref to 0, so only the rules whose
// gates are all 0 match, one in base^d, and a lookup has to rule out the rest
// before it gets to their own refs.
func orderingBenchModule(n, d, base int) (*Module, *Term) {
	var sb strings.Builder
	sb.WriteString("package test\n\n")
	var q strings.Builder
	q.WriteString("{")
	for i := range n {
		var body strings.Builder
		for j := range d {
			if j > 0 {
				body.WriteString("; ")
			}
			// Spread gate values so only rules with i % base^d == 0 match the
			// all-zero query -- i.e. the gates actually prune.
			fmt.Fprintf(&body, "input.h%d = %d", j, (i/pow(base, j))%base)
		}
		fmt.Fprintf(&body, "; input.c%d = 0", i)
		fmt.Fprintf(&sb, "p if { %s }\n", body.String())

		if i > 0 {
			q.WriteString(", ")
		}
		fmt.Fprintf(&q, `"c%d": 0`, i)
	}
	for j := range d {
		fmt.Fprintf(&q, `, "h%d": 0`, j)
	}
	q.WriteString("}")
	return MustParseModule(sb.String()), MustParseTerm(q.String())
}

func pow(b, e int) int {
	r := 1
	for range e {
		r *= b
	}
	return r
}

type countingResolver struct {
	input *Term
	calls int
}

func (r *countingResolver) Resolve(ref Ref) (Value, error) {
	r.calls++
	if ref.HasPrefix(InputRootRef) {
		v, err := r.input.Value.Find(ref[1:])
		if err != nil {
			return nil, nil
		}
		return v, nil
	}
	panic("illegal value")
}
