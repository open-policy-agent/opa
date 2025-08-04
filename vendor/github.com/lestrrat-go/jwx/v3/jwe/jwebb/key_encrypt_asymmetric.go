package jwebb

import (
	"crypto/aes"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"

	"github.com/lestrrat-go/jwx/v3/internal/tokens"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/keygen"
)

// KeyEncryptRSA15 encrypts the CEK using RSA PKCS#1 v1.5
func KeyEncryptRSA15(cek []byte, _ string, pubkey *rsa.PublicKey) (keygen.ByteSource, error) {
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubkey, cek)
	if err != nil {
		return nil, fmt.Errorf(`failed to encrypt using PKCS1v15: %w`, err)
	}
	return keygen.ByteKey(encrypted), nil
}

// KeyEncryptRSAOAEP encrypts the CEK using RSA OAEP
func KeyEncryptRSAOAEP(cek []byte, alg string, pubkey *rsa.PublicKey) (keygen.ByteSource, error) {
	var hash hash.Hash
	switch alg {
	case tokens.RSA_OAEP:
		hash = sha1.New()
	case tokens.RSA_OAEP_256:
		hash = sha256.New()
	case tokens.RSA_OAEP_384:
		hash = sha512.New384()
	case tokens.RSA_OAEP_512:
		hash = sha512.New()
	default:
		return nil, fmt.Errorf(`failed to generate key encrypter for RSA-OAEP: RSA_OAEP/RSA_OAEP_256/RSA_OAEP_384/RSA_OAEP_512 required`)
	}

	encrypted, err := rsa.EncryptOAEP(hash, rand.Reader, pubkey, cek, []byte{})
	if err != nil {
		return nil, fmt.Errorf(`failed to OAEP encrypt: %w`, err)
	}
	return keygen.ByteKey(encrypted), nil
}

// generateECDHESKeyECDSA generates the key material for ECDSA keys using ECDH-ES
func generateECDHESKeyECDSA(alg string, calg string, keysize uint32, pubkey *ecdsa.PublicKey, apu, apv []byte) (keygen.ByteWithECPublicKey, error) {
	// Generate the key directly
	kg, err := keygen.Ecdhes(alg, calg, int(keysize), pubkey, apu, apv)
	if err != nil {
		return keygen.ByteWithECPublicKey{}, fmt.Errorf(`failed to generate ECDSA key: %w`, err)
	}

	bwpk, ok := kg.(keygen.ByteWithECPublicKey)
	if !ok {
		return keygen.ByteWithECPublicKey{}, fmt.Errorf(`key generator generated invalid key (expected ByteWithECPublicKey)`)
	}

	return bwpk, nil
}

// generateECDHESKeyX25519 generates the key material for X25519 keys using ECDH-ES
func generateECDHESKeyX25519(alg string, calg string, keysize uint32, pubkey *ecdh.PublicKey) (keygen.ByteWithECPublicKey, error) {
	// Generate the key directly
	kg, err := keygen.X25519(alg, calg, int(keysize), pubkey)
	if err != nil {
		return keygen.ByteWithECPublicKey{}, fmt.Errorf(`failed to generate X25519 key: %w`, err)
	}

	bwpk, ok := kg.(keygen.ByteWithECPublicKey)
	if !ok {
		return keygen.ByteWithECPublicKey{}, fmt.Errorf(`key generator generated invalid key (expected ByteWithECPublicKey)`)
	}

	return bwpk, nil
}

// KeyEncryptECDHESKeyWrapECDSA encrypts the CEK using ECDH-ES with key wrapping for ECDSA keys
func KeyEncryptECDHESKeyWrapECDSA(cek []byte, alg string, apu, apv []byte, pubkey *ecdsa.PublicKey, keysize uint32, calg string) (keygen.ByteSource, error) {
	bwpk, err := generateECDHESKeyECDSA(alg, calg, keysize, pubkey, apu, apv)
	if err != nil {
		return nil, err
	}

	// For key wrapping algorithms, wrap the CEK with the generated key
	block, err := aes.NewCipher(bwpk.Bytes())
	if err != nil {
		return nil, fmt.Errorf(`failed to generate cipher from generated key: %w`, err)
	}

	jek, err := Wrap(block, cek)
	if err != nil {
		return nil, fmt.Errorf(`failed to wrap data: %w`, err)
	}

	bwpk.ByteKey = keygen.ByteKey(jek)
	return bwpk, nil
}

// KeyEncryptECDHESKeyWrapX25519 encrypts the CEK using ECDH-ES with key wrapping for X25519 keys
func KeyEncryptECDHESKeyWrapX25519(cek []byte, alg string, _ []byte, _ []byte, pubkey *ecdh.PublicKey, keysize uint32, calg string) (keygen.ByteSource, error) {
	bwpk, err := generateECDHESKeyX25519(alg, calg, keysize, pubkey)
	if err != nil {
		return nil, err
	}

	// For key wrapping algorithms, wrap the CEK with the generated key
	block, err := aes.NewCipher(bwpk.Bytes())
	if err != nil {
		return nil, fmt.Errorf(`failed to generate cipher from generated key: %w`, err)
	}

	jek, err := Wrap(block, cek)
	if err != nil {
		return nil, fmt.Errorf(`failed to wrap data: %w`, err)
	}

	bwpk.ByteKey = keygen.ByteKey(jek)
	return bwpk, nil
}

// KeyEncryptECDHESECDSA encrypts using ECDH-ES direct (no key wrapping) for ECDSA keys
func KeyEncryptECDHESECDSA(_ []byte, alg string, apu, apv []byte, pubkey *ecdsa.PublicKey, keysize uint32, calg string) (keygen.ByteSource, error) {
	bwpk, err := generateECDHESKeyECDSA(alg, calg, keysize, pubkey, apu, apv)
	if err != nil {
		return nil, err
	}

	// For direct ECDH-ES, return the generated key directly
	return bwpk, nil
}

// KeyEncryptECDHESX25519 encrypts using ECDH-ES direct (no key wrapping) for X25519 keys
func KeyEncryptECDHESX25519(_ []byte, alg string, _, _ []byte, pubkey *ecdh.PublicKey, keysize uint32, calg string) (keygen.ByteSource, error) {
	bwpk, err := generateECDHESKeyX25519(alg, calg, keysize, pubkey)
	if err != nil {
		return nil, err
	}

	// For direct ECDH-ES, return the generated key directly
	return bwpk, nil
}
