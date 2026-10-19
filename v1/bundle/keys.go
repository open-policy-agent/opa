// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package bundle provide helpers that assist in creating the verification and signing key configuration
package bundle

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/open-policy-agent/opa/v1/keys"

	"github.com/open-policy-agent/opa/v1/util"
)

const (
	defaultTokenSigningAlg = "RS256"
)

// KeyConfig holds the keys used to sign or verify bundles and tokens
// Moved to own package, alias kept for backwards compatibility
type KeyConfig = keys.Config

// VerificationConfig represents the key configuration used to verify a signed bundle
type VerificationConfig struct {
	PublicKeys map[string]*KeyConfig
	KeyID      string   `json:"keyid"`
	Scope      string   `json:"scope"`
	Exclude    []string `json:"exclude_files"`

	// AllowUnsignedDeltaBundles opts out of rejecting delta bundles, which cannot
	// carry a signature. Snapshot bundles must still be signed.
	AllowUnsignedDeltaBundles bool `json:"allow_unsigned_delta_bundles"`

	signatureOptional bool
}

// NewVerificationConfig return a new VerificationConfig
func NewVerificationConfig(keys map[string]*KeyConfig, id, scope string, exclude []string) *VerificationConfig {
	return &VerificationConfig{
		PublicKeys: keys,
		KeyID:      id,
		Scope:      scope,
		Exclude:    exclude,
	}
}

// WithOptionalSignature marks vc as not requiring a .signatures.json file. A
// signature that IS present is still fully verified.
func (vc *VerificationConfig) WithOptionalSignature() *VerificationConfig {
	vc.signatureOptional = true
	return vc
}

// SignatureRequired reports whether a bundle must carry a .signatures.json file.
// A nil config means no verification is configured, so no signature is required.
func (vc *VerificationConfig) SignatureRequired() bool {
	return vc != nil && !vc.signatureOptional
}

// UnsignedDeltaBundlesAllowed reports whether a delta bundle, which cannot carry a
// signature, may be activated under this config. Delta bundles are rejected when a
// signature is required, unless the `allow_unsigned_delta_bundles` opt-out is set.
// A nil config means no verification is configured, so delta bundles are allowed.
func (vc *VerificationConfig) UnsignedDeltaBundlesAllowed() bool {
	return !vc.SignatureRequired() || vc.AllowUnsignedDeltaBundles
}

// ValidateAndInjectDefaults validates the config and inserts default values
func (vc *VerificationConfig) ValidateAndInjectDefaults(keys map[string]*KeyConfig) error {
	// This is only reachable from an explicit `signing` block. Such a block with
	// no keys to verify against can never load a signed bundle, so reject it here
	// rather than starting up with a config that cannot work.
	if len(keys) == 0 {
		return errors.New("no verification keys configured")
	}

	vc.PublicKeys = keys

	if vc.KeyID != "" {
		found := false
		for key := range keys {
			if key == vc.KeyID {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("key id %s not found", vc.KeyID)
		}
	}
	return nil
}

// GetPublicKey returns the public key corresponding to the given key id
func (vc *VerificationConfig) GetPublicKey(id string) (*KeyConfig, error) {
	var kc *KeyConfig
	var ok bool

	if kc, ok = vc.PublicKeys[id]; !ok {
		return nil, fmt.Errorf("verification key corresponding to ID %v not found", id)
	}
	return kc, nil
}

// SigningConfig represents the key configuration used to generate a signed bundle
type SigningConfig struct {
	Plugin     string
	Key        string
	Algorithm  string
	ClaimsPath string
}

// NewSigningConfig return a new SigningConfig
func NewSigningConfig(key, alg, claimsPath string) *SigningConfig {
	if alg == "" {
		alg = defaultTokenSigningAlg
	}

	return &SigningConfig{
		Plugin:     defaultSignerID,
		Key:        key,
		Algorithm:  alg,
		ClaimsPath: claimsPath,
	}
}

// WithPlugin sets the signing plugin in the signing config
func (s *SigningConfig) WithPlugin(plugin string) *SigningConfig {
	if plugin != "" {
		s.Plugin = plugin
	}
	return s
}

// GetPrivateKey returns the private key or secret from the signing config
func (s *SigningConfig) GetPrivateKey() (any, error) {
	var keyData string

	alg, ok := jwa.LookupSignatureAlgorithm(s.Algorithm)
	if !ok {
		return nil, fmt.Errorf("unknown signature algorithm: %s", s.Algorithm)
	}

	// Check if the key looks like PEM data first (starts with -----BEGIN)
	if strings.HasPrefix(s.Key, "-----BEGIN") {
		keyData = s.Key
	} else {
		// Try to read as a file path
		if _, err := os.Stat(s.Key); err == nil {
			bs, err := os.ReadFile(s.Key)
			if err != nil {
				return nil, err
			}
			keyData = string(bs)
		} else if os.IsNotExist(err) {
			// Not a file, treat as raw key data
			keyData = s.Key
		} else {
			return nil, err
		}
	}

	// For HMAC algorithms, return the key as bytes
	if alg == jwa.HS256() || alg == jwa.HS384() || alg == jwa.HS512() {
		return []byte(keyData), nil
	}

	// For RSA/ECDSA algorithms, parse the PEM-encoded key
	block, _ := pem.Decode([]byte(keyData))
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the key")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		return x509.ParsePKCS8PrivateKey(block.Bytes)
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("unsupported key type: %s", block.Type)
	}
}

// GetClaims returns the claims by reading the file specified in the signing config
func (s *SigningConfig) GetClaims() (map[string]any, error) {
	var claims map[string]any

	bs, err := os.ReadFile(s.ClaimsPath)
	if err != nil {
		return claims, err
	}

	if err := util.UnmarshalJSON(bs, &claims); err != nil {
		return claims, err
	}
	return claims, nil
}
