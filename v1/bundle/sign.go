// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle provide helpers that assist in the creating a signed bundle
package bundle

import (
	"fmt"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

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
	token, err := generateToken(files, sc, keyID)
	if err != nil {
		return "", err
	}

	privateKey, err := sc.GetPrivateKey()
	if err != nil {
		return "", err
	}

	// Parse the algorithm string to jwa.SignatureAlgorithm
	alg, ok := jwa.LookupSignatureAlgorithm(sc.Algorithm)
	if !ok {
		return "", fmt.Errorf("unknown signature algorithm: %s", sc.Algorithm)
	}

	// In order to sign the token with a kid, we need a key ID _on_ the key
	// (note: we might be able to make this more efficient if we just load
	// the key as a JWK from the start)
	jwkKey, err := jwk.Import(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to import private key: %w", err)
	}
	if err := jwkKey.Set(jwk.KeyIDKey, keyID); err != nil {
		return "", fmt.Errorf("failed to set key ID on JWK: %w", err)
	}

	// Since v3.0.6, jwx will take the fast path for signing the token if
	// there's exactly one WithKey in the options with no sub-options
	signed, err := jwt.Sign(token, jwt.WithKey(alg, jwkKey))
	if err != nil {
		return "", err
	}
	return string(signed), nil
}

func generateToken(files []FileInfo, sc *SigningConfig, keyID string) (jwt.Token, error) {
	tb := jwt.NewBuilder()
	tb.Claim("files", files)

	if sc.ClaimsPath != "" {
		claims, err := sc.GetClaims()
		if err != nil {
			return nil, err
		}

		for k, v := range claims {
			tb.Claim(k, v)
		}
	} else if keyID != "" {
		// keyid claim is deprecated but include it for backwards compatibility.
		tb.Claim("keyid", keyID)
	}
	return tb.Build()
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
