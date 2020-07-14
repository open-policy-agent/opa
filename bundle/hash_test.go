// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestHashFile(t *testing.T) {

	mapInput := map[string]interface{}{
		"key1": []interface{}{
			"element1",
			"element2",
		},
		"key2": map[string]interface{}{
			"a": 0,
			"b": 1,
			"c": json.Number("123.45678911111111111111111111111111111111111111111111111"),
		},
	}

	arrayInput := []interface{}{
		[]string{"foo", "bar"},
		mapInput,
		`package example`,
		[]string{"$", "α", "©", "™"},
	}

	tests := map[string]struct {
		input     interface{}
		algorithm HashingAlgorithm
	}{
		"map":                    {mapInput, SHA256},
		"array":                  {arrayInput, MD5},
		"string":                 {"abc", SHA256},
		"string_with_html_chars": {"<foo></foo>", SHA256},
		"null":                   {`null`, SHA512},
		"bool":                   {false, SHA256},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			h, _ := NewSignatureHasher(tc.algorithm)

			// compute hash from the raw bytes
			a := encodePrimitive(tc.input)
			hash := h.(*hasher).h()
			hash.Write(a)
			d1 := hash.Sum(nil)

			// compute hash on the input
			d2, err := h.(*hasher).HashFile(tc.input)
			if err != nil {
				t.Fatalf("Unexpected error %v", err)
			}

			if !bytes.Equal(d1, d2) {
				t.Fatalf("Digests are not equal. Expected: %x but got: %x", d1, d2)
			}
		})
	}
}

func TestHashFileBytes(t *testing.T) {

	mapInput := map[string]interface{}{
		"key1": []interface{}{
			"element1",
			"element2",
		},
		"key2": map[string]interface{}{
			"a": 0,
			"b": 1,
			"c": json.Number("123.45678911111111111111111111111111111111111111111111111"),
		},
	}

	arrayInput := []interface{}{
		[]string{"foo", "bar"},
		mapInput,
		`package example`,
		[]string{"$", "α", "©", "™"},
	}

	arrayBytes, _ := json.Marshal(arrayInput)
	mapBytes, _ := json.Marshal(mapInput)

	tests := map[string]struct {
		input     []byte
		algorithm HashingAlgorithm
	}{
		"map_byte_array":   {mapBytes, SHA256},
		"array_byte_array": {arrayBytes, MD5},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			h, _ := NewSignatureHasher(tc.algorithm)

			// compute hash from the raw bytes
			hash := h.(*hasher).h()
			hash.Write(tc.input)
			d1 := hash.Sum(nil)

			// compute hash on the input
			d2, err := h.(*hasher).HashFile(tc.input)
			if err != nil {
				t.Fatalf("Unexpected error %v", err)
			}

			if !bytes.Equal(d1, d2) {
				t.Fatalf("Digests are not equal. Expected: %x but got: %x", d1, d2)
			}
		})
	}
}
