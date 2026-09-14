// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"hash"
	"io"

	"github.com/open-policy-agent/opa/v1/util"
)

// HashingAlgorithm represents a subset of hashing algorithms implemented in Go
type HashingAlgorithm string

// Supported values for HashingAlgorithm
const (
	MD5       HashingAlgorithm = "MD5"
	SHA1      HashingAlgorithm = "SHA-1"
	SHA224    HashingAlgorithm = "SHA-224"
	SHA256    HashingAlgorithm = "SHA-256"
	SHA384    HashingAlgorithm = "SHA-384"
	SHA512    HashingAlgorithm = "SHA-512"
	SHA512224 HashingAlgorithm = "SHA-512-224"
	SHA512256 HashingAlgorithm = "SHA-512-256"
)

// String returns the string representation of a HashingAlgorithm
func (alg HashingAlgorithm) String() string {
	return string(alg)
}

// SignatureHasher computes a signature digest for a file with (structured or unstructured) data and policy
type SignatureHasher interface {
	HashFile(v any) ([]byte, error)
}

type hasher struct {
	h func() hash.Hash // hash function factory
}

// NewSignatureHasher returns a signature hasher suitable for a particular hashing algorithm
func NewSignatureHasher(alg HashingAlgorithm) (SignatureHasher, error) {
	h := &hasher{}

	switch alg {
	case MD5:
		h.h = md5.New
	case SHA1:
		h.h = sha1.New
	case SHA224:
		h.h = sha256.New224
	case SHA256:
		h.h = sha256.New
	case SHA384:
		h.h = sha512.New384
	case SHA512:
		h.h = sha512.New
	case SHA512224:
		h.h = sha512.New512_224
	case SHA512256:
		h.h = sha512.New512_256
	default:
		return nil, fmt.Errorf("unsupported hashing algorithm: %s", alg)
	}

	return h, nil
}

// HashFile hashes the file content, JSON or binary, both in golang native format.
func (h *hasher) HashFile(v any) ([]byte, error) {
	hf := h.h()
	walk(v, hf, newPrimitiveEncoder())
	return hf.Sum(nil), nil
}

// walk hashes the file content, JSON or binary, both in golang native format.
//
// Computation for unstructured documents is a hash of the document.
//
// Computation for the types of structured JSON document is as follows:
//
// object: Hash {, then each key (in alphabetical order) and digest of the value, then comma (between items) and finally }.
//
// array: Hash [, then digest of the value, then comma (between items) and finally ].
func walk(v any, h io.Writer, pe *primitiveEncoder) {

	switch x := v.(type) {
	case map[string]any:
		_, _ = h.Write([]byte("{"))

		for i, key := range util.KeysSorted(x) {
			if i > 0 {
				_, _ = h.Write([]byte(","))
			}

			_, _ = h.Write(pe.encode(key))
			_, _ = h.Write([]byte(":"))
			walk(x[key], h, pe)
		}

		_, _ = h.Write([]byte("}"))
	case []any:
		_, _ = h.Write([]byte("["))

		for i, e := range x {
			if i > 0 {
				_, _ = h.Write([]byte(","))
			}
			walk(e, h, pe)
		}

		_, _ = h.Write([]byte("]"))
	case []byte:
		_, _ = h.Write(x)
	default:
		_, _ = h.Write(pe.encode(x))
	}
}

// primitiveEncoder reuses its buffer across encode calls.
type primitiveEncoder struct {
	buf *bytes.Buffer
	enc *json.Encoder
}

func newPrimitiveEncoder() *primitiveEncoder {
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	return &primitiveEncoder{buf: buf, enc: enc}
}

// encode's return value aliases the encoder's buffer and is only valid
// until the next call.
func (pe *primitiveEncoder) encode(v any) []byte {
	pe.buf.Reset()
	_ = pe.enc.Encode(v)
	return bytes.Trim(pe.buf.Bytes(), "\n")
}

func encodePrimitive(v any) []byte {
	return newPrimitiveEncoder().encode(v)
}
