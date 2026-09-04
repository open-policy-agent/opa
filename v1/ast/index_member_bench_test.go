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

// BenchmarkBuildMembershipIndex is the cost of putting `input.x in [k
// literals]` into the index. Each element is a value the rule may reach
// input.x by, and refindices.insert rescans the rule's indices on every one, so
// recording k of them used to be O(k^2):
//
//	         rescanning    recorded together
//	  100        121 us               20 us
//	 1000       11.8 ms             0.22 ms
//	10000       1128 ms             2.69 ms
//
// A decade of k cost 97x rescanning and costs 11x now. strings.any_prefix_match
// had this fixed for its own base collection when it landed; insertAll is that
// fix, shared.
func BenchmarkBuildMembershipIndex(b *testing.B) {
	for _, k := range []int{100, 1000, 10000} {
		b.Run(strconv.Itoa(k), func(b *testing.B) {
			c := MustCompileModules(map[string]string{"test.rego": membershipPolicy(k)})
			rules := c.Modules["test.rego"].Rules

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				index := newBaseDocEqIndex(func(Ref) bool { return false })
				if !index.Build(rules) {
					b.Fatal("expected index build to succeed")
				}
			}
		})
	}
}

// membershipPolicy is one rule reaching input.x by k values, with a second
// constraint below so that the rule has a path to lose if the k values are
// mishandled (see refindices.alternated).
func membershipPolicy(k int) string {
	var sb strings.Builder

	sb.WriteString("package test\n\np if {\n\tinput.x in [")
	for i := range k {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%d", i)
	}
	sb.WriteString("]\n\tinput.y == 1\n}\n")

	return sb.String()
}
