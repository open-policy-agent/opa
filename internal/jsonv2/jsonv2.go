// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.27

package jsonv2

// This migration must not change OPA's JSON output, but json.Marshal defaults
// to v2 semantics (no HTML escaping, non-deterministic map key order). So
// every entry point into v2 here must establish v1 options (see
// [jsonv1.DefaultOptionsV1]); nested encodes inherit them from the caller's
// encoder. MarshalMarshalerTo is the only such entry point today, but that's
// incidental — any new json.Marshal, json.MarshalWrite, or jsontext.NewEncoder
// added here must do the same.

import (
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
)

// WriteMarshalerToArray writes the JSON array of items to the encoder.
func WriteMarshalerToArray[T json.MarshalerTo](e *jsontext.Encoder, items []T) error {
	e.WriteToken(jsontext.BeginArray)
	for _, item := range items {
		if err := item.MarshalJSONTo(e); err != nil {
			return err
		}
	}
	return e.WriteToken(jsontext.EndArray)
}

// WriteField writes the object member name and then v's JSON encoding, so that
// the member is written and checked in one statement. A nil v is written as
// JSON null rather than dispatched to MarshalJSONTo: v1's reflection-based
// encoder already does this for a nil pointer, so writing null here keeps
// output identical to v1, rather than panicking on the types whose
// MarshalJSONTo assumes a non-nil receiver.
func WriteField[T json.MarshalerTo](e *jsontext.Encoder, name string, v T) error {
	e.WriteToken(jsontext.String(name))
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return e.WriteToken(jsontext.Null)
	}
	return v.MarshalJSONTo(e)
}

// WriteFieldArray writes the object member name and then the JSON array of items.
func WriteFieldArray[T json.MarshalerTo](e *jsontext.Encoder, name string, items []T) error {
	e.WriteToken(jsontext.String(name))
	return WriteMarshalerToArray(e, items)
}

// WriteFieldValue is [WriteField] for values that don't implement [json.MarshalerTo].
func WriteFieldValue(e *jsontext.Encoder, name string, v any) error {
	e.WriteToken(jsontext.String(name))
	return json.MarshalEncode(e, v)
}

// WriteMarshalerToArrayOrNull is [WriteMarshalerToArray] but writes null for a nil
// slice, as encoding/json v1 does. Types whose pre-1.27 MarshalJSON returns "[]"
// for an empty value must keep using [WriteMarshalerToArray].
func WriteMarshalerToArrayOrNull[T json.MarshalerTo](e *jsontext.Encoder, items []T) error {
	if items == nil {
		return e.WriteToken(jsontext.Null)
	}
	return WriteMarshalerToArray(e, items)
}

// MarshalMarshalerTo provides a MarshalJSON implementation for any type that
// implements json.MarshalerTo. json.Marshal dispatches to MarshalJSONTo, so this
// doesn't recurse; the constraint is what guarantees that at compile time.
//
// This is the entry point into v2 that establishes v1 options, per the
// package-level comment above.
func MarshalMarshalerTo[T json.MarshalerTo](v T) ([]byte, error) {
	return json.Marshal(v, jsonv1.DefaultOptionsV1())
}
