// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.27

package jsonv2

import (
	"bytes"
	"encoding/json/jsontext"
	"strings"
	"testing"
)

// widget's MarshalJSONTo assumes a non-nil receiver, mirroring the ast
// package's marshalers, to check that WriteField only calls it when v is
// non-nil.
type widget struct {
	Name string
}

func (w *widget) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)
	e.WriteToken(jsontext.String("name"))
	e.WriteToken(jsontext.String(w.Name))
	return e.WriteToken(jsontext.EndObject)
}

func TestWriteFieldNilPointer(t *testing.T) {
	var buf bytes.Buffer
	enc := jsontext.NewEncoder(&buf)

	enc.WriteToken(jsontext.BeginObject)
	if err := WriteField(enc, "widget", (*widget)(nil)); err != nil {
		t.Fatalf("WriteField with nil pointer panicked or errored: %v", err)
	}
	enc.WriteToken(jsontext.EndObject)

	if got, want := strings.TrimSpace(buf.String()), `{"widget":null}`; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestWriteFieldNonNilPointer(t *testing.T) {
	var buf bytes.Buffer
	enc := jsontext.NewEncoder(&buf)

	enc.WriteToken(jsontext.BeginObject)
	if err := WriteField(enc, "widget", &widget{Name: "foo"}); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	enc.WriteToken(jsontext.EndObject)

	if got, want := strings.TrimSpace(buf.String()), `{"widget":{"name":"foo"}}`; got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}
