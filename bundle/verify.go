// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle provide helpers that assist in the bundle signature verification process
package bundle

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
	"github.com/open-policy-agent/opa/internal/jwx/jws"
	"github.com/open-policy-agent/opa/internal/jwx/jws/verify"
	"github.com/open-policy-agent/opa/util"
)

// VerifyBundleSignature verifies the bundle signature using the given public keys or secret.
// If a signature is verified, it keeps track of the files specified in the JWT payload
func VerifyBundleSignature(sc SignaturesConfig, bvc *VerificationConfig) (map[string]FileInfo, error) {
	files := map[string]FileInfo{}

	if len(sc.Signatures) == 0 {
		return files, fmt.Errorf(".signatures.json: missing JWT (expected exactly one)")
	}

	if len(sc.Signatures) > 1 {
		return files, fmt.Errorf(".signatures.json: multiple JWTs not supported (expected exactly one)")
	}

	for _, token := range sc.Signatures {

		// decode JWT to check if the payload specifies the key to use for JWT signature verification
		parts, err := jws.SplitCompact(token)
		if err != nil {
			return files, err
		}

		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return files, err
		}

		var buf bytes.Buffer
		buf.Write(payload)

		var jpl DecodedSignature
		if err := util.UnmarshalJSON(buf.Bytes(), &jpl); err != nil {
			return files, err
		}

		// verify the JWT signature
		err = verifyJWTSignature(token, jpl, bvc)
		if err != nil {
			return files, err
		}

		// build the map of file names to their info
		for _, file := range jpl.Files {
			files[file.Name] = file
		}
	}
	return files, nil
}

func verifyJWTSignature(token string, payload DecodedSignature, bvc *VerificationConfig) error {
	// check for the id of the key to use for JWT signature verification
	// first in the OPA config. If not found, then check the JWT payload
	keyID := bvc.KeyID
	if keyID == "" {
		keyID = payload.KeyID
	}

	if keyID == "" {
		return fmt.Errorf("verification key ID is empty")
	}

	// now that we have the keyID, fetch the actual key
	keyConfig, err := bvc.GetPublicKey(keyID)
	if err != nil {
		return err
	}

	// verify JWT signature
	alg := jwa.SignatureAlgorithm(keyConfig.Algorithm)
	key, err := verify.GetSigningKey(keyConfig.Key, alg)
	if err != nil {
		return err
	}

	_, err = jws.Verify([]byte(token), alg, key)
	if err != nil {
		return err
	}

	// verify the scope
	scope := bvc.Scope
	if scope == "" {
		scope = keyConfig.Scope
	}

	if payload.Scope != scope {
		return fmt.Errorf("scope mismatch")
	}
	return nil
}

// VerifyBundleFile verifies the hash of a file in the bundle matches to that provided in the bundle's signature
func VerifyBundleFile(path string, data bytes.Buffer, files map[string]FileInfo) error {
	var file FileInfo
	var ok bool

	if file, ok = files[path]; !ok {
		return fmt.Errorf("file %v not included in bundle signature", path)
	}

	if file.Algorithm == "" {
		return fmt.Errorf("no hashing algorithm provided for file %v", path)
	}

	hash, err := NewSignatureHasher(HashingAlgorithm(file.Algorithm))
	if err != nil {
		return err
	}

	// hash the file content
	// For unstructured files, hash the byte stream of the file
	// For structured files, read the byte stream and parse into a JSON structure;
	// then recursively order the fields of all objects alphabetically and then apply
	// the hash function to result to compute the hash. This ensures that the digital signature is
	// independent of whitespace and other non-semantic JSON features.
	var value interface{}
	if IsStructuredDoc(path) {
		err := util.Unmarshal(data.Bytes(), &value)
		if err != nil {
			return err
		}
	} else {
		value = data.Bytes()
	}

	bs, err := hash.HashFile(value)
	if err != nil {
		return err
	}

	// compare file hash with same file in the JWT payloads
	fb, err := hex.DecodeString(file.Hash)
	if err != nil {
		return err
	}

	if !bytes.Equal(fb, bs) {
		return fmt.Errorf("%v: digest mismatch (want: %x, got: %x)", path, fb, bs)
	}

	delete(files, path)
	return nil
}
