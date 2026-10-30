// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem"
)

// collectionPolicy is n rules, each granting one group access, with head deciding
// whether a caller can stop at the first that holds. Nothing else discriminates,
// so the group is what an index has to work with.
func collectionPolicy(n, members int, head string) (string, map[string]any) {
	var sb strings.Builder
	sb.WriteString("package test\n\n")
	for i := range n {
		sb.WriteString(strings.ReplaceAll(head, "IDX", strconv.Itoa(i)) + " if {\n")
		fmt.Fprintf(&sb, "\tdata.groups.g%d.members[input.subject]\n}\n\n", i)
	}

	groups := make(map[string]any, n)
	for k := range n {
		ms := make(map[string]any, members)
		for j := range members {
			ms[fmt.Sprintf("u%d_%d", k, j)] = true
		}
		groups[fmt.Sprintf("g%d", k)] = map[string]any{"members": ms}
	}

	return sb.String(), map[string]any{"groups": groups}
}

// BenchmarkCollectionLookupEval is what asking the collection is worth to a whole
// query, and what it costs. The subject's group decides how far evaluation gets
// before a rule holds, so the match position is a dimension of its own.
//
// A ruleset of `allow` definitions all producing true lets evaluation stop at the
// first match, and the index leaves the collections alone: an early match is then
// cheap and a late one is not, and asking would have cost what evaluation was
// about to spend. `allow contains` cannot stop, so every candidate is evaluated
// whatever the position -- and there excluding all but one is worth its asking.
//
//	500 rules, 200 members per group
//
//	              early exit                  no early exit
//	            before        after         before        after
//	  match=0    11670 ns/op   14632 ns/op  356424 ns/op  149821 ns/op
//	  match=250 190453 ns/op  192324 ns/op  357568 ns/op  146598 ns/op
//	  match=499 352908 ns/op  359272 ns/op  346803 ns/op  152011 ns/op
func BenchmarkCollectionLookupEval(b *testing.B) {
	const n, members = 500, 200

	for _, head := range []string{"allow", `allow contains "yes"`} {
		kind := "early-exit"
		if strings.Contains(head, "contains") {
			kind = "no-early-exit"
		}

		module, data := collectionPolicy(n, members, head)
		store := inmem.NewFromObject(data)

		for _, match := range []int{0, n / 2, n - 1} {
			pq, err := New(
				ParsedQuery(ast.MustParseBody("data.test.allow")),
				ParsedModule(ast.MustParseModule(module)),
				Store(store),
				GenerateJSON(noOpGenerateJSON),
			).PrepareForEval(b.Context())
			if err != nil {
				b.Fatal(err)
			}

			input := ast.MustParseTerm(fmt.Sprintf(`{"subject": "u%d_0"}`, match))
			if rs, err := pq.Eval(b.Context(), EvalParsedInput(input.Value)); err != nil {
				b.Fatal(err)
			} else if len(rs) != 1 {
				b.Fatalf("expected one result, got %d", len(rs))
			}

			b.Run(fmt.Sprintf("%s/match=%d", kind, match), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := pq.Eval(b.Context(), EvalParsedInput(input.Value)); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// BenchmarkCollectionLookupPartial is the data-filtering shape: the subject is
// known and the resource is not, so nothing but the group can exclude a rule --
// and partial evaluation reaches every candidate, so excluding one pays.
//
//	                            before        after
//	100 rules                  187577 ns/op   61043 ns/op
//	500 rules                  658317 ns/op  173877 ns/op
func BenchmarkCollectionLookupPartial(b *testing.B) {
	const members = 200

	for _, n := range []int{100, 500} {
		var sb strings.Builder
		sb.WriteString("package test\n\n")
		for i := range n {
			fmt.Fprintf(&sb, "allow if {\n\tinput.resource.id == %q\n", fmt.Sprintf("r%d", i))
			fmt.Fprintf(&sb, "\tdata.groups.g%d.members[input.subject]\n}\n\n", i)
		}
		_, data := collectionPolicy(n, members, "allow")

		input := ast.MustParseTerm(fmt.Sprintf(`{"subject": "u%d_0"}`, n/2))
		pq, err := New(
			ParsedQuery(ast.MustParseBody("data.test.allow")),
			ParsedModule(ast.MustParseModule(sb.String())),
			Store(inmem.NewFromObject(data)),
			ParsedInput(input.Value),
			Unknowns([]string{"input.resource"}),
		).PrepareForPartial(b.Context())
		if err != nil {
			b.Fatal(err)
		}

		b.Run(fmt.Sprintf("rules=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := pq.Partial(b.Context()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkCollectionLookupAnyOf is the same with each rule granting any of k
// groups.
//
//	500 rules, 3 groups each, 200 members per group
//
//	                           before         after
//	or/early-exit/match=0       18.2 us/op     18.0 us/op
//	or/early-exit/match=499     1071 us/op     1075 us/op
//	or/no-early-exit/match=0    1122 us/op      152 us/op
//	or/no-early-exit/match=499  1069 us/op      152 us/op
//	or/partial                  2575 us/op      176 us/op
func BenchmarkCollectionLookupAnyOf(b *testing.B) {
	const n, k, members = 500, 3, 200

	forms := []struct {
		name string
		cond func(groups []string) string
	}{
		{"or", func(groups []string) string {
			lookups := make([]string, len(groups))
			for j, g := range groups {
				lookups[j] = fmt.Sprintf("{data.groups[%s].members[input.subject]}", g)
			}
			return strings.Join(lookups, " or ")
		}},
		{"some-in", func(groups []string) string {
			return fmt.Sprintf("some g in {%s}\n\tdata.groups[g].members[input.subject]", strings.Join(groups, ", "))
		}},
	}

	groups := make(map[string]any, n*k)
	for g := range n * k {
		ms := make(map[string]any, members)
		for j := range members {
			ms[fmt.Sprintf("u%d_%d", g, j)] = true
		}
		groups[fmt.Sprintf("g%d", g)] = map[string]any{"members": ms}
	}
	store := inmem.NewFromObject(map[string]any{"groups": groups})

	module := func(cond func([]string) string, head, extra func(int) string) string {
		var sb strings.Builder
		sb.WriteString("package test\n\nimport future.keywords.or\n\n")
		for i := range n {
			ids := make([]string, k)
			for j := range k {
				ids[j] = strconv.Quote(fmt.Sprintf("g%d", i*k+j))
			}
			fmt.Fprintf(&sb, "%s if {\n%s\t%s\n}\n\n", head(i), extra(i), cond(ids))
		}
		return sb.String()
	}
	none := func(int) string { return "" }

	// The subject is in the last group of its rule.
	subject := func(rule int) ast.Value {
		return ast.MustParseTerm(fmt.Sprintf(`{"subject": "u%d_0"}`, rule*k+k-1)).Value
	}

	for _, form := range forms {
		for _, head := range []string{"allow", `allow contains "yes"`} {
			kind := "early-exit"
			if strings.Contains(head, "contains") {
				kind = "no-early-exit"
			}

			pq, err := New(
				ParsedQuery(ast.MustParseBody("data.test.allow")),
				ParsedModule(ast.MustParseModule(module(form.cond, func(int) string { return head }, none))),
				Store(store),
				GenerateJSON(noOpGenerateJSON),
			).PrepareForEval(b.Context())
			if err != nil {
				b.Fatal(err)
			}

			for _, match := range []int{0, n / 2, n - 1} {
				input := subject(match)
				if rs, err := pq.Eval(b.Context(), EvalParsedInput(input)); err != nil {
					b.Fatal(err)
				} else if len(rs) != 1 {
					b.Fatalf("expected one result, got %d", len(rs))
				}

				b.Run(fmt.Sprintf("%s/%s/match=%d", form.name, kind, match), func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						if _, err := pq.Eval(b.Context(), EvalParsedInput(input)); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}

		resource := func(i int) string { return fmt.Sprintf("\tinput.resource.id == \"r%d\"\n", i) }
		pq, err := New(
			ParsedQuery(ast.MustParseBody("data.test.allow")),
			ParsedModule(ast.MustParseModule(module(form.cond, func(int) string { return "allow" }, resource))),
			Store(store),
			ParsedInput(subject(n/2)),
			Unknowns([]string{"input.resource"}),
		).PrepareForPartial(b.Context())
		if err != nil {
			b.Fatal(err)
		}
		b.Run(form.name+"/partial", func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := pq.Partial(b.Context()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
