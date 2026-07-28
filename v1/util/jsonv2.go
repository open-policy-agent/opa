// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.27

package util

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
)

type stringOrBytes interface {
	string | []byte
}

// JsonEqual returns true if the canonical JSON encoding of a and b are equal,
// meaning that a and b are compared without regard to things like whitespace,
// key order, etc. For more details, see [jsontext.Value.Canonicalize].
func JsonEqual[A, B stringOrBytes](a A, b B) bool {
	v1, v2 := jsontext.Value(a), jsontext.Value(b)

	v1.Canonicalize()
	v2.Canonicalize()

	return bytes.Equal(v1, v2)
}

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

// MarshalMarshalerTo provides a MarshalJSON implementation for any type that
// implements json.MarshalerTo. json.Marshal dispatches to MarshalJSONTo, so this
// doesn't recurse; the constraint is what guarantees that at compile time.
func MarshalMarshalerTo[T json.MarshalerTo](v T) ([]byte, error) {
	return json.Marshal(v)
}
