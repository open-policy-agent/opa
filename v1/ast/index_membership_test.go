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

// everyCandidate is a resolver answering that every candidate goes on to be
// evaluated, which is what makes asking a collection worthwhile.
type everyCandidate struct{ testResolver }

func (everyCandidate) IndexEveryCandidateEvaluated() bool { return true }

// collectionPolicy is n rules that each grant one group access to one resource,
// with the group check written as cond. The resource is what a data-filtering
// query leaves unknown, so the group check is the only condition that can
// exclude anything.
func collectionPolicy(n int, cond string) string {
	var sb strings.Builder
	sb.WriteString("package test\n\n")
	for i := range n {
		sb.WriteString("allow if {\n\tinput.action == \"view\"\n")
		fmt.Fprintf(&sb, "\tinput.resource.root == %q\n", fmt.Sprintf("root%d", i))
		fmt.Fprintf(&sb, "\t"+cond+"\n}\n\n", i)
	}
	return sb.String()
}

// collectionData is n groups of m members each, held the way kind asks for. Base
// data cannot hold a set, so there are only the two.
func collectionData(t *testing.T, n, m int, kind string) *Term {
	t.Helper()

	var groups []string
	for k := range n {
		var members []string
		for j := range m {
			id := fmt.Sprintf("u%d_%d", k, j)
			if kind == "object" {
				members = append(members, fmt.Sprintf("%q: true", id))
			} else {
				members = append(members, fmt.Sprintf("%q", id))
			}
		}
		open, close := "{", "}"
		if kind == "array" {
			open, close = "[", "]"
		}
		groups = append(groups, fmt.Sprintf("%q: {\"members\": %s%s%s}",
			fmt.Sprintf("g%d", k), open, strings.Join(members, ", "), close))
	}

	return MustParseTerm("{\"groups\": {" + strings.Join(groups, ", ") + "}}")
}

func TestBaseDocEqIndexCollectionMembership(t *testing.T) {
	const n, m = 50, 10

	for _, tc := range []struct {
		note   string
		kind   string
		cond   string
		expect int
	}{
		{
			note:   "object, key lookup",
			kind:   "object",
			cond:   "data.groups.g%d.members[input.subject]",
			expect: 1,
		},
		{
			// `in` asks whether the subject is one of the collection's values,
			// which an object answers only by being walked -- so the rules stay
			// candidates, as they did before this.
			note:   "object, in: not indexed",
			kind:   "object",
			cond:   "input.subject in data.groups.g%d.members",
			expect: n,
		},
		{
			note:   "array, in: not indexed",
			kind:   "array",
			cond:   "input.subject in data.groups.g%d.members",
			expect: n,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Compile(map[string]*Module{"test.rego": MustParseModule(collectionPolicy(n, tc.cond))})
			if c.Failed() {
				t.Fatal(c.Errors)
			}

			// The resource is what a data-filtering query does not know, which
			// is what leaves the group check as the only condition able to
			// exclude anything.
			resolver := testResolver{
				input:       MustParseTerm(fmt.Sprintf(`{"subject": "u%d_0", "action": "view"}`, n/2)),
				data:        collectionData(t, n, m, tc.kind),
				unknownRefs: NewSet(NewTerm(MustParseRef("input.resource.root"))),
			}

			res, err := c.RuleIndex(MustParseRef("data.test.allow")).Lookup(everyCandidate{resolver})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Rules) != tc.expect {
				t.Errorf("expected %d candidates, got %d", tc.expect, len(res.Rules))
			}
		})
	}
}

