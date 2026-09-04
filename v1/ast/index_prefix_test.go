// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// selectsWith is selects() with control over the resolver, for the cases where
// a ref has to be unknown or fail.
func selectsWith(t *testing.T, index RuleIndex, resolver testResolver) []string {
	t.Helper()

	res, err := index.Lookup(resolver)
	if err != nil {
		t.Fatal(err)
	}

	values := make([]string, 0, len(res.Rules))
	for _, rule := range res.Rules {
		values = append(values, rule.Head.Value.String())
	}
	slices.Sort(values)
	return values
}

func allRules(t *testing.T, index RuleIndex) []string {
	t.Helper()

	res, err := index.AllRules(testResolver{})
	if err != nil {
		t.Fatal(err)
	}

	values := make([]string, 0, len(res.Rules))
	for _, rule := range res.Rules {
		values = append(values, rule.Head.Value.String())
	}
	slices.Sort(values)
	return values
}

func TestIndexStartsWith(t *testing.T) {
	index := indexFor(t, `package test

p := 1 if startswith(input.path, "/api/")

p := 2 if startswith(input.path, "/api/v2")

p := 3 if startswith(input.path, "/admin")

# Written the way it reads rather than the way the compiler emits it: the
# operand is hoisted to a local either way, and the index has to resolve it
# back to input.path.
p := 4 if {
	path := input.path
	startswith(path, "/")
}

# A prefix of nothing holds for every string.
p := 5 if startswith(input.path, "")

# A different ref is a different level of the trie.
p := 6 if startswith(input.method, "P")
`)

	for _, tc := range []struct {
		note  string
		input string
		exp   []string
	}{
		{
			note:  "one prefix matches, and the ones it extends",
			input: `{"path": "/api/v1/users"}`,
			exp:   []string{"1", "4", "5"},
		},
		{
			note:  "the longer of two overlapping prefixes matches too",
			input: `{"path": "/api/v2/users"}`,
			exp:   []string{"1", "2", "4", "5"},
		},
		{
			note:  "prefixes that share a first byte but diverge",
			input: `{"path": "/admin/x"}`,
			exp:   []string{"3", "4", "5"},
		},
		{
			note:  "the value is exactly a prefix",
			input: `{"path": "/api/"}`,
			exp:   []string{"1", "4", "5"},
		},
		{
			note:  "the value stops one byte short of every prefix",
			input: `{"path": "/"}`,
			exp:   []string{"4", "5"},
		},
		{
			note:  "the empty string only satisfies the empty prefix",
			input: `{"path": ""}`,
			exp:   []string{"5"},
		},
		{
			note:  "no prefix matches",
			input: `{"path": "/health"}`,
			exp:   []string{"4", "5"},
		},
		{
			// startswith on a non-string is a type error, which makes the rule
			// undefined -- the same trade the indexer already makes for
			// glob.match, whose rules are likewise dropped for a value it
			// cannot match.
			note:  "a non-string value satisfies no prefix",
			input: `{"path": 42}`,
			exp:   []string{},
		},
		{
			note:  "an absent ref satisfies no prefix",
			input: `{}`,
			exp:   []string{},
		},
		{
			note:  "prefixes on a second ref are indexed independently",
			input: `{"path": "/health", "method": "POST"}`,
			exp:   []string{"4", "5", "6"},
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			if act := selects(t, index, tc.input); !slices.Equal(act, tc.exp) {
				t.Errorf("expected %v, got %v", tc.exp, act)
			}
		})
	}
}

