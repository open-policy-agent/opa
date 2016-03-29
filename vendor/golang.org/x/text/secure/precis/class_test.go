// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package precis

import (
	"testing"

	"golang.org/x/text/runes"
)

// Compile-time regression test to ensure that Class is a Set
var _ runes.Set = (*class)(nil)

// Ensure that certain characters are (or are not) in the identifer class.
func TestClassContains(t *testing.T) {
	tests := []struct {
		class      *class
		allowed    []rune
		disallowed []rune
	}{
		{
			class:      identifier,
			allowed:    []rune("Aa0\u0021\u007e\u00df\u3007"),
			disallowed: []rune("\u2150\u2100\u2200\u3164\u2190\u2600\u303b"),
		},
		{
			class:      freeform,
			allowed:    []rune("Aa0\u0021\u007e\u00df\u3007 \u2150\u2100\u2200\u2190\u2600"),
			disallowed: []rune("\u3164\u303b"),
		},
	}

	for _, rt := range tests {
		for _, r := range rt.allowed {
			if !rt.class.Contains(r) {
				t.Errorf("Class %d should contain \"%U\"", rt.class, r)
			}
		}
		for _, r := range rt.disallowed {
			if rt.class.Contains(r) {
				t.Errorf("Class %d should not contain \"%U\"", rt.class, r)
			}
		}
	}
}