// TestBaseDocEqIndexCollectionKeyShapes pins which references are read as a
// lookup in a collection. Anything else keeps the rules it is written in, which
// is what they did before.
func TestBaseDocEqIndexCollectionKeyShapes(t *testing.T) {
	for _, tc := range []struct {
		note    string
		cond    string
		indexed bool
	}{
		{
			note:    "ground prefix, key is a ref",
			cond:    "data.groups.devs.members[input.subject]",
			indexed: true,
		},
		{
			note:    "ground prefix, key is a local bound to a ref",
			cond:    "s := input.subject\n\tdata.groups.devs.members[s]",
			indexed: true,
		},
		{
			note:    "prefix contains a variable",
			cond:    "data.groups[input.g].members[input.subject]",
			indexed: false,
		},
		{
			// What a structured subject renders as: array indices and all, still
			// ground, so still the collection's key.
			note:    "key reached through array indices",
			cond:    "data.groups.devs.members[input.subject[0].identifiers[0].id]",
			indexed: true,
		},
		{
			note:    "key contains a variable",
			cond:    "data.groups.devs.members[input.subject[i]]",
			indexed: false,
		},
		{
			note:    "collection is not in data",
			cond:    "input.groups.devs.members[input.subject]",
			indexed: false,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			module := fmt.Sprintf(`package test

p if {
	%s
}

q := 1`, tc.cond)

			c := NewCompiler()
			c.Compile(map[string]*Module{"test.rego": MustParseModule(module)})
			if c.Failed() {
				t.Fatal(c.Errors)
			}

			idx, ok := c.RuleIndex(MustParseRef("data.test.p")).(*baseDocEqIndex)
			if !ok {
				t.Fatal("expected an index")
			}
			if got := len(idx.memberships) > 0; got != tc.indexed {
				t.Errorf("expected indexed=%v, got %v", tc.indexed, got)
			}
		})
	}
}

// TestBaseDocEqIndexVirtualCollection covers a collection another rule produces.
// The resolver answers for base documents, so a virtual one is never recorded --
// asking about it would exclude rules over an answer it cannot give.
func TestBaseDocEqIndexVirtualCollection(t *testing.T) {
	for _, tc := range []struct {
		note   string
		module string
	}{
		{
			note: "collection is a complete rule",
			module: `package test

groups := {"g0": {"members": {"alice": true}}}

allow if data.test.groups.g0.members[input.subject]`,
		},
		{
			note: "collection is a partial set rule",
			module: `package test

members contains "alice"

allow if data.test.members[input.subject]`,
		},
		{
			note: "collection is produced by a general ref rule",
			module: `package test

g[x].members := {"alice": true} if x := "g0"

allow if data.test.g.g0.members[input.subject]`,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Compile(map[string]*Module{"test.rego": MustParseModule(tc.module)})
			if c.Failed() {
				t.Fatal(c.Errors)
			}

			idx, ok := c.RuleIndex(MustParseRef("data.test.allow")).(*baseDocEqIndex)
			if !ok {
				t.Fatal("expected an index")
			}
			if len(idx.memberships) != 0 {
				t.Errorf("expected no collection recorded, got %d", len(idx.memberships))
			}
		})
	}
}

// TestBaseDocEqIndexCollectionMembershipKeeps covers the cases a lookup must not
// exclude a rule over: it has to be as ready to be wrong about the collection as
// it was before reading one.
func TestBaseDocEqIndexCollectionMembershipKeeps(t *testing.T) {
	const module = `package test

allow if {
	input.action == "view"
	data.groups.g0.members[input.subject]
}

allow if {
	input.action == "view"
	data.groups.g1.members[input.subject]
}`

	data := MustParseTerm(`{"groups": {"g0": {"members": {"alice": true}}, "g1": {"members": {"bob": true}}}}`)
	input := MustParseTerm(`{"subject": "alice", "action": "view"}`)

	for _, tc := range []struct {
		note     string
		resolver testResolver
		expect   int
	}{
		{
			note:     "a member of one group reaches that rule only",
			resolver: testResolver{input: input, data: data},
			expect:   1,
		},
		{
			note:     "a member of neither reaches none",
			resolver: testResolver{input: MustParseTerm(`{"subject": "carol", "action": "view"}`), data: data},
			expect:   0,
		},
		{
			note: "an unknown collection keeps every rule",
			resolver: testResolver{
				input:       input,
				data:        data,
				unknownRefs: NewSet(NewTerm(MustParseRef("data.groups"))),
			},
			expect: 2,
		},
		{
			note:     "a collection that is not there reaches no rule",
			resolver: testResolver{input: input, data: MustParseTerm(`{}`)},
			expect:   0,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			c := NewCompiler()
			c.Compile(map[string]*Module{"test.rego": MustParseModule(module)})
			if c.Failed() {
				t.Fatal(c.Errors)
			}

			res, err := c.RuleIndex(MustParseRef("data.test.allow")).Lookup(everyCandidate{tc.resolver})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Rules) != tc.expect {
				t.Errorf("expected %d candidates, got %d: %v", tc.expect, len(res.Rules), res.Rules)
			}
		})
	}
}