func TestIndexAnyPrefixMatch(t *testing.T) {
	index := indexFor(t, `package test

p := 1 if strings.any_prefix_match(input.path, ["/api/", "/admin/"])

p := 2 if strings.any_prefix_match(input.path, {"/api/v2", "/internal"})

# A single base string is the same as startswith.
p := 3 if strings.any_prefix_match(input.path, "/api/v2/beta")

# Overlapping with a plain startswith on the same ref.
p := 4 if startswith(input.path, "/admin/")
`)

	for _, tc := range []struct {
		note  string
		input string
		exp   []string
	}{
		{
			note:  "one alternative of an array base matches",
			input: `{"path": "/api/v1"}`,
			exp:   []string{"1"},
		},
		{
			note:  "the other alternative of an array base matches",
			input: `{"path": "/admin/users"}`,
			exp:   []string{"1", "4"},
		},
		{
			note:  "an alternative of a set base matches",
			input: `{"path": "/internal/metrics"}`,
			exp:   []string{"2"},
		},
		{
			note:  "alternatives from different rules match the same value",
			input: `{"path": "/api/v2/beta/x"}`,
			exp:   []string{"1", "2", "3"},
		},
		{
			note:  "no alternative matches",
			input: `{"path": "/public"}`,
			exp:   []string{},
		},
		{
			// strings.any_prefix_match takes a collection of search strings as
			// well as a single one, and holds if any element matches, so the
			// index has to look at every element.
			note:  "a collection of search strings, one of which matches",
			input: `{"path": ["/nope", "/api/v1"]}`,
			exp:   []string{"1"},
		},
		{
			note:  "a collection of search strings, none of which matches",
			input: `{"path": ["/nope", "/nope2"]}`,
			exp:   []string{},
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			if act := selects(t, index, tc.input); !slices.Equal(act, tc.exp) {
				t.Errorf("expected %v, got %v", tc.exp, act)
			}
		})
	}
}

// TestIndexPrefixNotIndexable covers the shapes the indexer has to leave alone.
// Every rule here must come back for every input: a constraint the index cannot
// represent is one it must not act on.
func TestIndexPrefixNotIndexable(t *testing.T) {
	all := []string{"1", "2", "3", "4", "5", "6", "7"}

	index := indexFor(t, `package test

# The result is captured, so a rule the call returns false for still has to run.
p := 1 if {
	x := startswith(input.path, "/api")
	x == true
}

p := 2 if {
	x := strings.any_prefix_match(input.path, ["/api"])
	x == true
}

# The base is not known until the rule runs.
p := 3 if startswith(input.path, input.prefix)

p := 4 if strings.any_prefix_match(input.path, input.prefixes)

# A base collection that is not all strings cannot be split into prefixes, and
# indexing only some of them would exclude rules the rest would have matched.
p := 5 if strings.any_prefix_match(input.path, ["/api", input.other])

# Negation and with-statements are off limits for the indexer as a whole.
p := 6 if not startswith(input.path, "/api")

p := 7 if startswith(input.path, "/api") with input.path as "/api"
`)

	for _, input := range []string{
		`{"path": "/api/v1", "prefix": "/api", "prefixes": ["/api"], "other": "/x"}`,
		`{"path": "/nope", "prefix": "/api", "prefixes": ["/api"], "other": "/x"}`,
		`{"path": 42, "prefix": "/api", "prefixes": ["/api"], "other": "/x"}`,
	} {
		t.Run(input, func(t *testing.T) {
			if act := selects(t, index, input); !slices.Equal(act, all) {
				t.Errorf("expected every rule to be a candidate, got %v", act)
			}
		})
	}
}

