// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import (
	"slices"
	"strings"
	"testing"
)

func TestSpecsFromStruct(t *testing.T) {
	type leaf struct {
		Buckets []float64 `json:"buckets,omitempty"`
	}
	type mid struct {
		Leaf   *leaf  `json:"leaf,omitempty"`
		Name   string `json:"name"`
		hidden string //nolint:unused // unexported: must be ignored
		Skip   string `json:"-"`
	}
	type namedEntry struct {
		Value string `json:"value"`
	}
	type embedded struct {
		Shared string `json:"shared"`
	}
	type root struct {
		embedded
		Mid     mid                    `json:"mid"`
		Entries map[string]*namedEntry `json:"entries,omitempty"`
		List    []namedEntry           `json:"list,omitempty"`
		Raw     string                 `json:"raw,omitempty"`
	}

	specs := SpecsFromStruct[root]("top")

	// Collect into a lookup keyed by the dotted pattern for easy assertion.
	got := map[string][]string{}
	for _, s := range specs {
		got[strings.Join(s.Pattern, ".")] = s.Keys
	}

	want := map[string][]string{
		"top":           {"shared", "mid", "entries", "list", "raw"}, // embedded flattened, "-" skipped, unexported skipped
		"top.mid":       {"leaf", "name"},
		"top.mid.leaf":  {"buckets"},
		"top.entries.*": {"value"},
		"top.list.*":    {"value"},
	}

	if len(got) != len(want) {
		t.Fatalf("expected %d specs, got %d: %v", len(want), len(got), got)
	}
	for pat, wantKeys := range want {
		gotKeys, ok := got[pat]
		if !ok {
			t.Errorf("missing spec for pattern %q", pat)
			continue
		}
		if !equalUnordered(gotKeys, wantKeys) {
			t.Errorf("pattern %q: want keys %v, got %v", pat, wantKeys, gotKeys)
		}
	}
}

func equalUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac, bc := slices.Clone(a), slices.Clone(b)
	slices.Sort(ac)
	slices.Sort(bc)
	return slices.Equal(ac, bc)
}
