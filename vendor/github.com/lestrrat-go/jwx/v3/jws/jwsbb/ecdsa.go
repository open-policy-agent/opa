package jwsbb

import (
	"crypto"
	"crypto/ecdsa"
	"encoding/asn1"
	"fmt"
	"io"
	"math/big"

	"github.com/lestrrat-go/dsig"
	"github.com/lestrrat-go/jwx/v3/internal/ecutil"
)

// ecdsaHashToDsigAlgorithm maps ECDSA hash functions to dsig algorithm constants
func ecdsaHashToDsigAlgorithm(h crypto.Hash) (string, error) {
	switch h {
	case crypto.SHA256:
		return dsig.ECDSAWithP256AndSHA256, nil
	case crypto.SHA384:
		return dsig.ECDSAWithP384AndSHA384, nil
	case crypto.SHA512:
		return dsig.ECDSAWithP521AndSHA512, nil
	default:
		return "", fmt.Errorf("unsupported ECDSA hash function: %v", h)
	}
}

// UnpackASN1ECDSASignature unpacks an ASN.1 encoded ECDSA signature into r and s values.
// This is typically used when working with crypto.Signer interfaces that return ASN.1 encoded signatures.
func UnpackASN1ECDSASignature(signed []byte, r, s *big.Int) error {
	// Okay, this is silly, but hear me out. When we use the
	// crypto.Signer interface, the PrivateKey is hidden.
	// But we need some information about the key (its bit size).
	//
	// So while silly, we're going to have to make another call
	// here and fetch the Public key.
	// (This probably means that this information should be cached somewhere)
	var p struct {
		R *big.Int // TODO: get this from a pool?
		S *big.Int
	}
	if _, err := asn1.Unmarshal(signed, &p); err != nil {
		return fmt.Errorf(`failed to unmarshal ASN1 encoded signature: %w`, err)
	}

	r.Set(p.R)
	s.Set(p.S)
	return nil
}

// UnpackECDSASignature unpacks a JWS-format ECDSA signature into r and s values.
// The signature should be in the format specified by RFC 7515 (r||s as fixed-length byte arrays).
func UnpackECDSASignature(signature []byte, pubkey *ecdsa.PublicKey, r, s *big.Int) error {
	keySize := ecutil.CalculateKeySize(pubkey.Curve)
	if len(signature) != keySize*2 {
		return fmt.Errorf(`invalid signature length for curve %q`, pubkey.Curve.Params().Name)
	}

	r.SetBytes(signature[:keySize])
	s.SetBytes(signature[keySize:])

	return nil
}

// PackECDSASignature packs the r and s values from an ECDSA signature into a JWS-format byte slice.
// The output format follows RFC 7515: r||s as fixed-length byte arrays.
func PackECDSASignature(r *big.Int, sbig *big.Int, curveBits int) ([]byte, error) {
	keyBytes := curveBits / 8
	if curveBits%8 > 0 {
		keyBytes++
	}

	// Serialize r and s into fixed-length bytes
	rBytes := r.Bytes()
	rBytesPadded := make([]byte, keyBytes)
	copy(rBytesPadded[keyBytes-len(rBytes):], rBytes)

	sBytes := sbig.Bytes()
	sBytesPadded := make([]byte, keyBytes)
	copy(sBytesPadded[keyBytes-len(sBytes):], sBytes)

	// Output as r||s
	return append(rBytesPadded, sBytesPadded...), nil
}

// SignECDSA generates an ECDSA signature for the given payload using the specified private key and hash.
// The raw parameter should be the pre-computed signing input (typically header.payload).
//
// rr is an io.Reader that provides randomness for signing. if rr is nil, it defaults to rand.Reader.
//
// This function is now a thin wrapper around dsig.SignECDSA. For new projects, you should
// consider using dsig instead of this function.
func SignECDSA(key *ecdsa.PrivateKey, payload []byte, h crypto.Hash, rr io.Reader) ([]byte, error) {
	dsigAlg, err := ecdsaHashToDsigAlgorithm(h)
	if err != nil {
		return nil, fmt.Errorf("jwsbb.SignECDSA: %w", err)
	}

	return dsig.Sign(key, dsigAlg, payload, rr)
}