// TestIndexPrefixWithOtherConstraints checks that a prefix constraint composes
// with the rest of the index rather than displacing it.
func TestIndexPrefixWithOtherConstraints(t *testing.T) {
	index := indexFor(t, `package test

p := 1 if {
	startswith(input.path, "/api/")
	input.method == "GET"
}

p := 2 if {
	startswith(input.path, "/api/")
	input.method == "POST"
}

# Two constraints on one ref. They are recorded side by side and the trie
# reaches the rule through either, so a value satisfying only one of them
# still selects it -- over-approximating is allowed, dropping the rule is not.
p := 3 if {
	path := input.path
	startswith(path, "/a")
	path == "/admin"
}

p := 4 if input.method == "GET"
`)

	for _, tc := range []struct {
		note  string
		input string
		exp   []string
	}{
		{
			note:  "prefix and scalar both hold",
			input: `{"path": "/api/x", "method": "GET"}`,
			exp:   []string{"1", "3", "4"},
		},
		{
			note:  "the prefix holds but the scalar does not",
			input: `{"path": "/api/x", "method": "DELETE"}`,
			exp:   []string{"3"},
		},
		{
			note:  "the scalar holds but the prefix does not",
			input: `{"path": "/health", "method": "GET"}`,
			exp:   []string{"4"},
		},
		{
			note:  "a rule constrained twice on one ref is selected when both hold",
			input: `{"path": "/admin"}`,
			exp:   []string{"3"},
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			if act := selects(t, index, tc.input); !slices.Equal(act, tc.exp) {
				t.Errorf("expected %v, got %v", tc.exp, act)
			}
		})
	}
}

// TestIndexPrefixUnknownAndAllRules covers the two traversals that ignore the
// input: partial evaluation, where the ref is unknown, and AllRules.
func TestIndexPrefixUnknownAndAllRules(t *testing.T) {
	index := indexFor(t, `package test

p := 1 if startswith(input.path, "/api/")

p := 2 if strings.any_prefix_match(input.path, ["/admin", "/internal"])

p := 3 if input.path == "/health"
`)

	all := []string{"1", "2", "3"}

	t.Run("unknown ref", func(t *testing.T) {
		resolver := testResolver{
			input:       MustParseTerm(`{}`),
			unknownRefs: NewSet(MustParseTerm("input.path")),
		}
		if act := selectsWith(t, index, resolver); !slices.Equal(act, all) {
			t.Errorf("expected every rule to be a candidate, got %v", act)
		}
	})

	t.Run("all rules", func(t *testing.T) {
		if act := allRules(t, index); !slices.Equal(act, all) {
			t.Errorf("expected every rule, got %v", act)
		}
	})
}

// TestIndexPrefixDocumentedShapes mirrors the table in
// docs/docs/policy-performance.md, so the documented answer to "is this
// indexed" stays the one the indexer gives.
func TestIndexPrefixDocumentedShapes(t *testing.T) {
	for _, tc := range []struct {
		body    string
		indexed bool
	}{
		{body: `startswith(input.path, "/api")`, indexed: true},
		{body: `x := input.path; startswith(x, "/api")`, indexed: true},
		{body: `strings.any_prefix_match(input.path, ["/a", "/b"])`, indexed: true},
		{body: `strings.any_prefix_match(input.path, {"/a", "/b"})`, indexed: true},
		{body: `strings.any_prefix_match(input.path, "/api")`, indexed: true},
		{body: `x := input; startswith(x.foo, "a/")`, indexed: true},
		{body: `startswith(input.path, input.prefix)`, indexed: false},
		{body: `strings.any_prefix_match(input.path, input.bases)`, indexed: false},
		{body: `strings.any_prefix_match(input.path, ["/a", input.b])`, indexed: false},
		{body: `some i; startswith(input.path[i], "/api")`, indexed: false},
		{body: `x := startswith(input.path, "/api"); x == true`, indexed: false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			index := indexFor(t, fmt.Sprintf("package test\n\np := 1 if { %s }\n", tc.body))

			// The input defines every ref the rule reads, so nothing but a
			// prefix constraint can exclude the rule -- and no value here
			// starts with any of the prefixes.
			selected := selects(t, index, `{"path": "/zzz", "prefix": "/api", "bases": ["/a"], "foo": "zzz", "b": "/q"}`)

			if indexed := len(selected) == 0; indexed != tc.indexed {
				t.Errorf("expected indexed=%v, got %v (selected %v)", tc.indexed, indexed, selected)
			}
		})
	}
}

