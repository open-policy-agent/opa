package jwk

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// PEMDecoder is an interface to describe an object that can decode
// a key from PEM encoded ASN.1 DER format.
//
// A PEMDecoder can be specified as an option to `jwk.Parse()` or `jwk.ParseKey()`
// along with the `jwk.WithPEM()` option.
type PEMDecoder interface {
	Decode([]byte) (interface{}, []byte, error)
}

// PEMEncoder is an interface to describe an object that can encode
// a key into PEM encoded ASN.1 DER format.
//
// `jwk.Key` instances do not implement a way to encode themselves into
// PEM format. Normally you can just use `jwk.EncodePEM()` to do this, but
// this interface allows you to generalize the encoding process by
// abstracting the `jwk.EncodePEM()` function using `jwk.PEMEncodeFunc`
// along with alternate implementations, should you need them.
type PEMEncoder interface {
	Encode(interface{}) (string, []byte, error)
}

type PEMEncodeFunc func(interface{}) (string, []byte, error)

func (f PEMEncodeFunc) Encode(v interface{}) (string, []byte, error) {
	return f(v)
}

func encodeX509(v interface{}) (string, []byte, error) {
	// we can't import jwk, so just use the interface
	if key, ok := v.(Key); ok {
		var raw interface{}
		if err := Export(key, &raw); err != nil {
			return "", nil, fmt.Errorf(`failed to get raw key out of %T: %w`, key, err)
		}

		v = raw
	}

	// Try to convert it into a certificate
	switch v := v.(type) {
	case *rsa.PrivateKey:
		return pmRSAPrivateKey, x509.MarshalPKCS1PrivateKey(v), nil
	case *ecdsa.PrivateKey:
		marshaled, err := x509.MarshalECPrivateKey(v)
		if err != nil {
			return "", nil, err
		}
		return pmECPrivateKey, marshaled, nil
	case ed25519.PrivateKey:
		marshaled, err := x509.MarshalPKCS8PrivateKey(v)
		if err != nil {
			return "", nil, err
		}
		return pmPrivateKey, marshaled, nil
	case *rsa.PublicKey, *ecdsa.PublicKey, ed25519.PublicKey:
		marshaled, err := x509.MarshalPKIXPublicKey(v)
		if err != nil {
			return "", nil, err
		}
		return pmPublicKey, marshaled, nil
	default:
		return "", nil, fmt.Errorf(`unsupported type %T for ASN.1 DER encoding`, v)
	}
}

// EncodePEM encodes the key into a PEM encoded ASN.1 DER format.
// The key can be a jwk.Key or a raw key instance, but it must be one of
// the types supported by `x509` package.
//
// Internally, it uses the same routine as `jwk.EncodeX509()`, and therefore
// the same caveats apply
func EncodePEM(v interface{}) ([]byte, error) {
	typ, marshaled, err := encodeX509(v)
	if err != nil {
		return nil, fmt.Errorf(`failed to encode key in x509: %w`, err)
	}

	block := &pem.Block{
		Type:  typ,
		Bytes: marshaled,
	}
	return pem.EncodeToMemory(block), nil
}

const (
	pmPrivateKey    = `PRIVATE KEY`
	pmPublicKey     = `PUBLIC KEY`
	pmECPrivateKey  = `EC PRIVATE KEY`
	pmRSAPublicKey  = `RSA PUBLIC KEY`
	pmRSAPrivateKey = `RSA PRIVATE KEY`
)

func NewPEMDecoder() PEMDecoder {
	return pemDecoder{}
}

type pemDecoder struct{}

// DecodePEM decodes a key in PEM encoded ASN.1 DER format.
// and returns a raw key
func (pemDecoder) Decode(src []byte) (interface{}, []byte, error) {
	block, rest := pem.Decode(src)
	if block == nil {
		return nil, nil, fmt.Errorf(`failed to decode PEM data`)
	}

	switch block.Type {
	// Handle the semi-obvious cases
	case pmRSAPrivateKey:
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse PKCS1 private key: %w`, err)
		}
		return key, rest, nil
	case pmRSAPublicKey:
		key, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse PKCS1 public key: %w`, err)
		}
		return key, rest, nil
	case pmECPrivateKey:
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse EC private key: %w`, err)
		}
		return key, rest, nil
	case pmPublicKey:
		// XXX *could* return dsa.PublicKey
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse PKIX public key: %w`, err)
		}
		return key, rest, nil
	case pmPrivateKey:
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse PKCS8 private key: %w`, err)
		}
		return key, rest, nil
	case "CERTIFICATE":
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf(`failed to parse certificate: %w`, err)
		}
		return cert.PublicKey, rest, nil
	default:
		return nil, nil, fmt.Errorf(`invalid PEM block type %s`, block.Type)
	}
}
