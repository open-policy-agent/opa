// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"encoding/json"
	"io"

	v1 "github.com/open-policy-agent/opa/v1/util"
)

// UnmarshalJSON parses the JSON encoded data and stores the result in the value
// pointed to by x.
//
// This function is intended to be used in place of the standard json.Marshal
// function when json.Number is required.
func UnmarshalJSON(bs []byte, x interface{}) error {
	return v1.UnmarshalJSON(bs, x)
}

// NewJSONDecoder returns a new decoder that reads from r.
//
// This function is intended to be used in place of the standard json.NewDecoder
// when json.Number is required.
func NewJSONDecoder(r io.Reader) *json.Decoder {
	return v1.NewJSONDecoder(r)
}

// MustUnmarshalJSON parse the JSON encoded data and returns the result.
//
// If the data cannot be decoded, this function will panic. This function is for
// test purposes.
func MustUnmarshalJSON(bs []byte) interface{} {
	return v1.MustUnmarshalJSON(bs)
}

// MustMarshalJSON returns the JSON encoding of x
//
// If the data cannot be encoded, this function will panic. This function is for
// test purposes.
func MustMarshalJSON(x interface{}) []byte {
	return v1.MustMarshalJSON(x)
}

// RoundTrip encodes to JSON, and decodes the result again.
//
// Thereby, it is converting its argument to the representation expected by
// rego.Input and inmem's Write operations. Works with both references and
// values.
func RoundTrip(x *interface{}) error {
	return v1.RoundTrip(x)
}

// Reference returns a pointer to its argument unless the argument already is
// a pointer. If the argument is **t, or ***t, etc, it will return *t.
//
// Used for preparing Go types (including pointers to structs) into values to be
// put through util.RoundTrip().
func Reference(x interface{}) *interface{} {
	return v1.Reference(x)
}

// Unmarshal decodes a YAML, JSON or JSON extension value into the specified type.
func Unmarshal(bs []byte, v interface{}) error {
	return v1.Unmarshal(bs, v)
}