// TestBaseDocEqIndexCollectionSet covers a collection that is a set. Base data
// read from JSON never holds one, but a store keeping ast.Value can, and a `with`
// statement replacing the collection can put one there -- and a set answers the key
// question in one lookup exactly as an object does.
func TestBaseDocEqIndexCollectionSet(t *testing.T) {
	const module = `package test

allow contains "g0" if data.groups.g0.members[input.subject]

allow contains "g1" if data.groups.g1.members[input.subject]`

	// g0 holds a set, g1 an object, so one lookup exercises both branches.
	data := MustParseTerm(`{"groups": {
		"g0": {"members": {"carol"}},
		"g1": {"members": {"bob": true}}
	}}`)

	for _, tc := range []struct {
		subject string
		expect  int
	}{
		{"carol", 1}, // in the set
		{"bob", 1},   // in the object
		{"dave", 0},  // in neither: the set has to exclude, not fall through
	} {
		t.Run(tc.subject, func(t *testing.T) {
			c := NewCompiler()
			c.Compile(map[string]*Module{"test.rego": MustParseModule(module)})
			if c.Failed() {
				t.Fatal(c.Errors)
			}

			res, err := c.RuleIndex(MustParseRef("data.test.allow")).Lookup(testResolver{
				input: MustParseTerm(fmt.Sprintf(`{"subject": %q}`, tc.subject)),
				data:  data,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Rules) != tc.expect {
				t.Errorf("expected %d candidates, got %d", tc.expect, len(res.Rules))
			}
		})
	}
}

// TestBaseDocEqIndexCollectionsConjunction covers a rule reading more than one
// collection: every one of them has to hold for the rule to match.
func TestBaseDocEqIndexCollectionsConjunction(t *testing.T) {
	const module = `package test

allow contains "a" if {
	data.groups.mandatory.members[input.subject]
	data.groups.others.members[input.subject]
}

allow contains "b" if {
	data.groups.mandatory.members[input.subject]
	data.groups.third.members[input.subject]
}`

	data := MustParseTerm(`{"groups": {
		"mandatory": {"members": {"alice": true, "bob": true, "carol": true}},
		"others": {"members": {"alice": true}},
		"third": {"members": {"bob": true}}
	}}`)

	for _, tc := range []struct {
		subject string
		expect  int
	}{
		{"alice", 1}, // mandatory and others
		{"bob", 1},   // mandatory and third
		{"carol", 0}, // mandatory alone holds neither rule
		{"dave", 0},
	} {
		t.Run(tc.subject, func(t *testing.T) {
			c := NewCompiler()
			c.Compile(map[string]*Module{"test.rego": MustParseModule(module)})
			if c.Failed() {
				t.Fatal(c.Errors)
			}

			res, err := c.RuleIndex(MustParseRef("data.test.allow")).Lookup(testResolver{
				input: MustParseTerm(fmt.Sprintf(`{"subject": %q}`, tc.subject)),
				data:  data,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Rules) != tc.expect {
				t.Errorf("expected %d candidates, got %d", tc.expect, len(res.Rules))
			}
		})
	}
}

// TestBaseDocEqIndexCollectionInAlternative guards the soundness of recording a
// collection: one that only an `or` operand reads does not have to hold for the
// rule to match. Operands are indexed into a scratch of their own, whose
// collections Build does not read, so nothing is recorded and no answer is lost --
// at the price of the rule staying a candidate for a subject in neither group.
func TestBaseDocEqIndexCollectionInAlternative(t *testing.T) {
	const module = `package test

import future.keywords.or

allow contains "yes" if {
	{data.groups.g0.members[input.subject]} or {data.groups.g1.members[input.subject]}
}`

	data := MustParseTerm(`{"groups": {"g0": {"members": {"u0": true}}, "g1": {"members": {"u1": true}}}}`)

	for _, subject := range []string{"u0", "u1", "u2"} {
		t.Run(subject, func(t *testing.T) {
			c := NewCompiler()
			c.Compile(map[string]*Module{"test.rego": MustParseModule(module)})
			if c.Failed() {
				t.Fatal(c.Errors)
			}

			res, err := c.RuleIndex(MustParseRef("data.test.allow")).Lookup(testResolver{
				input: MustParseTerm(fmt.Sprintf(`{"subject": %q}`, subject)),
				data:  data,
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Rules) != 1 {
				t.Errorf("expected the rule to stay a candidate, got %d", len(res.Rules))
			}
		})
	}
}

// TestBaseDocEqIndexCollectionEarlyExit covers when a collection is worth asking
// about. Where the caller stops at the first candidate that holds, gather would
// have to ask about every one of them to return any -- and the collection is what
// the rule tests first anyway, so the asking is work evaluation was going to do.
func TestBaseDocEqIndexCollectionEarlyExit(t *testing.T) {
	const n = 20

	for _, tc := range []struct {
		note     string
		head     string
		everyOne bool
		expect   int
	}{
		{
			note:   "one value for every definition: left to evaluation",
			head:   "allow if {",
			expect: n,
		},
		{
			note:     "one value, but the caller evaluates every candidate",
			head:     "allow if {",
			everyOne: true,
			expect:   1,
		},
		{
			note:   "a partial set cannot exit early: asked",
			head:   "allow contains \"yes\" if {",
			expect: 1,
		},
		{
			note:   "definitions disagreeing on the value cannot exit early: asked",
			head:   "allow := IDX if {",
			expect: 1,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			var sb strings.Builder
			sb.WriteString("package test\n\n")
			for i := range n {
				sb.WriteString(strings.ReplaceAll(tc.head, "IDX", strconv.Itoa(i)) + "\n")
				fmt.Fprintf(&sb, "\tdata.groups.g%d.members[input.subject]\n}\n\n", i)
			}

			c := NewCompiler()
			c.Compile(map[string]*Module{"test.rego": MustParseModule(sb.String())})
			if c.Failed() {
				t.Fatal(c.Errors)
			}

			base := testResolver{
				input: MustParseTerm(`{"subject": "u0_0"}`),
				data:  collectionData(t, n, 4, "object"),
			}
			var resolver ValueResolver = base
			if tc.everyOne {
				resolver = everyCandidate{base}
			}

			res, err := c.RuleIndex(MustParseRef("data.test.allow")).Lookup(resolver)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Rules) != tc.expect {
				t.Errorf("expected %d candidates, got %d", tc.expect, len(res.Rules))
			}
		})
	}
}

// BenchmarkLookupCollectionMembership is what asking the collection buys a
// lookup over one rule per group. The collection is never on the trie, so the
// cost does not follow how many members it holds.
//
// 50 rules, one group each, the subject in one of them:
//
//	                 candidates   lookup
//	members=10                1   7.4us
//	members=1000              1   7.7us
//	members=100000            1   8.2us
func BenchmarkLookupCollectionMembership(b *testing.B) {
	const n = 50

	for _, m := range []int{10, 1000, 100000} {
		var groups []string
		for k := range n {
			var members []string
			for j := range m {
				members = append(members, fmt.Sprintf("%q: true", fmt.Sprintf("u%d_%d", k, j)))
			}
			groups = append(groups, fmt.Sprintf("%q: {\"members\": {%s}}",
				fmt.Sprintf("g%d", k), strings.Join(members, ", ")))
		}
		data := MustParseTerm("{\"groups\": {" + strings.Join(groups, ", ") + "}}")

		c := NewCompiler()
		c.Compile(map[string]*Module{"test.rego": MustParseModule(
			collectionPolicy(n, "data.groups.g%d.members[input.subject]"))})
		if c.Failed() {
			b.Fatal(c.Errors)
		}

		resolver := testResolver{
			input:       MustParseTerm(fmt.Sprintf(`{"subject": "u%d_0", "action": "view"}`, n/2)),
			data:        data,
			unknownRefs: NewSet(NewTerm(MustParseRef("input.resource.root"))),
		}

		index := c.RuleIndex(MustParseRef("data.test.allow"))
		res, err := index.Lookup(resolver)
		if err != nil {
			b.Fatal(err)
		}
		candidates := float64(len(res.Rules))

		b.Run(fmt.Sprintf("members=%d", m), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := index.Lookup(resolver); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(candidates, "candidates")
		})
	}
}
