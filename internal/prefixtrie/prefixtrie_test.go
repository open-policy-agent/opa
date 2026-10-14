// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package prefixtrie

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

var insertCases = []struct {
	note string
	keys []string
}{
	{note: "disjoint", keys: []string{"a", "b", "c"}},
	{note: "nested, shortest first", keys: []string{"a", "ab", "abc"}},
	{note: "nested, longest first", keys: []string{"abc", "ab", "a"}},
	{note: "split an edge in the middle", keys: []string{"abcdef", "abcxyz"}},
	{note: "split an edge twice", keys: []string{"abcdef", "abcxyz", "abq"}},
	{note: "split at the very first byte", keys: []string{"abc", "xbc"}},
	{note: "empty key among others", keys: []string{"", "a", "ab"}},
	{note: "duplicate insertions", keys: []string{"ab", "ab", "ab"}},
	{note: "shared stem", keys: []string{"/api/v1", "/api/v2", "/api", "/admin", "/a"}},
	{note: "high bytes", keys: []string{"\xff\x00", "\xff\x01", "\x00"}},
}

func TestInsertAndPrefixesOf(t *testing.T) {
	for _, tc := range insertCases {
		t.Run(tc.note, func(t *testing.T) {
			trie := &Trie[string]{}
			values := map[string]*string{}
			for _, k := range tc.keys {
				v := trie.Insert(k)
				if v == nil {
					t.Fatalf("Insert(%q) returned nil", k)
				}
				if prev, ok := values[k]; ok && prev != v {
					t.Errorf("Insert(%q) returned a second value for the same key", k)
				}
				*v = k
				values[k] = v
			}

			want := slices.Compact(slices.Sorted(slices.Values(tc.keys)))
			var got []string
			for _, e := range trie.Entries() {
				got = append(got, e.Key)
				if values[e.Key] != e.Value {
					t.Errorf("Entries() reported a different value for %q", e.Key)
				}
			}
			if !slices.Equal(got, want) {
				t.Errorf("expected entries %q, got %q", want, got)
			}

			for _, s := range probes(tc.keys) {
				var exp []string
				for _, k := range want {
					if strings.HasPrefix(s, k) {
						exp = append(exp, k)
					}
				}

				var act []string
				if err := trie.PrefixesOf(s, func(v *string) error {
					act = append(act, *v)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				// Shortest first, which is sorted order for prefixes of one string.
				if !slices.Equal(act, exp) {
					t.Errorf("PrefixesOf(%q): expected %q, got %q", s, exp, act)
				}
				if got := trie.HasPrefixOf(s); got != (len(exp) > 0) {
					t.Errorf("HasPrefixOf(%q): expected %v, got %v", s, len(exp) > 0, got)
				}
			}
		})
	}
}

func TestSuffixesOf(t *testing.T) {
	for _, tc := range insertCases {
		t.Run(tc.note, func(t *testing.T) {
			trie := &Trie[string]{}
			for _, k := range tc.keys {
				*trie.Insert(Reverse(k)) = k
			}

			want := slices.Compact(slices.Sorted(slices.Values(tc.keys)))
			var reversed []string
			for _, k := range tc.keys {
				reversed = append(reversed, Reverse(k))
			}

			for _, s := range append(probes(tc.keys), probes(reversed)...) {
				var exp []string
				for _, k := range want {
					if strings.HasSuffix(s, k) {
						exp = append(exp, k)
					}
				}

				var act []string
				if err := trie.SuffixesOf(s, func(v *string) error {
					act = append(act, *v)
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				slices.Sort(act)
				if !slices.Equal(act, exp) {
					t.Errorf("SuffixesOf(%q): expected %q, got %q", s, exp, act)
				}
				if got := trie.HasSuffixOf(s); got != (len(exp) > 0) {
					t.Errorf("HasSuffixOf(%q): expected %v, got %v", s, len(exp) > 0, got)
				}
			}
		})
	}
}

func TestHasPrefixOfAndHasSuffixOfMatchBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	randStrings := func() []string {
		strs := make([]string, rng.IntN(8))
		for i := range strs {
			b := make([]byte, rng.IntN(5))
			for j := range b {
				b[j] = "abc"[rng.IntN(3)]
			}
			strs[i] = string(b)
		}
		return strs
	}

	for range 10000 {
		keys, probes := randStrings(), randStrings()
		prefixes, suffixes := &Trie[struct{}]{}, &Trie[struct{}]{}
		for _, k := range keys {
			prefixes.Insert(k)
			suffixes.Insert(Reverse(k))
		}
		for _, s := range probes {
			expPrefix := slices.ContainsFunc(keys, func(k string) bool { return strings.HasPrefix(s, k) })
			if got := prefixes.HasPrefixOf(s); got != expPrefix {
				t.Fatalf("keys %q: HasPrefixOf(%q): expected %v, got %v", keys, s, expPrefix, got)
			}
			expSuffix := slices.ContainsFunc(keys, func(k string) bool { return strings.HasSuffix(s, k) })
			if got := suffixes.HasSuffixOf(s); got != expSuffix {
				t.Fatalf("keys %q: HasSuffixOf(%q): expected %v, got %v", keys, s, expSuffix, got)
			}
		}
	}
}

func TestEmptyTrie(t *testing.T) {
	var trie Trie[struct{}]
	if trie.HasPrefixOf("") || trie.HasPrefixOf("a") || trie.HasSuffixOf("a") {
		t.Error("expected an empty trie to match nothing")
	}
	if entries := trie.Entries(); len(entries) != 0 {
		t.Errorf("expected no entries, got %v", entries)
	}

	var nilTrie *Trie[struct{}]
	nilTrie.Compact()
	if err := nilTrie.Values(func(*struct{}) error { return errors.New("visited") }); err != nil {
		t.Error("expected a nil trie to have no values")
	}
}

func TestValues(t *testing.T) {
	trie := &Trie[string]{}
	for _, k := range []string{"/api/v2", "/api", "", "/admin", "/api/v1"} {
		*trie.Insert(k) = k
	}

	var got []string
	if err := trie.Values(func(v *string) error {
		got = append(got, *v)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if exp := []string{"", "/admin", "/api", "/api/v1", "/api/v2"}; !slices.Equal(got, exp) {
		t.Errorf("expected %q, got %q", exp, got)
	}

	stop := errors.New("stop")
	var n int
	err := trie.Values(func(*string) error {
		n++
		if n == 2 {
			return stop
		}
		return nil
	})
	if !errors.Is(err, stop) || n != 2 {
		t.Errorf("expected Values to stop after the second value with %v, got n=%d err=%v", stop, n, err)
	}
}

func TestCompaction(t *testing.T) {
	const n = 10000
	trie := &Trie[struct{}]{}
	for i := range n {
		trie.Insert(fmt.Sprintf("/some/quite/long/shared/stem/%06d", i))
	}
	trie.Compact()

	if got := len(trie.Entries()); got != n {
		t.Errorf("expected %d keys, got %d", n, got)
	}

	// A compressed trie holds at most 2p-1 nodes; a byte-per-node one would
	// hold ~30 per key here.
	if nodes, limit := countNodes(trie), 2*n; nodes > limit {
		t.Errorf("expected at most %d nodes, got %d", limit, nodes)
	}

	var spare func(*Trie[struct{}]) int
	spare = func(p *Trie[struct{}]) int {
		s := cap(p.edges) - len(p.edges)
		for _, e := range p.edges {
			if e.node != nil {
				s += spare(e.node)
			}
		}
		return s
	}
	if s := spare(trie); s != 0 {
		t.Errorf("expected no spare edge capacity after Compact, got %d", s)
	}

	if !trie.HasPrefixOf("/some/quite/long/shared/stem/000042/x") {
		t.Error("expected a lookup to still match after Compact")
	}
}

func countNodes[V any](p *Trie[V]) int {
	if p == nil {
		return 0
	}
	n := 1
	for _, e := range p.edges {
		n += countNodes(e.node)
	}
	return n
}

// probes returns strings worth looking up for a set of keys: each key itself,
// one byte short of it, one byte past it, and a few misses.
func probes(keys []string) []string {
	probes := []string{"", "z", "zzzzzz"}
	for _, k := range keys {
		probes = append(probes, k, k+"x", k+"xyz", "x"+k)
		if len(k) > 0 {
			probes = append(probes, k[:len(k)-1], k[1:])
		}
	}
	return probes
}
