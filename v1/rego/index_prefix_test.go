// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// TestPrefixIndexMatchesUnindexedEval is the property the prefix index owes its
// callers: the index is an optimisation, so a query has to give the same answer
// with it as without it. The rule index in v1/ast is tested on the rules it
// selects; this one runs the policies and compares.
//
// The policies are generated rather than written out because the shapes that
// break a prefix index are combinations -- one prefix extending another, a
// prefix alongside an equality on the same reference, a value that isn't a
// string at all -- and a fixed table only reaches the combinations someone
// thought of. The alphabet is deliberately tiny so that prefixes collide,
// nest, and split trie edges constantly.
func TestPrefixIndexMatchesUnindexedEval(t *testing.T) {
	ctx := t.Context()

	for seed := range 4 {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			rnd := rand.New(rand.NewSource(int64(seed)))

			for range 100 {
				policy := randomPrefixPolicy(rnd)

				pq, err := New(Query("data.test.p"), Module("test.rego", policy)).PrepareForEval(ctx)
				if err != nil {
					t.Fatalf("%v\npolicy:\n%s", err, policy)
				}

				for range 10 {
					input := randomPrefixInput(rnd)

					var results [2]string
					for i, indexing := range []bool{true, false} {
						rs, err := pq.Eval(ctx, EvalInput(input), EvalRuleIndexing(indexing))
						if err != nil {
							t.Fatalf("%v\npolicy:\n%s", err, policy)
						}
						results[i] = fmt.Sprint(rs)
					}

					if results[0] != results[1] {
						t.Fatalf("indexed %v != unindexed %v for input %v\npolicy:\n%s",
							results[0], results[1], input, policy)
					}
				}
			}
		})
	}
}

// prefixAlphabet is small enough that randomly generated prefixes overlap,
// nest, and diverge mid-edge all the time.
const prefixAlphabet = "ab/"

func randomPrefixString(rnd *rand.Rand, maxLen int) string {
	var sb strings.Builder
	for range rnd.Intn(maxLen + 1) {
		sb.WriteByte(prefixAlphabet[rnd.Intn(len(prefixAlphabet))])
	}
	return sb.String()
}

func randomPrefixPolicy(rnd *rand.Rand) string {
	var sb strings.Builder
	sb.WriteString("package test\n\nimport rego.v1\n\n")

	writeList := func(opening, closing string, n int) {
		sb.WriteString(opening)
		for j := range n {
			if j > 0 {
				sb.WriteString(", ")
			}
			fmt.Fprintf(&sb, "%q", randomPrefixString(rnd, 4))
		}
		sb.WriteString(closing)
	}

	for i := range 1 + rnd.Intn(8) {
		switch rnd.Intn(7) {
		case 0:
			fmt.Fprintf(&sb, "p contains %d if startswith(input.path, %q)\n\n", i, randomPrefixString(rnd, 4))
		case 1:
			fmt.Fprintf(&sb, "p contains %d if strings.any_prefix_match(input.path, ", i)
			writeList("[", "]", 1+rnd.Intn(4))
			sb.WriteString(")\n\n")
		case 2:
			fmt.Fprintf(&sb, "p contains %d if strings.any_prefix_match(input.path, ", i)
			writeList("{", "}", 1+rnd.Intn(3))
			sb.WriteString(")\n\n")
		case 3:
			// A prefix and an inequality on the same reference.
			fmt.Fprintf(&sb, "p contains %d if {\n\tx := input.path\n\tstartswith(x, %q)\n\tx != %q\n}\n\n",
				i, randomPrefixString(rnd, 3), randomPrefixString(rnd, 4))
		case 4:
			// A prefix and an equality on the same reference, which the index
			// records side by side and reaches the rule through either.
			fmt.Fprintf(&sb, "p contains %d if {\n\tx := input.path\n\tstartswith(x, %q)\n\tx == %q\n}\n\n",
				i, randomPrefixString(rnd, 2), randomPrefixString(rnd, 3))
		case 5:
			// A prefix on one reference and an equality on another.
			fmt.Fprintf(&sb, "p contains %d if {\n\tstartswith(input.path, %q)\n\tinput.other == %q\n}\n\n",
				i, randomPrefixString(rnd, 3), randomPrefixString(rnd, 2))
		case 6:
			// Two rules producing the same value by different means.
			fmt.Fprintf(&sb, "p contains %d if startswith(input.path, %q)\n\n", i, randomPrefixString(rnd, 3))
			fmt.Fprintf(&sb, "p contains %d if strings.any_prefix_match(input.path, ", i)
			writeList("[", "]", 2)
			sb.WriteString(")\n\n")
		}
	}

	return sb.String()
}

func randomPrefixInput(rnd *rand.Rand) map[string]any {
	input := map[string]any{
		"path":  randomPrefixString(rnd, 6),
		"other": randomPrefixString(rnd, 2),
	}

	switch rnd.Intn(6) {
	case 0:
		// strings.any_prefix_match takes a collection of search strings too.
		input["path"] = []any{randomPrefixString(rnd, 4), randomPrefixString(rnd, 4)}
	case 1:
		// Not a string at all: the calls error and the rules are undefined.
		input["path"] = 42
	case 2:
		delete(input, "path")
	}

	return input
}
