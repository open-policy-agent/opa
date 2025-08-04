package keygen

import (
	"crypto"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/lestrrat-go/jwx/v3/internal/ecutil"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/concatkdf"
	"github.com/lestrrat-go/jwx/v3/jwk"
)

// Bytes returns the byte from this ByteKey
func (k ByteKey) Bytes() []byte {
	return []byte(k)
}

func Random(n int) (ByteSource, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, buf); err != nil {
		return nil, fmt.Errorf(`failed to read from rand.Reader: %w`, err)
	}
	return ByteKey(buf), nil
}

// Ecdhes generates a new key using ECDH-ES
func Ecdhes(alg string, enc string, keysize int, pubkey *ecdsa.PublicKey, apu, apv []byte) (ByteSource, error) {
	priv, err := ecdsa.GenerateKey(pubkey.Curve, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf(`failed to generate key for ECDH-ES: %w`, err)
	}

	var algorithm string
	if alg == tokens.ECDH_ES {
		algorithm = enc
	} else {
		algorithm = alg
	}

	pubinfo := make([]byte, 4)
	binary.BigEndian.PutUint32(pubinfo, uint32(keysize)*8)

	if !priv.PublicKey.Curve.IsOnCurve(pubkey.X, pubkey.Y) {
		return nil, fmt.Errorf(`public key used does not contain a point (X,Y) on the curve`)
	}
	z, _ := priv.PublicKey.Curve.ScalarMult(pubkey.X, pubkey.Y, priv.D.Bytes())
	zBytes := ecutil.AllocECPointBuffer(z, priv.PublicKey.Curve)
	defer ecutil.ReleaseECPointBuffer(zBytes)
	kdf := concatkdf.New(crypto.SHA256, []byte(algorithm), zBytes, apu, apv, pubinfo, []byte{})
	kek := make([]byte, keysize)
	if _, err := kdf.Read(kek); err != nil {
		return nil, fmt.Errorf(`failed to read kdf: %w`, err)
	}

	return ByteWithECPublicKey{
		PublicKey: &priv.PublicKey,
		ByteKey:   ByteKey(kek),
	}, nil
}

// X25519 generates a new key using ECDH-ES with X25519
func X25519(alg string, enc string, keysize int, pubkey *ecdh.PublicKey) (ByteSource, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf(`failed to generate key for X25519: %w`, err)
	}

	var algorithm string
	if alg == tokens.ECDH_ES {
		algorithm = enc
	} else {
		algorithm = alg
	}

	pubinfo := make([]byte, 4)
	binary.BigEndian.PutUint32(pubinfo, uint32(keysize)*8)

	zBytes, err := priv.ECDH(pubkey)
	if err != nil {
		return nil, fmt.Errorf(`failed to compute Z: %w`, err)
	}
	kdf := concatkdf.New(crypto.SHA256, []byte(algorithm), zBytes, []byte{}, []byte{}, pubinfo, []byte{})
	kek := make([]byte, keysize)
	if _, err := kdf.Read(kek); err != nil {
		return nil, fmt.Errorf(`failed to read kdf: %w`, err)
	}

	return ByteWithECPublicKey{
		PublicKey: priv.PublicKey(),
		ByteKey:   ByteKey(kek),
	}, nil
}

// HeaderPopulate populates the header with the required EC-DSA public key
// information ('epk' key)
func (k ByteWithECPublicKey) Populate(h Setter) error {
	key, err := jwk.Import(k.PublicKey)
	if err != nil {
		return fmt.Errorf(`failed to create JWK: %w`, err)
	}

	if err := h.Set("epk", key); err != nil {
		return fmt.Errorf(`failed to write header: %w`, err)
	}
	return nil
}

// HeaderPopulate populates the header with the required AES GCM
// parameters ('iv' and 'tag')
func (k ByteWithIVAndTag) Populate(h Setter) error {
	if err := h.Set("iv", k.IV); err != nil {
		return fmt.Errorf(`failed to write header: %w`, err)
	}

	if err := h.Set("tag", k.Tag); err != nil {
		return fmt.Errorf(`failed to write header: %w`, err)
	}

	return nil
}

// HeaderPopulate populates the header with the required PBES2
// parameters ('p2s' and 'p2c')
func (k ByteWithSaltAndCount) Populate(h Setter) error {
	if err := h.Set("p2c", k.Count); err != nil {
		return fmt.Errorf(`failed to write header: %w`, err)
	}

	if err := h.Set("p2s", k.Salt); err != nil {
		return fmt.Errorf(`failed to write header: %w`, err)
	}

	return nil
}
