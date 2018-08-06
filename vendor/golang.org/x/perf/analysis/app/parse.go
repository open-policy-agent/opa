// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import "strings"

// parseQueryString splits a user-entered query into one or more storage server queries.
// The supported query formats are:
//     prefix | one vs two  - parsed as "prefix", {"one", "two"}
//     prefix one vs two    - parsed as "", {"prefix one", "two"}
//     anything else        - parsed as "", {"anything else"}
// The vs and | separators must not be quoted.
func parseQueryString(q string) (string, []string) {
	var queries []string
	var parts []string
	var prefix string
	quoting := false
	for r := 0; r < len(q); {
		switch c := q[r]; {
		case c == '"' && quoting:
			quoting = false
			r++
		case quoting:
			if c == '\\' {
				r++
			}
			r++
		case c == '"':
			quoting = true
			r++
		case c == ' ', c == '\t':
			switch part := q[:r]; {
			case part == "|" && prefix == "":
				prefix = strings.Join(parts, " ")
				parts = nil
			case part == "vs":
				queries = append(queries, strings.Join(parts, " "))
				parts = nil
			default:
				parts = append(parts, part)
			}
			q = q[r+1:]
			r = 0
		default:
			if c == '\\' {
				r++
			}
			r++
		}
	}
	if len(q) > 0 {
		parts = append(parts, q)
	}
	if len(parts) > 0 {
		queries = append(queries, strings.Join(parts, " "))
	}
	return prefix, queries
}
