// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// countingTestResolver counts what a lookup asks for. The countingResolver in
// index_ordering_bench_test.go deliberately refuses data refs, and these tests are
// about data refs.
type countingTestResolver struct {
	testResolver
	calls int
}

func (r *countingTestResolver) Resolve(ref Ref) (Value, error) {
	r.calls++
	return r.testResolver.Resolve(ref)
}

// unvaluedIndex compiles src and returns the index built for data.test.p, so that
// these tests see the refs RewriteDynamicTerms hoisted into locals -- which is
// where the refs no rule constrains to a value come from.
func unvaluedIndex(t *testing.T, src string) RuleIndex {
	t.Helper()

	c := NewCompiler()
	if c.Compile(map[string]*Module{"p.rego": MustParseModuleWithOpts(src, ParserOptions{
		FutureKeywords: []string{"and", "or"},
	})}); c.Failed() {
		t.Fatal(c.Errors)
	}

	index := c.RuleIndex(MustParseRef("data.test.p"))
	if index == nil {
		t.Fatal("no index built for data.test.p")
	}
	return index
}

func lookupValues(t *testing.T, index RuleIndex, r ValueResolver) []string {
	t.Helper()

	res, err := index.Lookup(r)
	if err != nil {
		t.Fatal(err)
	}

	values := make([]string, 0, len(res.Rules))
	for _, rule := range res.Rules {
		values = append(values, rule.Head.Value.String())
	}
	// Which rules survive is the point; the order candidates come back in is a
	// property of trie traversal, and this change moves some of them out of it.
	slices.Sort(values)
	return values
}

// TestIndexUnvaluedRefExcludesWhenAbsent is the behaviour partition must not give
// up: a rule that reads a ref nothing constrains to a value is still excluded when
// that ref is absent. Traversal used to answer this by branching on the ref; now
// Lookup asks per candidate.
func TestIndexUnvaluedRefExcludesWhenAbsent(t *testing.T) {
	index := unvaluedIndex(t, `package test

p := 1 if input.subject in data.groups.g0.members

p := 2 if input.subject in data.groups.g1.members
`)

	for _, tc := range []struct {
		note string
		data string
		exp  []string
	}{
		{note: "both groups present", data: `{"groups": {"g0": {"members": []}, "g1": {"members": []}}}`, exp: []string{"1", "2"}},
		{note: "g1 absent", data: `{"groups": {"g0": {"members": []}}}`, exp: []string{"1"}},
		{note: "g0 absent", data: `{"groups": {"g1": {"members": []}}}`, exp: []string{"2"}},
		{note: "both absent", data: `{"groups": {}}`, exp: []string{}},
	} {
		t.Run(tc.note, func(t *testing.T) {
			act := lookupValues(t, index, testResolver{
				input: MustParseTerm(`{"subject": "u1"}`),
				data:  MustParseTerm(tc.data),
			})
			if strings.Join(act, ",") != strings.Join(tc.exp, ",") {
				t.Errorf("expected %v, got %v", tc.exp, act)
			}
		})
	}
}

// TestIndexUnvaluedRefResolverError is the error path of that per-candidate ask.
// The refs partition keeps out of the trie are resolved while a lookup's result is
// being read, which is a place an error has to travel out of by itself -- the
// traversal it used to happen inside could not carry one.
func TestIndexUnvaluedRefResolverError(t *testing.T) {
	index := unvaluedIndex(t, `package test

p := 1 if input.subject in data.groups.g0.members
`)

	_, err := index.Lookup(testResolver{
		input:   MustParseTerm(`{"subject": "u1"}`),
		data:    MustParseTerm(`{"groups": {"g0": {"members": []}}}`),
		failRef: MustParseRef("data.groups.g0.members"),
	})
	if err == nil || err.Error() != "some error" {
		t.Fatalf("expected the resolver's error, got: %v", err)
	}
}

// TestIndexUnvaluedRefInOneAlternative guards the soundness of require: a ref only
// one operand of an `or` reads does not have to hold for the rule to match, so it
// must not exclude it.
func TestIndexUnvaluedRefInOneAlternative(t *testing.T) {
	index := unvaluedIndex(t, `package test

p := 1 if {
	input.subject in data.groups.g0.members or input.subject in data.groups.g1.members
}
`)

	// g0 is absent, so only the second operand can hold -- the rule stays.
	act := lookupValues(t, index, testResolver{
		input: MustParseTerm(`{"subject": "u1"}`),
		data:  MustParseTerm(`{"groups": {"g1": {"members": ["u1"]}}}`),
	})
	if len(act) != 1 {
		t.Fatalf("expected the rule to survive on the reachable alternative, got %v", act)
	}
}

// TestIndexUnvaluedRefsDoNotMultiply is the point of the change: a ref no rule
// constrains to a value is not a trie level, so n of them cost n resolves at
// lookup rather than n(n+1)/2 -- while selecting the same rules.
func TestIndexUnvaluedRefsDoNotMultiply(t *testing.T) {
	for _, n := range []int{10, 50, 100} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			var src strings.Builder
			src.WriteString("package test\n\n")
			groups := strings.Builder{}
			groups.WriteString(`{"groups": {`)
			for i := range n {
				fmt.Fprintf(&src, "p := %d if {\n\tinput.subject in data.groups.g%d.members\n\tinput.foo == \"A\"\n}\n\n", i, i)
				if i > 0 {
					groups.WriteString(", ")
				}
				fmt.Fprintf(&groups, `"g%d": {"members": ["u1"]}`, i)
			}
			groups.WriteString("}}")

			index := unvaluedIndex(t, src.String())
			r := &countingTestResolver{testResolver: testResolver{
				input: MustParseTerm(`{"subject": "u1", "foo": "A"}`),
				data:  MustParseTerm(groups.String()),
			}}

			res, err := index.Lookup(r)
			if err != nil {
				t.Fatal(err)
			}

			// Nothing discriminates between these rules -- input.foo is "A" in all
			// of them -- so every one is a candidate either way.
			if len(res.Rules) != n {
				t.Errorf("expected %d candidates, got %d", n, len(res.Rules))
			}

			// One resolve per candidate per ref it reads (input.subject and its own
			// group), plus input.foo. Quadratic would be n(n+1)/2 and up.
			if max := 3 * n; r.calls > max {
				t.Errorf("expected at most %d resolves for %d rules, got %d", max, n, r.calls)
			}
		})
	}
}
