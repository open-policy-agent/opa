// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package db

import "testing"

func TestParseWord(t *testing.T) {
	tests := []struct {
		word    string
		want    part
		wantErr bool
	}{
		{"key:value", part{"key", equals, "value", ""}, false},
		{"key>value", part{"key", gt, "value", ""}, false},
		{"key<value", part{"key", lt, "value", ""}, false},
		{"bogus query", part{}, true},
	}
	for _, test := range tests {
		t.Run(test.word, func(t *testing.T) {
			p, err := parseWord(test.word)
			if test.wantErr {
				if err == nil {
					t.Fatalf("have %#v, want error", p)
				}
				return
			}
			if err != nil {
				t.Fatalf("have error %v", err)
			}
			if p != test.want {
				t.Fatalf("parseWord = %#v, want %#v", p, test.want)
			}
			p, err = p.merge(part{p.key, gt, "", ""})
			if err != nil {
				t.Fatalf("failed to merge with noop: %v", err)
			}
			if p != test.want {
				t.Fatalf("merge with noop = %#v, want %#v", p, test.want)
			}
		})
	}
}
