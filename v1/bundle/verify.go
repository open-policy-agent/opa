// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle provide helpers that assist in the bundle signature verification process
package bundle

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/open-policy-agent/opa/v1/util"
)



// parseVerificationKey converts a string key to the appropriate type for jws.Verify
func parseVerificationKey(keyData, algorithm string) (interface{}, error) {
	alg, ok := jwa.LookupSignatureAlgorithm(algorithm)
	if !ok {
		return nil, fmt.Errorf("unknown signature algorithm: %s", algorithm)
	}
	
	// For HMAC algorithms, return the key as bytes
	if alg == jwa.HS256() || alg == jwa.HS384() || alg == jwa.HS512() {
		return []byte(keyData), nil
	}

	// For RSA/ECDSA algorithms, try to parse as PEM first
	if block, _ := pem.Decode([]byte(keyData)); block != nil {
		switch block.Type {
		case "RSA PUBLIC KEY":
			return x509.ParsePKCS1PublicKey(block.Bytes)
		case "PUBLIC KEY":
			return x509.ParsePKIXPublicKey(block.Bytes)
		case "RSA PRIVATE KEY":
			return x509.ParsePKCS1PrivateKey(block.Bytes)
		case "PRIVATE KEY":
			return x509.ParsePKCS8PrivateKey(block.Bytes)
		case "EC PRIVATE KEY":
			return x509.ParseECPrivateKey(block.Bytes)
		case "CERTIFICATE":
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			return cert.PublicKey, nil
		default:
			return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
		}
	}
	
	// If it's not PEM, assume it's raw key data for HMAC
	return []byte(keyData), nil
}

const defaultVerifierID = "_default"

var verifiers map[string]Verifier

// Verifier is the interface expected for implementations that verify bundle signatures.
type Verifier interface {
	VerifyBundleSignature(SignaturesConfig, *VerificationConfig) (map[string]FileInfo, error)
}

// VerifyBundleSignature will retrieve the Verifier implementation based
// on the Plugin specified in SignaturesConfig, and call its implementation
// of VerifyBundleSignature. VerifyBundleSignature verifies the bundle signature
// using the given public keys or secret. If a signature is verified, it keeps
// track of the files specified in the JWT payload
func VerifyBundleSignature(sc SignaturesConfig, bvc *VerificationConfig) (map[string]FileInfo, error) {
	// default implementation does not return a nil for map, so don't
	// do it here either
	files := make(map[string]FileInfo)
	var plugin string
	// for backwards compatibility, check if there is no plugin specified, and use default
	if sc.Plugin == "" {
		plugin = defaultVerifierID
	} else {
		plugin = sc.Plugin
	}
	verifier, err := GetVerifier(plugin)
	if err != nil {
		return files, err
	}
	return verifier.VerifyBundleSignature(sc, bvc)
}

// DefaultVerifier is the default bundle verification implementation. It verifies bundles by checking
// the JWT signature using a locally-accessible public key.
type DefaultVerifier struct{}

// VerifyBundleSignature verifies the bundle signature using the given public keys or secret.
// If a signature is verified, it keeps track of the files specified in the JWT payload
func (*DefaultVerifier) VerifyBundleSignature(sc SignaturesConfig, bvc *VerificationConfig) (map[string]FileInfo, error) {
	files := make(map[string]FileInfo)

	if len(sc.Signatures) == 0 {
		return files, errors.New(".signatures.json: missing JWT (expected exactly one)")
	}

	if len(sc.Signatures) > 1 {
		return files, errors.New(".signatures.json: multiple JWTs not supported (expected exactly one)")
	}

	for _, token := range sc.Signatures {
		payload, err := verifyJWTSignature(token, bvc)
		if err != nil {
			return files, err
		}

		for _, file := range payload.Files {
			files[file.Name] = file
		}
	}
	return files, nil
}

func verifyJWTSignature(token string, bvc *VerificationConfig) (*DecodedSignature, error) {
	// decode JWT to check if the header specifies the key to use and/or if claims have the scope.

	headersPart, payloadPart, _, err := jws.SplitCompact([]byte(token))
	if err != nil {
		return nil, err
	}

	var decodedHeader []byte
	if decodedHeader, err = base64.RawURLEncoding.DecodeString(string(headersPart)); err != nil {
		return nil, fmt.Errorf("failed to base64 decode JWT headers: %w", err)
	}

	var hdr map[string]interface{}
	if err := json.Unmarshal(decodedHeader, &hdr); err != nil {
		return nil, fmt.Errorf("failed to parse JWT headers: %w", err)
	}

	payload, err := base64.RawURLEncoding.DecodeString(string(payloadPart))
	if err != nil {
		return nil, err
	}

	var ds DecodedSignature
	if err := json.Unmarshal(payload, &ds); err != nil {
		return nil, err
	}

	// check for the id of the key to use for JWT signature verification
	// first in the OPA config. If not found, then check the JWT kid.
	keyID := bvc.KeyID
	if keyID == "" {
		if kid, ok := hdr["kid"].(string); ok {
			keyID = kid
		}
	}
	if keyID == "" {
		// If header has no key id, check the deprecated key claim.
		keyID = ds.KeyID
	}

	if keyID == "" {
		return nil, errors.New("verification key ID is empty")
	}

	// now that we have the keyID, fetch the actual key
	keyConfig, err := bvc.GetPublicKey(keyID)
	if err != nil {
		return nil, err
	}

	// verify JWT signature
	alg, ok := jwa.LookupSignatureAlgorithm(keyConfig.Algorithm)
	if !ok {
		return nil, fmt.Errorf("unknown signature algorithm: %s", keyConfig.Algorithm)
	}
	
	// Parse the key into the appropriate type
	parsedKey, err := parseVerificationKey(keyConfig.Key, keyConfig.Algorithm)
	if err != nil {
		return nil, err
	}
	
	_, err = jws.Verify([]byte(token), jws.WithKey(alg, parsedKey))
	if err != nil {
		return nil, err
	}

	// verify the scope
	scope := bvc.Scope
	if scope == "" {
		scope = keyConfig.Scope
	}

	if ds.Scope != scope {
		return nil, errors.New("scope mismatch")
	}
	return &ds, nil
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
	var value any
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

// GetVerifier returns the Verifier registered under the given id
func GetVerifier(id string) (Verifier, error) {
	verifier, ok := verifiers[id]
	if !ok {
		return nil, fmt.Errorf("no verifier exists under id %s", id)
	}
	return verifier, nil
}

// RegisterVerifier registers a Verifier under the given id
func RegisterVerifier(id string, v Verifier) error {
	if id == defaultVerifierID {
		return fmt.Errorf("verifier id %s is reserved, use a different id", id)
	}
	verifiers[id] = v
	return nil
}

func init() {
	verifiers = map[string]Verifier{
		defaultVerifierID: &DefaultVerifier{},
	}
}
