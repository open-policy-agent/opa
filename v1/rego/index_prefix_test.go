// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	inmem "github.com/open-policy-agent/opa/v1/storage/inmem"
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

// TestIndexMatchesUnindexedEval is the same property for every kind of condition
// the index reads, including the collection lookups it checks per candidate:
// evaluation and partial evaluation give the same answers with the index as
// without it. Groups, subjects and values come from small alphabets so that
// rules match, miss and overlap, and the data and input are generated in shapes
// the conditions do not expect as well: arrays, scalars, nothing at all.
func TestIndexMatchesUnindexedEval(t *testing.T) {
	ctx := t.Context()

	for seed := range 4 {
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			rnd := rand.New(rand.NewSource(int64(seed)))

			for range 50 {
				policy := randomIndexPolicy(rnd)
				data := randomIndexData(rnd)
				store := inmem.NewFromObject(data)

				pq, err := New(Query("data.test"), Module("test.rego", policy), Store(store)).PrepareForEval(ctx)
				if err != nil {
					t.Fatalf("%v\npolicy:\n%s", err, policy)
				}
				ppq, err := New(Query("data.test.p"), Module("test.rego", policy), Store(store),
					Unknowns([]string{"input.r"})).PrepareForPartial(ctx)
				if err != nil {
					t.Fatalf("%v\npolicy:\n%s", err, policy)
				}

				for range 10 {
					input := randomIndexInput(rnd)

					var evals, partials [2]string
					for i, indexing := range []bool{true, false} {
						rs, err := pq.Eval(ctx, EvalInput(input), EvalRuleIndexing(indexing))
						if err != nil {
							t.Fatalf("%v\npolicy:\n%s", err, policy)
						}
						evals[i] = fmt.Sprint(rs)

						pqs, err := ppq.Partial(ctx, EvalInput(input), EvalRuleIndexing(indexing))
						if err != nil {
							t.Fatalf("%v\npolicy:\n%s", err, policy)
						}
						partials[i] = fmt.Sprint(pqs.Queries, pqs.Support)
					}

					if evals[0] != evals[1] {
						t.Fatalf("indexed %v != unindexed %v\ninput: %v\ndata: %v\npolicy:\n%s",
							evals[0], evals[1], input, data, policy)
					}
					if partials[0] != partials[1] {
						t.Fatalf("partial: indexed %v != unindexed %v\ninput: %v\ndata: %v\npolicy:\n%s",
							partials[0], partials[1], input, data, policy)
					}
				}
			}
		})
	}
}

var (
	indexValues = []string{`"a"`, `"b"`, `1`, `true`}
	indexGroups = []string{"g1", "g2", "g3"}
	indexUsers  = []string{"u1", "u2", "u3"}
)

func randomIndexPolicy(rnd *rand.Rand) string {
	pick := func(xs []string) string { return xs[rnd.Intn(len(xs))] }
	val := func() string { return pick(indexValues) }
	grp := func() string { return pick(indexGroups) }

	conds := []func() string{
		func() string { return "input.x == " + val() },
		func() string { return fmt.Sprintf("input.x in {%s, %s}", val(), val()) },
		func() string { return val() + " in input.xs" },
		func() string { return "input.xs[_] == " + val() },
		func() string { return "some s in input.xs\n\ts == " + val() },
		func() string { return `startswith(input.x, "a")` },
		func() string { return `glob.match("a.*", ["."], input.x)` },
		func() string { return "input.flag" },
		func() string { return "not input.x == " + val() },
		func() string { return fmt.Sprintf("{input.x == %s} or {input.x == %s}", val(), val()) },
		func() string { return fmt.Sprintf("input.x == %s\n\tdata.groups.%s.members[input.u]", val(), grp()) },
		func() string { return fmt.Sprintf("data.groups.%s.members[input.u]", grp()) },
		func() string { return fmt.Sprintf("data.groups.%s.members[input.us[_]]", grp()) },
		func() string {
			return fmt.Sprintf("{data.groups.%s.members[input.u]} or {data.groups.%s.members[input.u]}", grp(), grp())
		},
		func() string {
			return fmt.Sprintf("{data.groups.%s.members[input.u]} or {input.x == %s}", grp(), val())
		},
		func() string { return fmt.Sprintf("k := %q\n\tdata.groups[k].members[input.u]", grp()) },
		func() string { return fmt.Sprintf("k := %q\n\tj := k\n\tdata.groups[j].members[input.u]", grp()) },
		func() string { return fmt.Sprintf("some r in {%s, %s}\n\tinput.x == r", val(), val()) },
		func() string {
			return fmt.Sprintf("some g in {%q, %q}\n\tdata.groups[g].members[input.u]", grp(), grp())
		},
		func() string {
			return fmt.Sprintf("some g in {%q, %q}\n\tdata.groups[g].members[input.us[_]]", grp(), grp())
		},
		func() string {
			return fmt.Sprintf("some g, p in {%q: \"read\", %q: \"write\"}\n\tdata.groups[g].perms[p][input.u]", grp(), grp())
		},
		// The unknown of the partial evaluation, next to a lookup.
		func() string { return fmt.Sprintf("input.r == %s\n\tdata.groups.%s.members[input.u]", val(), grp()) },
	}

	var sb strings.Builder
	sb.WriteString("package test\n\nimport future.keywords.or\n\n")
	for i := range 1 + rnd.Intn(6) {
		fmt.Fprintf(&sb, "p contains %d if {\n\t%s\n}\n\n", i, conds[rnd.Intn(len(conds))]())
	}
	// Definitions agreeing on their value, which evaluation may stop at.
	for range rnd.Intn(3) {
		fmt.Fprintf(&sb, "q if {\n\t%s\n}\n\n", conds[rnd.Intn(len(conds))]())
	}
	return sb.String()
}

func randomIndexData(rnd *rand.Rand) map[string]any {
	users := func() map[string]any {
		m := map[string]any{}
		for _, u := range indexUsers {
			if rnd.Intn(2) == 0 {
				m[u] = true
			}
		}
		return m
	}

	groups := map[string]any{}
	for _, g := range indexGroups {
		switch rnd.Intn(6) {
		case 0:
			continue // no such group
		case 1:
			groups[g] = map[string]any{"members": []any{"u1", "u2"}} // an array: asked for a position
		case 2:
			groups[g] = map[string]any{"members": "u1"} // not a collection
		default:
			groups[g] = map[string]any{
				"members": users(),
				"perms":   map[string]any{"read": users(), "write": users()},
			}
		}
	}
	return map[string]any{"groups": groups}
}

func randomIndexInput(rnd *rand.Rand) map[string]any {
	value := func() any {
		switch rnd.Intn(5) {
		case 0:
			return "a"
		case 1:
			return "b"
		case 2:
			return 1
		case 3:
			return true
		default:
			return "ab"
		}
	}
	user := func() any { return indexUsers[rnd.Intn(len(indexUsers))] }

	input := map[string]any{
		"x":    value(),
		"xs":   []any{value(), value()},
		"u":    user(),
		"us":   []any{user(), user()},
		"flag": rnd.Intn(2) == 0,
	}

	switch rnd.Intn(8) {
	case 0:
		delete(input, "x")
	case 1:
		input["xs"] = map[string]any{"k1": value(), "k2": value()}
	case 2:
		input["xs"] = value()
	case 3:
		delete(input, "u")
	case 4:
		input["u"] = 1
	case 5:
		input["us"] = map[string]any{"k": user()}
	case 6:
		input["us"] = user()
	case 7:
		delete(input, "flag")
	}
	return input
}
