// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"bytes"
	"testing"
)

func TestParser_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note         string
		input        string
		expStmtCount int
	}{
		{
			note: "v0",
			input: `package test
p[x] { 
	c = ["a", "b", "c"][i] 
}`,
			expStmtCount: 2, //package, p
		},
		{
			note: "v1",
			input: `package test
p contains x if { 
	c = ["a", "b", "c"][i] 
}`,
			// v1 Keywords are not recognized, and interpreted as individual statements
			expStmtCount: 5, //package, p, contains, x, if
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			parser := NewParser().
				WithFilename("test.rego").
				WithReader(bytes.NewBufferString(tc.input))
			stmts, _, err := parser.Parse()
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if len(stmts) != tc.expStmtCount {
				t.Fatalf("Expected %d statements but got %d:\n\n%v", tc.expStmtCount, len(stmts), stmts)
			}
		})
	}
}