// SignECDSACryptoSigner generates an ECDSA signature using a crypto.Signer interface.
// This function works with hardware security modules and other crypto.Signer implementations.
// The signature is converted from ASN.1 format to JWS format (r||s).
//
// rr is an io.Reader that provides randomness for signing. If rr is nil, it defaults to rand.Reader.
func SignECDSACryptoSigner(signer crypto.Signer, raw []byte, h crypto.Hash, rr io.Reader) ([]byte, error) {
	signed, err := SignCryptoSigner(signer, raw, h, h, rr)
	if err != nil {
		return nil, fmt.Errorf(`failed to sign payload using crypto.Signer: %w`, err)
	}

	return signECDSACryptoSigner(signer, signed)
}

func signECDSACryptoSigner(signer crypto.Signer, signed []byte) ([]byte, error) {
	cpub := signer.Public()
	pubkey, ok := cpub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf(`expected *ecdsa.PublicKey, got %T`, pubkey)
	}
	curveBits := pubkey.Curve.Params().BitSize

	var r, s big.Int
	if err := UnpackASN1ECDSASignature(signed, &r, &s); err != nil {
		return nil, fmt.Errorf(`failed to unpack ASN1 encoded signature: %w`, err)
	}

	return PackECDSASignature(&r, &s, curveBits)
}

func ecdsaVerify(key *ecdsa.PublicKey, buf []byte, h crypto.Hash, r, s *big.Int) error {
	hasher := h.New()
	hasher.Write(buf)
	digest := hasher.Sum(nil)
	if !ecdsa.Verify(key, digest, r, s) {
		return fmt.Errorf("jwsbb.ECDSAVerifier: invalid ECDSA signature")
	}
	return nil
}

// VerifyECDSA verifies an ECDSA signature for the given payload.
// This function verifies the signature using the specified public key and hash algorithm.
// The payload parameter should be the pre-computed signing input (typically header.payload).
//
// This function is now a thin wrapper around dsig.VerifyECDSA. For new projects, you should
// consider using dsig instead of this function.
func VerifyECDSA(key *ecdsa.PublicKey, payload, signature []byte, h crypto.Hash) error {
	dsigAlg, err := ecdsaHashToDsigAlgorithm(h)
	if err != nil {
		return fmt.Errorf("jwsbb.VerifyECDSA: %w", err)
	}

	return dsig.Verify(key, dsigAlg, payload, signature)
}

// VerifyECDSACryptoSigner verifies an ECDSA signature for crypto.Signer implementations.
// This function is useful for verifying signatures created by hardware security modules
// or other implementations of the crypto.Signer interface.
// The payload parameter should be the pre-computed signing input (typically header.payload).
func VerifyECDSACryptoSigner(signer crypto.Signer, payload, signature []byte, h crypto.Hash) error {
	var pubkey *ecdsa.PublicKey
	switch cpub := signer.Public(); cpub := cpub.(type) {
	case ecdsa.PublicKey:
		pubkey = &cpub
	case *ecdsa.PublicKey:
		pubkey = cpub
	default:
		return fmt.Errorf(`jwsbb.VerifyECDSACryptoSigner: expected *ecdsa.PublicKey, got %T`, cpub)
	}

	var r, s big.Int
	if err := UnpackECDSASignature(signature, pubkey, &r, &s); err != nil {
		return fmt.Errorf("jwsbb.ECDSAVerifier: failed to unpack ASN.1 encoded ECDSA signature: %w", err)
	}

	return ecdsaVerify(pubkey, payload, h, &r, &s)
}
