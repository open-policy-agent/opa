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
	"github.com/lestrrat-go/jwx/v3/jws/jwsbb"
	"github.com/open-policy-agent/opa/v1/util"
)

// parseVerificationKey converts a string key to the appropriate type for jws.Verify
func parseVerificationKey(keyData string, alg jwa.SignatureAlgorithm) (any, error) {
	// For HMAC algorithms, return the key as bytes
	if alg == jwa.HS256() || alg == jwa.HS384() || alg == jwa.HS512() {
		return []byte(keyData), nil
	}

	// For RSA/ECDSA algorithms, try to parse as PEM first
	block, _ := pem.Decode([]byte(keyData))
	if block != nil {
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
		}
	}

	return nil, errors.New("failed to parse PEM block containing the key")
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
	tokbytes := []byte(token)
	hdrb64, payloadb64, signatureb64, err := jwsbb.SplitCompact(tokbytes)
	if err != nil {
		return nil, fmt.Errorf("failed to split compact JWT: %w", err)
	}

	// check for the id of the key to use for JWT signature verification
	// first in the OPA config. If not found, then check the JWT kid.
	keyID := bvc.KeyID
	if keyID == "" {
		// Use jwsbb.Header to access into the "kid" header field, which we will
		// use to determine the key to use for verification.
		hdr := jwsbb.HeaderParseCompact(hdrb64)
		v, err := jwsbb.HeaderGetString(hdr, "kid")
		switch {
		case err == nil:
			// err == nils means we found the key ID in the header
			keyID = v
		case errors.Is(err, jwsbb.ErrHeaderNotFound()):
			// no "kid" in the header. no op.
		default:
			// some other error occurred while trying to extract the key ID
			return nil, fmt.Errorf("failed to extract key ID from headers: %w", err)
		}
	}

	// Because we want to fallback to ds.KeyID when we can't find the
	// keyID, we need to parse the payload here already.
	//
	// (lestrrat) Whoa, you're going to trust the payload before you
	// verify the signature? Even if it's for backwrds compatibility,
	// Is this OK?
	decoder := base64.RawURLEncoding
	payload := make([]byte, decoder.DecodedLen(len(payloadb64)))
	if _, err := decoder.Decode(payload, payloadb64); err != nil {
		return nil, fmt.Errorf("failed to base64 decode JWT payload: %w", err)
	}

	var ds DecodedSignature
	if err := json.Unmarshal(payload, &ds); err != nil {
		return nil, err
	}

	// If header has no key id, check the deprecated key claim.
	if keyID == "" {
		keyID = ds.KeyID
	}

	// If we still don't have a keyID, we cannot proceed
	if keyID == "" {
		return nil, errors.New("verification key ID is empty")
	}

	// now that we have the keyID, fetch the actual key
	keyConfig, err := bvc.GetPublicKey(keyID)
	if err != nil {
		return nil, err
	}

	alg, ok := jwa.LookupSignatureAlgorithm(keyConfig.Algorithm)
	if !ok {
		return nil, fmt.Errorf("unknown signature algorithm: %s", keyConfig.Algorithm)
	}

	// Parse the key into the appropriate type
	parsedKey, err := parseVerificationKey(keyConfig.Key, alg)
	if err != nil {
		return nil, err
	}

	signature := make([]byte, decoder.DecodedLen(len(signatureb64)))
	if _, err = decoder.Decode(signature, signatureb64); err != nil {
		return nil, fmt.Errorf("failed to base64 decode JWT signature: %w", err)
	}

	signbuf := make([]byte, len(hdrb64)+1+len(payloadb64))
	copy(signbuf, hdrb64)
	signbuf[len(hdrb64)] = '.'
	copy(signbuf[len(hdrb64)+1:], payloadb64)

	if err := jwsbb.Verify(parsedKey, alg.String(), signbuf, signature); err != nil {
		return nil, fmt.Errorf("failed to verify JWT signature: %w", err)
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
