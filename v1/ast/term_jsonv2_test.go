// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.27

package ast

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"strings"
	"testing"
)

// textAppenderValue is a minimal Value that only implements
// encoding.TextAppender (not json.MarshalerTo), to exercise the
// TextAppender fallback branch of marshalValueTo.
type textAppenderValue struct {
	text string
	err  error
}

func (textAppenderValue) Compare(Value) int       { return 0 }
func (textAppenderValue) Find(Ref) (Value, error) { return nil, nil }
func (textAppenderValue) Hash() int               { return 0 }
func (textAppenderValue) IsGround() bool          { return true }
func (v textAppenderValue) String() string        { return v.text }
func (v textAppenderValue) StringLength() int     { return len(v.text) }

func (v textAppenderValue) AppendText(buf []byte) ([]byte, error) {
	if v.err != nil {
		return buf, v.err
	}
	return append(buf, v.text...), nil
}

func TestMarshalValueToTextAppenderError(t *testing.T) {
	wantErr := errors.New("boom")
	v := textAppenderValue{err: wantErr}

	enc := jsontext.NewEncoder(new(bytes.Buffer))
	err := marshalValueTo(enc, v)
	if err == nil {
		t.Fatalf("expected error from AppendText to be propagated, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped error %v, got %v", wantErr, err)
	}
}

func TestMarshalValueToTextAppenderQuoting(t *testing.T) {
	v := textAppenderValue{text: "2026-01-01T00:00:00Z"}

	var sb bytes.Buffer
	enc := jsontext.NewEncoder(&sb)
	if err := marshalValueTo(enc, v); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := strings.TrimSpace(sb.String())
	want := `"2026-01-01T00:00:00Z"`
	if got != want {
		t.Fatalf("expected quoted JSON string %q, got %q", want, got)
	}
}
