// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle provide helpers that assist in the creating a signed bundle
package bundle

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws"
)

// getSignatureAlgorithm returns the appropriate jwa.SignatureAlgorithm for known algorithms,
// falling back to lookup for unknown ones
func getSignatureAlgorithm(algStr string) (jwa.SignatureAlgorithm, error) {
	switch algStr {
	case "HS256":
		return jwa.HS256(), nil
	case "HS384":
		return jwa.HS384(), nil
	case "HS512":
		return jwa.HS512(), nil
	case "RS256":
		return jwa.RS256(), nil
	case "RS384":
		return jwa.RS384(), nil
	case "RS512":
		return jwa.RS512(), nil
	case "PS256":
		return jwa.PS256(), nil
	case "PS384":
		return jwa.PS384(), nil
	case "PS512":
		return jwa.PS512(), nil
	case "ES256":
		return jwa.ES256(), nil
	case "ES384":
		return jwa.ES384(), nil
	case "ES512":
		return jwa.ES512(), nil
	default:
		// Fall back to lookup for unknown algorithms
		alg, ok := jwa.LookupSignatureAlgorithm(algStr)
		if !ok {
			return jwa.EmptySignatureAlgorithm(), fmt.Errorf("unknown signature algorithm: %s", algStr)
		}
		return alg, nil
	}
}

const defaultSignerID = "_default"

var signers map[string]Signer

// Signer is the interface expected for implementations that generate bundle signatures.
type Signer interface {
	GenerateSignedToken([]FileInfo, *SigningConfig, string) (string, error)
}

// GenerateSignedToken will retrieve the Signer implementation based on the Plugin specified
// in SigningConfig, and call its implementation of GenerateSignedToken. The signer generates
// a signed token given the list of files to be included in the payload and the bundle
// signing config. The keyID if non-empty, represents the value for the "keyid" claim in the token.
func GenerateSignedToken(files []FileInfo, sc *SigningConfig, keyID string) (string, error) {
	var plugin string
	// for backwards compatibility, check if there is no plugin specified, and use default
	if sc.Plugin == "" {
		plugin = defaultSignerID
	} else {
		plugin = sc.Plugin
	}
	signer, err := GetSigner(plugin)
	if err != nil {
		return "", err
	}
	return signer.GenerateSignedToken(files, sc, keyID)
}

// DefaultSigner is the default bundle signing implementation. It signs bundles by generating
// a JWT and signing it using a locally-accessible private key.
type DefaultSigner struct{}

// GenerateSignedToken generates a signed token given the list of files to be
// included in the payload and the bundle signing config. The keyID if non-empty,
// represents the value for the "keyid" claim in the token
func (*DefaultSigner) GenerateSignedToken(files []FileInfo, sc *SigningConfig, keyID string) (string, error) {
	payload, err := generatePayload(files, sc, keyID)
	if err != nil {
		return "", err
	}

	privateKey, err := sc.GetPrivateKey()
	if err != nil {
		return "", err
	}

	// Parse the algorithm string to jwa.SignatureAlgorithm
	alg, err := getSignatureAlgorithm(sc.Algorithm)
	if err != nil {
		return "", err
	}
	
	// Create signing options
	opts := []jws.SignOption{
		jws.WithKey(alg, privateKey),
	}
	
	if keyID != "" {
		headers := jws.NewHeaders()
		if err := headers.Set(jws.KeyIDKey, keyID); err != nil {
			return "", err
		}
		opts = append(opts, jws.WithKey(alg, privateKey, jws.WithProtectedHeaders(headers)))
		// Remove the previous WithKey option and replace it with one that includes headers
		opts = []jws.SignOption{opts[len(opts)-1]}
	}

	token, err := jws.Sign(payload, opts...)
	if err != nil {
		return "", err
	}
	return string(token), nil
}

func generatePayload(files []FileInfo, sc *SigningConfig, keyID string) ([]byte, error) {
	payload := make(map[string]any)
	payload["files"] = files

	if sc.ClaimsPath != "" {
		claims, err := sc.GetClaims()
		if err != nil {
			return nil, err
		}

		maps.Copy(payload, claims)
	} else if keyID != "" {
		// keyid claim is deprecated but include it for backwards compatibility.
		payload["keyid"] = keyID
	}
	return json.Marshal(payload)
}

// GetSigner returns the Signer registered under the given id
func GetSigner(id string) (Signer, error) {
	signer, ok := signers[id]
	if !ok {
		return nil, fmt.Errorf("no signer exists under id %s", id)
	}
	return signer, nil
}

// RegisterSigner registers a Signer under the given id
func RegisterSigner(id string, s Signer) error {
	if id == defaultSignerID {
		return fmt.Errorf("signer id %s is reserved, use a different id", id)
	}
	signers[id] = s
	return nil
}

func init() {
	signers = map[string]Signer{
		defaultSignerID: &DefaultSigner{},
	}
}