// TestIndexPrefixTrieStaysCompressed is the reason for a radix trie rather than
// a node per byte: the node count follows the number of prefixes, not their
// total length.
func TestIndexPrefixTrieStaysCompressed(t *testing.T) {
	const n = 500

	var src strings.Builder
	src.WriteString("package test\n\np := 1 if strings.any_prefix_match(input.path, [")
	for i := range n {
		if i > 0 {
			src.WriteString(", ")
		}
		// Long, mostly-shared prefixes: a byte-per-node trie would need one
		// node per byte of the shared stem plus one per distinguishing suffix.
		fmt.Fprintf(&src, `"/some/quite/long/shared/stem/%06d"`, i)
	}
	src.WriteString("])\n")

	index := indexFor(t, src.String())

	root, ok := index.(*baseDocEqIndex)
	if !ok {
		t.Fatalf("expected a baseDocEqIndex, got %T", index)
	}

	prefixes := root.root.next.prefixes
	if prefixes == nil {
		t.Fatal("expected the prefixes to be indexed")
	}

	if got := len(prefixes.walk()); got != n {
		t.Errorf("expected %d prefixes in the trie, got %d", n, got)
	}

	// A compressed trie holds at most 2p-1 nodes; a byte-per-node one would
	// hold ~30 per prefix here.
	if nodes, max := countPrefixNodes(prefixes), 2*n; nodes > max {
		t.Errorf("expected at most %d prefix trie nodes, got %d", max, nodes)
	}
}

func countPrefixNodes(p *prefixTrie) int {
	if p == nil {
		return 0
	}
	n := 1
	for _, edge := range p.edges {
		n += countPrefixNodes(edge.node)
	}
	return n
}

