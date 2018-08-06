// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package query provides tools for parsing a query.
package query

// SplitWords splits q into words using shell syntax (whitespace
// can be escaped with double quotes or with a backslash).
func SplitWords(q string) []string {
	var words []string
	word := make([]byte, len(q))
	w := 0
	quoting := false
	for r := 0; r < len(q); r++ {
		switch c := q[r]; {
		case c == '"' && quoting:
			quoting = false
		case quoting:
			if c == '\\' {
				r++
			}
			if r < len(q) {
				word[w] = q[r]
				w++
			}
		case c == '"':
			quoting = true
		case c == ' ', c == '\t':
			if w > 0 {
				words = append(words, string(word[:w]))
			}
			w = 0
		case c == '\\':
			r++
			fallthrough
		default:
			if r < len(q) {
				word[w] = q[r]
				w++
			}
		}
	}
	if w > 0 {
		words = append(words, string(word[:w]))
	}
	return words
}
