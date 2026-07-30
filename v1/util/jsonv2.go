// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.27

package util

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
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
// the member is written and checked in one statement.
func WriteField[T json.MarshalerTo](e *jsontext.Encoder, name string, v T) error {
	e.WriteToken(jsontext.String(name))
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
func MarshalMarshalerTo[T json.MarshalerTo](v T) ([]byte, error) {
	return json.Marshal(v)
}
