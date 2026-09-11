// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"testing"
)

// unorderedRefs returns the indexed refs in an arbitrary order -- i.e. what
// building the trie would do if it simply didn't sort by frequency. The real
// unsorted source is frequency.Keys(), whose order is Go-map-randomized; to
// keep the benchmark and test deterministic we canonicalize first, then apply
// a fixed-seed shuffle. This is the honest "no optimization" baseline: not a
// deliberately-worst ordering, just one that ignores frequency.
func unorderedRefs(seed int64) func(*refindices) []Ref {
	return func(i *refindices) []Ref {
		refs := i.frequency.Keys()
		slices.SortFunc(refs, func(a, b Ref) int { return a.Compare(b) })
		rand.New(rand.NewSource(seed)).Shuffle(len(refs), func(a, b int) {
			refs[a], refs[b] = refs[b], refs[a]
		})
		return refs
	}
}

// buildIndexWithOrder mirrors baseDocEqIndex.Build exactly, except the ref
// insertion order is supplied by orderFn instead of being read from
// refindices.Sorted() directly -- letting us compare the real (descending)
// ordering against alternatives using the exact same trie construction and
// traversal code.
func buildIndexWithOrder(rules []*Rule, orderFn func(*refindices) []Ref) *baseDocEqIndex {
	idx := newBaseDocEqIndex(func(Ref) bool { return false })
	idx.kind = rules[0].Head.RuleKind()

	indices := newrefindices(idx.isVirtual)
	values := make(map[Var]Value)

	for ridx := range rules {
		WalkRules(rules[ridx], func(rule *Rule) bool {
			if rule.Default {
				idx.defaultRule = rule
				return false
			}
			if idx.onlyGroundRefs {
				idx.onlyGroundRefs = rule.Head.Reference.IsGround()
			}
			if !slices.ContainsFunc(rule.Body, skipIndexingOperator) {
				clear(values)
				for i := range rule.Body {
					indices.Update(rule, rule.Body[i], values)
				}
			}
			return false
		})
	}

	order := orderFn(indices)

	for ridx := range rules {
		var prio int
		WalkRules(rules[ridx], func(rule *Rule) bool {
			if rule.Default {
				return false
			}
			node := idx.root
			if len(indices.rules[rule]) > 0 {
				for _, ref := range order {
					var vals []*refindex
					for _, ri := range indices.rules[rule] {
						if ri.Ref.Equal(ref) {
							vals = append(vals, ri)
						}
					}
					if len(vals) == 0 {
						node = node.Insert(ref, nil, nil)
					} else {
						node = node.Insert(ref, vals[0].Value, vals[0].Mapper)
					}
				}
			}
			node.append([...]int{ridx, prio}, rule)
			prio++
			return false
		})
	}

	return idx
}

// orderingBenchModule builds a rule set in which the position of the
// highest-frequency refs drives an asymptotic difference in *lookup*
// (trie-traversal) cost.
//
// Each rule constrains d shared "gate" refs (input.h0..h{d-1}, referenced by
// every rule -- so highest frequency) plus one rule-unique "detail" ref
// (input.cN, frequency 1). The query matches every rule's detail ref but
// only the small fraction of rules whose gate values are all 0.
//
// This is a common real-world shape: every rule checks a handful of common
// attributes (method, tenant, ...) and one specific thing (an exact resource
// path). What makes it a good demonstrator is how the gates' position
// interacts with traversal:
//
//   - Gates early (frequency-descending, what Sorted() produces): resolving
//     the gates near the top of the trie prunes the candidate set by base^d
//     before any detail ref is reached, so only the surviving handful of
//     rules' detail levels are ever walked. Traversal is ~O(N).
//   - Gates scattered/late (an order that ignores frequency): every rule's
//     detail ref matches the query, so rules stay live down long runs of
//     unique detail levels, and the pruning gates aren't reached until deep
//     in the trie -- after much of it has been traversed. Traversal degrades
//     toward ~O(N^2).
//
// (Note the descending trie is physically *larger* -- the gates fan out into
// base^d buckets near the root -- but almost none of it is visited. The win
// is entirely in traversal cost, not trie size.)
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

// BenchmarkRuleIndexRefOrdering measures Lookup cost for the same rule set and
// query with vs. without the frequency-descending ref ordering: the real
// Sorted() order that Build() uses, against an order that ignores frequency
// (what you'd get by skipping the sort). It isolates the ordering effect
// alone -- both indexes use the identical trie code, and only Lookup is timed.
func BenchmarkRuleIndexRefOrdering(b *testing.B) {
	const (
		n    = 400
		d    = 4
		base = 3
	)
	mod, input := orderingBenchModule(n, d, base)

	orders := []struct {
		name string
		fn   func(*refindices) []Ref
	}{
		{"frequency-descending", (*refindices).Sorted}, // what Build() uses
		{"unordered", unorderedRefs(1)},                // no frequency sort
	}

	for _, o := range orders {
		idx := buildIndexWithOrder(mod.Rules, o.fn)
		resolver := testResolver{input: input}
		b.Run(o.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				if _, err := idx.Lookup(resolver); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestRuleIndexRefOrderingGap documents (and guards the correctness invariant
// behind) the traversal-cost gap the benchmark exploits: sorting by frequency
// and not sorting must return the same candidate rules, and the sorted order
// must resolve far fewer refs. To avoid depending on any single arbitrary
// order, the unsorted baseline is averaged over several shuffles.
func TestRuleIndexRefOrderingGap(t *testing.T) {
	const seeds = 8
	for _, n := range []int{100, 200, 400} {
		mod, input := orderingBenchModule(n, 4, 3)

		desc := buildIndexWithOrder(mod.Rules, (*refindices).Sorted)
		dr := &countingResolver{input: input}
		descRes, err := desc.Lookup(dr)
		if err != nil {
			t.Fatal(err)
		}

		total := 0
		for seed := range seeds {
			un := buildIndexWithOrder(mod.Rules, unorderedRefs(int64(seed)))
			ur := &countingResolver{input: input}
			unRes, err := un.Lookup(ur)
			if err != nil {
				t.Fatal(err)
			}
			if !NewRuleSet(descRes.Rules...).Equal(NewRuleSet(unRes.Rules...)) {
				t.Fatalf("n=%d seed=%d: candidate rule sets differ between orderings", n, seed)
			}
			total += ur.calls
		}
		meanUnordered := float64(total) / float64(seeds)

		t.Logf("n=%4d  descending: %6d resolves   unordered (mean of %d): %8.0f resolves   (%.1fx)",
			n, dr.calls, seeds, meanUnordered, meanUnordered/float64(dr.calls))

		if meanUnordered <= float64(dr.calls) {
			t.Fatalf("n=%d: expected descending ordering to resolve fewer refs than unordered, got descending=%d unordered mean=%.0f",
				n, dr.calls, meanUnordered)
		}
	}
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
