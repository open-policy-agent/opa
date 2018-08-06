// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package query

import (
	"reflect"
	"testing"
)

func TestSplitQueryWords(t *testing.T) {
	for _, test := range []struct {
		q    string
		want []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"hello\\ world", []string{"hello world"}},
		{`"key:value two" and\ more`, []string{"key:value two", "and more"}},
		{`one" two"\ three four`, []string{"one two three", "four"}},
		{`"4'7\""`, []string{`4'7"`}},
	} {
		have := SplitWords(test.q)
		if !reflect.DeepEqual(have, test.want) {
			t.Errorf("splitQueryWords(%q) = %+v, want %+v", test.q, have, test.want)
		}
	}
}
