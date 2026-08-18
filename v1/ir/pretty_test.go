// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ir

import (
	"bytes"
	"testing"
)

func TestPrettyRenderStmt(t *testing.T) {
	stmt := &ReturnLocalStmt{Source: Local(1)}

	stmt.SetLocation(0, 1, 1, "test.rego", []byte("p = 1"))

	var buf bytes.Buffer
	if err := Pretty(&buf, stmt); err != nil {
		t.Fatal(err)
	}

	want := "*ir.ReturnLocalStmt &{Source:Local<1> Location:{File:0 Col:1 Row:1 EndCol:6 EndRow:1 Text:p = 1 file:test.rego}}\n"
	got := buf.String()
	if got != want {
		t.Errorf("unexpected output:\nwant: %q\ngot:  %q", want, got)
	}
}