// TestPrefixTrieInsertAndTraverse exercises the trie directly, including the
// edge splits that only happen for particular insertion orders.
func TestPrefixTrieInsertAndTraverse(t *testing.T) {
	for _, tc := range []struct {
		note     string
		prefixes []string
	}{
		{note: "disjoint", prefixes: []string{"a", "b", "c"}},
		{note: "nested, shortest first", prefixes: []string{"a", "ab", "abc"}},
		{note: "nested, longest first", prefixes: []string{"abc", "ab", "a"}},
		{note: "split an edge in the middle", prefixes: []string{"abcdef", "abcxyz"}},
		{note: "split an edge twice", prefixes: []string{"abcdef", "abcxyz", "abq"}},
		{note: "split at the very first byte", prefixes: []string{"abc", "xbc"}},
		{note: "empty prefix among others", prefixes: []string{"", "a", "ab"}},
		{note: "duplicate insertions", prefixes: []string{"ab", "ab", "ab"}},
		{note: "shared stem", prefixes: []string{"/api/v1", "/api/v2", "/api", "/admin", "/a"}},
		{note: "high bytes", prefixes: []string{"\xff\x00", "\xff\x01", "\x00"}},
	} {
		t.Run(tc.note, func(t *testing.T) {
			trie := &prefixTrie{}
			nodes := map[string]*trieNode{}
			for _, p := range tc.prefixes {
				node := trie.insert(p)
				if node == nil {
					t.Fatalf("insert(%q) returned nil", p)
				}
				if prev, ok := nodes[p]; ok && prev != node {
					t.Errorf("insert(%q) returned a second node for the same prefix", p)
				}
				nodes[p] = node
			}

			// walk() has to report exactly the distinct prefixes inserted.
			want := slices.Compact(slices.Sorted(slices.Values(tc.prefixes)))
			var got []string
			for _, entry := range trie.walk() {
				got = append(got, entry.prefix)
				if nodes[entry.prefix] != entry.node {
					t.Errorf("walk() reported a different node for %q", entry.prefix)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("expected prefixes %q, got %q", want, got)
			}

			// A lookup has to find every inserted prefix that the value starts
			// with, and nothing else. Checked against the naive answer.
			for _, s := range prefixTrieProbes(tc.prefixes) {
				var exp []string
				for _, p := range want {
					if strings.HasPrefix(s, p) {
						exp = append(exp, p)
					}
				}
				slices.Sort(exp)

				act := prefixTrieMatches(t, trie, nodes, s)
				if !slices.Equal(act, exp) {
					t.Errorf("traverse(%q): expected %q, got %q", s, exp, act)
				}
			}
		})
	}
}

// prefixTrieProbes returns strings worth looking up for a set of prefixes: each
// prefix itself, one byte short of it, one byte past it, and a few misses.
func prefixTrieProbes(prefixes []string) []string {
	probes := []string{"", "z", "zzzzzz"}
	for _, p := range prefixes {
		probes = append(probes, p, p+"x", p+"xyz")
		if len(p) > 0 {
			probes = append(probes, p[:len(p)-1])
		}
	}
	return probes
}

// prefixTrieMatches reports which prefixes a traversal of s reaches, by way of
// the rule each prefix's node was given.
func prefixTrieMatches(t *testing.T, trie *prefixTrie, nodes map[string]*trieNode, s string) []string {
	t.Helper()

	// Give each prefix node a rule of its own, so the traversal result names
	// the prefixes it reached.
	byRule := map[*Rule]string{}
	for prefix, node := range nodes {
		node.rules = node.rules[:0]
		rule := &Rule{Head: NewHead(Var("p"), nil, InternedTerm(len(byRule)))}
		byRule[rule] = prefix
		node.append([2]int{len(byRule), 0}, rule)
	}

	tr := newTrieTraversalResult()
	if err := trie.traverse(s, testResolver{input: MustParseTerm(`{}`)}, tr); err != nil {
		t.Fatal(err)
	}

	var matched []string
	for _, pos := range tr.ordering {
		for _, node := range tr.unordered[pos] {
			matched = append(matched, byRule[node.rule])
		}
	}
	slices.Sort(matched)

	return matched
}

// TestIndexPrefixSelectsSameRulesAsEvaluation is the property that matters: the
// index may return rules that turn out not to hold, but never omit one that
// does. Checked against the answer computed by running the prefix checks
// directly.
func TestIndexPrefixSelectsSameRulesAsEvaluation(t *testing.T) {
	prefixes := [][]string{
		{"/api"},
		{"/api/v1"},
		{"/api/v1/users"},
		{"/api/v2"},
		{"/admin", "/api/v1/u"},
		{"/a"},
		{""},
		{"/health", "/metrics"},
		{"/apis"},
		{"/api/v1/users/1"},
	}

	var src strings.Builder
	src.WriteString("package test\n\n")
	for i, ps := range prefixes {
		if len(ps) == 1 {
			fmt.Fprintf(&src, "p := %d if startswith(input.path, %q)\n\n", i, ps[0])
			continue
		}
		fmt.Fprintf(&src, "p := %d if strings.any_prefix_match(input.path, [", i)
		for j, p := range ps {
			if j > 0 {
				src.WriteString(", ")
			}
			fmt.Fprintf(&src, "%q", p)
		}
		src.WriteString("])\n\n")
	}

	index := indexFor(t, src.String())

	for _, path := range []string{
		"", "/", "/a", "/ap", "/api", "/apis", "/api/", "/api/v1", "/api/v1/users",
		"/api/v1/users/1", "/api/v1/users/2", "/api/v2/x", "/admin", "/health",
		"/metrics/x", "/nope",
	} {
		t.Run(path, func(t *testing.T) {
			var exp []string
			for i, ps := range prefixes {
				if slices.ContainsFunc(ps, func(p string) bool { return strings.HasPrefix(path, p) }) {
					exp = append(exp, strconv.Itoa(i))
				}
			}
			slices.Sort(exp)

			act := selects(t, index, fmt.Sprintf(`{"path": %q}`, path))
			for _, want := range exp {
				if !slices.Contains(act, want) {
					t.Errorf("rule %s holds for %q but the index did not select it (got %v)", want, path, act)
				}
			}
		})
	}
}
