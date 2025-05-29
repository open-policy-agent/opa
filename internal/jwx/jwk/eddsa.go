package jwk

import (
	"crypto/ed25519"
	"crypto/x509"
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
)

func newEdDSAPublicKey(key ed25519.PublicKey) (*EdDSAPublicKey, error) {

	var hdr StandardHeaders
	err := hdr.Set(KeyTypeKey, jwa.EC)
	if err != nil {
		return nil, fmt.Errorf("failed to set Key Type: %w", err)
	}

	return &EdDSAPublicKey{
		StandardHeaders: &hdr,
		key:             key,
	}, nil
}

func newEdDSAPrivateKey(key ed25519.PrivateKey) (*EdDSAPrivateKey, error) {

	var hdr StandardHeaders
	err := hdr.Set(KeyTypeKey, jwa.EC)
	if err != nil {
		return nil, fmt.Errorf("failed to set Key Type: %w", err)
	}

	return &EdDSAPrivateKey{
		StandardHeaders: &hdr,
		key:             key,
	}, nil
}

// Materialize returns the standard EdDSA Public Key representation stored in the internal representation
func (k *EdDSAPublicKey) Materialize() (interface{}, error) {
	if k.key == nil {
		return nil, errors.New("key has no ed25519.PublicKey associated with it")
	}
	return k.key, nil
}

// Materialize returns the standard EdDSA Private Key representation stored in the internal representation
func (k *EdDSAPrivateKey) Materialize() (interface{}, error) {
	if k.key == nil {
		return nil, errors.New("key has no ed25519.PrivateKey associated with it")
	}
	return k.key, nil
}

// GenerateKey creates a ECDSAPublicKey from JWK format
func (k *EdDSAPublicKey) GenerateKey(keyJSON *RawKeyJSON) error {
	if keyJSON.X == nil || keyJSON.Crv == "" {
		return errors.New("missing mandatory key parameters X, Crv")
	}

	switch keyJSON.Crv {
	case string(jwa.Ed25519):
	default:
		return fmt.Errorf("invalid curve name %s", keyJSON.Crv)
	}

	parsedKey, err := x509.ParsePKIXPublicKey(keyJSON.X.Bytes())
	if err != nil {
		return fmt.Errorf("failed to parse public key: %v", err)
	}

	publicKey, ok := parsedKey.(ed25519.PublicKey)
	if !ok {
		return fmt.Errorf("not an Ed25519 public key")
	}

	*k = EdDSAPublicKey{
		StandardHeaders: &keyJSON.StandardHeaders,
		key:             publicKey,
	}

	return nil
}

// GenerateKey creates a ECDSAPrivateKey from JWK format
func (k *EdDSAPrivateKey) GenerateKey(keyJSON *RawKeyJSON) error {
	if keyJSON.D == nil {
		return errors.New("missing mandatory key parameter D")
	}

	parsedKey, err := x509.ParsePKCS8PrivateKey(keyJSON.D.Bytes())
	if err != nil {
		return fmt.Errorf("failed to parse private key: %v", err)
	}

	privateKey, ok := parsedKey.(ed25519.PrivateKey)
	if !ok {
		return fmt.Errorf("not an Ed25519 private key")
	}

	*k = EdDSAPrivateKey{
		StandardHeaders: &keyJSON.StandardHeaders,
		key:             privateKey,
	}

	return nil
}
