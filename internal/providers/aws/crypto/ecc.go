package crypto

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"math"
	"math/big"
)

type ecdsaSignature struct {
	R, S *big.Int
}

// ECDSAKey takes the given elliptic curve, and private key (d) byte slice
// and returns the private ECDSA key.
func ECDSAKey(curve elliptic.Curve, d []byte) *ecdsa.PrivateKey {
	return ECDSAKeyFromPoint(curve, (&big.Int{}).SetBytes(d))
}

// ECDSAKeyFromPoint takes the given elliptic curve and point and returns the
// private and public keypair
func ECDSAKeyFromPoint(curve elliptic.Curve, d *big.Int) *ecdsa.PrivateKey {
	dBytes := make([]byte, (curve.Params().BitSize+7)/8)
	d.FillBytes(dBytes)

	privKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: curve,
		},
		D: d,
	}

	var pubBytes []byte
	switch curve {
	case elliptic.P256():
		if ecdhPriv, err := ecdh.P256().NewPrivateKey(dBytes); err == nil {
			pubBytes = ecdhPriv.PublicKey().Bytes()
		}
	case elliptic.P384():
		if ecdhPriv, err := ecdh.P384().NewPrivateKey(dBytes); err == nil {
			pubBytes = ecdhPriv.PublicKey().Bytes()
		}
	case elliptic.P521():
		if ecdhPriv, err := ecdh.P521().NewPrivateKey(dBytes); err == nil {
			pubBytes = ecdhPriv.PublicKey().Bytes()
		}
	}

	if len(pubBytes) > 0 {
		byteLen := (curve.Params().BitSize + 7) / 8
		privKey.X = new(big.Int).SetBytes(pubBytes[1 : 1+byteLen])
		privKey.Y = new(big.Int).SetBytes(pubBytes[1+byteLen:])
	} else {
		panic(fmt.Sprintf("unsupported curve or invalid private key: %v", curve))
	}

	return privKey
}

// mathIntToBytes writes val as a big-endian, fixed-length byte slice into out,
// zero-padding on the left when val.Bytes() is shorter than out. This satisfies
// the uncompressed SEC 1 encoding (0x04 || X || Y) expected by crypto/ecdh's
// NewPublicKey: https://pkg.go.dev/crypto/ecdh#Curve.NewPublicKey
func mathIntToBytes(val *big.Int, out []byte) {
	valBytes := val.Bytes()
	copy(out[len(out)-len(valBytes):], valBytes)
}

// ECDSAPublicKey takes the provide curve and (x, y) coordinates and returns
// *ecdsa.PublicKey. Returns an error if the given points are not on the curve.
func ECDSAPublicKey(curve elliptic.Curve, x, y []byte) (*ecdsa.PublicKey, error) {
	xPoint := (&big.Int{}).SetBytes(x)
	yPoint := (&big.Int{}).SetBytes(y)

	byteLen := (curve.Params().BitSize + 7) / 8
	buf := make([]byte, 1+2*byteLen)
	buf[0] = 4 // uncompressed point
	mathIntToBytes(xPoint, buf[1:1+byteLen])
	mathIntToBytes(yPoint, buf[1+byteLen:])

	var err error
	switch curve {
	case elliptic.P256():
		_, err = ecdh.P256().NewPublicKey(buf)
	case elliptic.P384():
		_, err = ecdh.P384().NewPublicKey(buf)
	case elliptic.P521():
		_, err = ecdh.P521().NewPublicKey(buf)
	default:
		err = fmt.Errorf("unsupported curve for ECDSA: %v", curve)
	}

	if err != nil {
		return nil, fmt.Errorf("point(%v, %v) is not on the given curve", xPoint.String(), yPoint.String())
	}

	return &ecdsa.PublicKey{
		Curve: curve,
		X:     xPoint,
		Y:     yPoint,
	}, nil
}

// VerifySignature takes the provided public key, hash, and asn1 encoded signature and returns
// whether the given signature is valid.
func VerifySignature(key *ecdsa.PublicKey, hash []byte, signature []byte) (bool, error) {
	var ecdsaSignature ecdsaSignature

	_, err := asn1.Unmarshal(signature, &ecdsaSignature)
	if err != nil {
		return false, err
	}

	return ecdsa.Verify(key, hash, ecdsaSignature.R, ecdsaSignature.S), nil
}

// HMACKeyDerivation provides an implementation of a NIST-800-108 of a KDF (Key Derivation Function) in Counter Mode.
// For the purposes of this implantation HMAC is used as the PRF (Pseudorandom function), where the value of
// `r` is defined as a 4 byte counter.
func HMACKeyDerivation(hash func() hash.Hash, bitLen int, key []byte, label, context []byte) ([]byte, error) {
	// verify that we won't overflow the counter
	n := int64(math.Ceil((float64(bitLen) / 8) / float64(hash().Size())))
	if n > 0x7FFFFFFF {
		return nil, fmt.Errorf("unable to derive key of size %d using 32-bit counter", bitLen)
	}

	// verify the requested bit length is not larger then the length encoding size
	if int64(bitLen) > 0x7FFFFFFF {
		return nil, errors.New("bitLen is greater than 32-bits")
	}

	fixedInput := bytes.NewBuffer(nil)
	fixedInput.Write(label)
	fixedInput.WriteByte(0x00)
	fixedInput.Write(context)
	if err := binary.Write(fixedInput, binary.BigEndian, int32(bitLen)); err != nil {
		return nil, fmt.Errorf("failed to write bit length to fixed input string: %v", err)
	}

	var output []byte

	h := hmac.New(hash, key)

	for i := int64(1); i <= n; i++ {
		h.Reset()
		if err := binary.Write(h, binary.BigEndian, int32(i)); err != nil {
			return nil, err
		}
		_, err := h.Write(fixedInput.Bytes())
		if err != nil {
			return nil, err
		}
		output = append(output, h.Sum(nil)...)
	}

	return output[:bitLen/8], nil
}
