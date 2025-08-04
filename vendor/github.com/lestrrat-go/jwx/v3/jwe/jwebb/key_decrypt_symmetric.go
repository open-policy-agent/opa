package jwebb

import (
	"crypto/aes"
	cryptocipher "crypto/cipher"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"

	"golang.org/x/crypto/pbkdf2"

	"github.com/lestrrat-go/jwx/v3/internal/tokens"
)

// AES key wrap decryption functions

// Use constants from tokens package
// No need to redefine them here

func KeyDecryptAESKW(_, enckey []byte, _ string, sharedkey []byte) ([]byte, error) {
	block, err := aes.NewCipher(sharedkey)
	if err != nil {
		return nil, fmt.Errorf(`failed to create cipher from shared key: %w`, err)
	}

	cek, err := Unwrap(block, enckey)
	if err != nil {
		return nil, fmt.Errorf(`failed to unwrap data: %w`, err)
	}
	return cek, nil
}

func KeyDecryptDirect(_, _ []byte, _ string, cek []byte) ([]byte, error) {
	return cek, nil
}

func KeyDecryptPBES2(_, enckey []byte, alg string, password []byte, salt []byte, count int) ([]byte, error) {
	var hashFunc func() hash.Hash
	var keylen int

	switch alg {
	case tokens.PBES2_HS256_A128KW:
		hashFunc = sha256.New
		keylen = tokens.KeySize16
	case tokens.PBES2_HS384_A192KW:
		hashFunc = sha512.New384
		keylen = tokens.KeySize24
	case tokens.PBES2_HS512_A256KW:
		hashFunc = sha512.New
		keylen = tokens.KeySize32
	default:
		return nil, fmt.Errorf(`unsupported PBES2 algorithm: %s`, alg)
	}

	// Derive key using PBKDF2
	derivedKey := pbkdf2.Key(password, salt, count, keylen, hashFunc)

	// Use the derived key for AES key wrap
	return KeyDecryptAESKW(nil, enckey, alg, derivedKey)
}

func KeyDecryptAESGCMKW(recipientKey, _ []byte, _ string, sharedkey []byte, iv []byte, tag []byte) ([]byte, error) {
	if len(iv) != tokens.GCMIVSize {
		return nil, fmt.Errorf("GCM requires 96-bit iv, got %d", len(iv)*tokens.BitsPerByte)
	}
	if len(tag) != tokens.GCMTagSize {
		return nil, fmt.Errorf("GCM requires 128-bit tag, got %d", len(tag)*tokens.BitsPerByte)
	}

	block, err := aes.NewCipher(sharedkey)
	if err != nil {
		return nil, fmt.Errorf(`failed to create new AES cipher: %w`, err)
	}

	aesgcm, err := cryptocipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf(`failed to create new GCM wrap: %w`, err)
	}

	// Combine recipient key and tag for GCM decryption
	ciphertext := recipientKey[:]
	ciphertext = append(ciphertext, tag...)

	jek, err := aesgcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf(`failed to decode key: %w`, err)
	}

	return jek, nil
}
