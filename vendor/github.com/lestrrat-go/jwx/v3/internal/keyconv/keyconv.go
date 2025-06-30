package keyconv

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"fmt"

	"github.com/lestrrat-go/blackmagic"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"golang.org/x/crypto/ed25519"
)

// RSAPrivateKey assigns src to dst.
// `dst` should be a pointer to a rsa.PrivateKey.
// `src` may be rsa.PrivateKey, *rsa.PrivateKey, or a jwk.Key
func RSAPrivateKey(dst, src interface{}) error {
	if jwkKey, ok := src.(jwk.Key); ok {
		var raw rsa.PrivateKey
		if err := jwk.Export(jwkKey, &raw); err != nil {
			return fmt.Errorf(`failed to produce rsa.PrivateKey from %T: %w`, src, err)
		}
		src = &raw
	}

	var ptr *rsa.PrivateKey
	switch src := src.(type) {
	case rsa.PrivateKey:
		ptr = &src
	case *rsa.PrivateKey:
		ptr = src
	default:
		return fmt.Errorf(`keyconv: expected rsa.PrivateKey or *rsa.PrivateKey, got %T`, src)
	}

	return blackmagic.AssignIfCompatible(dst, ptr)
}

// RSAPublicKey assigns src to dst
// `dst` should be a pointer to a non-zero rsa.PublicKey.
// `src` may be rsa.PublicKey, *rsa.PublicKey, or a jwk.Key
func RSAPublicKey(dst, src interface{}) error {
	if jwkKey, ok := src.(jwk.Key); ok {
		pk, err := jwk.PublicRawKeyOf(jwkKey)
		if err != nil {
			return fmt.Errorf(`keyconv: failed to produce public key from %T: %w`, src, err)
		}
		src = pk
	}

	var ptr *rsa.PublicKey
	switch src := src.(type) {
	case rsa.PrivateKey:
		ptr = &src.PublicKey
	case *rsa.PrivateKey:
		ptr = &src.PublicKey
	case rsa.PublicKey:
		ptr = &src
	case *rsa.PublicKey:
		ptr = src
	default:
		return fmt.Errorf(`keyconv: expected rsa.PublicKey/rsa.PrivateKey or *rsa.PublicKey/*rsa.PrivateKey, got %T`, src)
	}

	return blackmagic.AssignIfCompatible(dst, ptr)
}

// ECDSAPrivateKey assigns src to dst, converting its type from a
// non-pointer to a pointer
func ECDSAPrivateKey(dst, src interface{}) error {
	if jwkKey, ok := src.(jwk.Key); ok {
		var raw ecdsa.PrivateKey
		if err := jwk.Export(jwkKey, &raw); err != nil {
			return fmt.Errorf(`keyconv: failed to produce ecdsa.PrivateKey from %T: %w`, src, err)
		}
		src = &raw
	}

	var ptr *ecdsa.PrivateKey
	switch src := src.(type) {
	case ecdsa.PrivateKey:
		ptr = &src
	case *ecdsa.PrivateKey:
		ptr = src
	default:
		return fmt.Errorf(`keyconv: expected ecdsa.PrivateKey or *ecdsa.PrivateKey, got %T`, src)
	}
	return blackmagic.AssignIfCompatible(dst, ptr)
}

// ECDSAPublicKey assigns src to dst, converting its type from a
// non-pointer to a pointer
func ECDSAPublicKey(dst, src interface{}) error {
	if jwkKey, ok := src.(jwk.Key); ok {
		pk, err := jwk.PublicRawKeyOf(jwkKey)
		if err != nil {
			return fmt.Errorf(`keyconv: failed to produce public key from %T: %w`, src, err)
		}
		src = pk
	}

	var ptr *ecdsa.PublicKey
	switch src := src.(type) {
	case ecdsa.PrivateKey:
		ptr = &src.PublicKey
	case *ecdsa.PrivateKey:
		ptr = &src.PublicKey
	case ecdsa.PublicKey:
		ptr = &src
	case *ecdsa.PublicKey:
		ptr = src
	default:
		return fmt.Errorf(`keyconv: expected ecdsa.PublicKey/ecdsa.PrivateKey or *ecdsa.PublicKey/*ecdsa.PrivateKey, got %T`, src)
	}
	return blackmagic.AssignIfCompatible(dst, ptr)
}

func ByteSliceKey(dst, src interface{}) error {
	if jwkKey, ok := src.(jwk.Key); ok {
		var raw []byte
		if err := jwk.Export(jwkKey, &raw); err != nil {
			return fmt.Errorf(`keyconv: failed to produce []byte from %T: %w`, src, err)
		}
		src = raw
	}

	if _, ok := src.([]byte); !ok {
		return fmt.Errorf(`keyconv: expected []byte, got %T`, src)
	}
	return blackmagic.AssignIfCompatible(dst, src)
}

func Ed25519PrivateKey(dst, src interface{}) error {
	if jwkKey, ok := src.(jwk.Key); ok {
		var raw ed25519.PrivateKey
		if err := jwk.Export(jwkKey, &raw); err != nil {
			return fmt.Errorf(`failed to produce ed25519.PrivateKey from %T: %w`, src, err)
		}
		src = &raw
	}

	var ptr *ed25519.PrivateKey
	switch src := src.(type) {
	case ed25519.PrivateKey:
		ptr = &src
	case *ed25519.PrivateKey:
		ptr = src
	default:
		return fmt.Errorf(`expected ed25519.PrivateKey or *ed25519.PrivateKey, got %T`, src)
	}
	return blackmagic.AssignIfCompatible(dst, ptr)
}

func Ed25519PublicKey(dst, src interface{}) error {
	if jwkKey, ok := src.(jwk.Key); ok {
		pk, err := jwk.PublicRawKeyOf(jwkKey)
		if err != nil {
			return fmt.Errorf(`keyconv: failed to produce public key from %T: %w`, src, err)
		}
		src = pk
	}

	switch key := src.(type) {
	case ed25519.PrivateKey:
		src = key.Public()
	case *ed25519.PrivateKey:
		src = key.Public()
	}

	var ptr *ed25519.PublicKey
	switch src := src.(type) {
	case ed25519.PublicKey:
		ptr = &src
	case *ed25519.PublicKey:
		ptr = src
	case *crypto.PublicKey:
		tmp, ok := (*src).(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf(`failed to retrieve ed25519.PublicKey out of *crypto.PublicKey`)
		}
		ptr = &tmp
	case crypto.PublicKey:
		tmp, ok := src.(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf(`failed to retrieve ed25519.PublicKey out of crypto.PublicKey`)
		}
		ptr = &tmp
	default:
		return fmt.Errorf(`expected ed25519.PublicKey or *ed25519.PublicKey, got %T`, src)
	}
	return blackmagic.AssignIfCompatible(dst, ptr)
}
