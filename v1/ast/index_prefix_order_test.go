// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strings"
	"testing"
)

// methodPrefixPolicy is the shape this file is about: a rule per (method,
// prefix set), the way an authorization policy admits a request by what it does
// and where. The prefix set is the reference the rule reaches by more than one
// value, so it is where insertPath stops building the rule's path -- and every
// rule has a second constraint below it.
func methodPrefixPolicy(methods, perRule int) string {
	var sb strings.Builder

	sb.WriteString("package test\n\n")
	for m := range methods {
		fmt.Fprintf(&sb, "allow if {\n\tinput.method == \"M%d\"\n\tstrings.any_prefix_match(input.path, [", m)
		for p := range perRule {
			if p > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%q", fmt.Sprintf("/m%d/p%d/", m, p))
		}
		sb.WriteString("])\n}\n\n")
	}

	return sb.String()
}

func methodPrefixIndex(tb testing.TB, methods, perRule int) *baseDocEqIndex {
	tb.Helper()

	c := MustCompileModules(map[string]string{"test.rego": methodPrefixPolicy(methods, perRule)})
	index := newBaseDocEqIndex(func(Ref) bool { return false })
	if !index.Build(c.Modules["test.rego"].Rules) {
		tb.Fatal("expected index build to succeed")
	}

	return index
}

// TestPrefixIndexKeepsSiblingConstraint pins what the ordering buys: a path
// that one rule's prefix set admits, presented with a method that no rule
// admits, is not a candidate for anything.
//
// Without ranking the prefix reference last (see refindices.alternated) the
// index would return the rule whose prefixes match, since input.method never
// made it onto that rule's path.
func TestPrefixIndexKeepsSiblingConstraint(t *testing.T) {
	index := methodPrefixIndex(t, 8, 4)

	for _, tc := range []struct {
		note, input string
		expected    int
	}{
		{
			note:     "method and prefix both match",
			input:    `{"method": "M3", "path": "/m3/p2/x"}`,
			expected: 1,
		},
		{
			// The path is one this policy admits, but not for this method.
			note:     "prefix matches, method does not",
			input:    `{"method": "NOPE", "path": "/m3/p2/x"}`,
			expected: 0,
		},
		{
			note:     "method matches, prefix does not",
			input:    `{"method": "M3", "path": "/nope/x"}`,
			expected: 0,
		},
		{
			// The prefixes belong to another method's rule.
			note:     "method and prefix match different rules",
			input:    `{"method": "M3", "path": "/m5/p1/x"}`,
			expected: 0,
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			result, err := index.Lookup(testResolver{input: MustParseTerm(tc.input)})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Rules) != tc.expected {
				t.Errorf("expected %d rule(s), got %d", tc.expected, len(result.Rules))
			}
		})
	}
}

// BenchmarkPrefixIndexLookupSiblingConstraint is the lookup on that policy for
// a path one rule admits under a method none of them do. What the index has to
// return is nothing; what it returned before the prefix reference was ranked
// last is the rule the path matched, because input.method was not on its path.
//
//	                            ranked last            by frequency
//	methods=10/prefixes=10    105 ns  0 rules      226 ns  1 rule
//	methods=100/prefixes=10   114 ns  0 rules      241 ns  1 rule
//	methods=100/prefixes=1000 105 ns  0 rules      241 ns  1 rule
//
// Both columns walk the same prefix trie; the difference is the level below it.
// One candidate out of a hundred rules is not much on its own -- the trie was
// already excluding the other ninety-nine -- and half of what shows here is the
// candidate the lookup no longer has to collect. It is the rule the caller then
// does not evaluate that this is for, which costs microseconds rather than
// nanoseconds; BenchmarkRuleIndexPrefixMatchSibling in topdown measures that.
func BenchmarkPrefixIndexLookupSiblingConstraint(b *testing.B) {
	for _, tc := range []struct{ methods, perRule int }{
		{10, 10}, {100, 10}, {100, 1000},
	} {
		b.Run(fmt.Sprintf("methods=%d/prefixes=%d", tc.methods, tc.perRule), func(b *testing.B) {
			index := methodPrefixIndex(b, tc.methods, tc.perRule)
			resolver := testResolver{input: MustParseTerm(`{"method": "NOPE", "path": "/m3/p2/x"}`)}

			result, err := index.Lookup(resolver)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			b.Logf("candidate rules returned: %d", len(result.Rules))

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				if _, err := index.Lookup(resolver); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
