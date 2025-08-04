package jwebb

import (
	"crypto/aes"
	cryptocipher "crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"

	"golang.org/x/crypto/pbkdf2"

	"github.com/lestrrat-go/jwx/v3/internal/tokens"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/keygen"
)

// KeyEncryptAESKW encrypts the CEK using AES key wrap
func KeyEncryptAESKW(cek []byte, _ string, sharedkey []byte) (keygen.ByteSource, error) {
	block, err := aes.NewCipher(sharedkey)
	if err != nil {
		return nil, fmt.Errorf(`failed to create cipher from shared key: %w`, err)
	}

	encrypted, err := Wrap(block, cek)
	if err != nil {
		return nil, fmt.Errorf(`failed to wrap data: %w`, err)
	}
	return keygen.ByteKey(encrypted), nil
}

// KeyEncryptDirect returns the CEK directly for DIRECT algorithm
func KeyEncryptDirect(_ []byte, _ string, sharedkey []byte) (keygen.ByteSource, error) {
	return keygen.ByteKey(sharedkey), nil
}

// KeyEncryptPBES2 encrypts the CEK using PBES2 password-based encryption
func KeyEncryptPBES2(cek []byte, alg string, password []byte) (keygen.ByteSource, error) {
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

	count := tokens.PBES2DefaultIterations
	salt := make([]byte, keylen)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil {
		return nil, fmt.Errorf(`failed to get random salt: %w`, err)
	}

	fullsalt := []byte(alg)
	fullsalt = append(fullsalt, byte(tokens.PBES2NullByteSeparator))
	fullsalt = append(fullsalt, salt...)

	// Derive key using PBKDF2
	derivedKey := pbkdf2.Key(password, fullsalt, count, keylen, hashFunc)

	// Use the derived key for AES key wrap
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf(`failed to create cipher from derived key: %w`, err)
	}
	encrypted, err := Wrap(block, cek)
	if err != nil {
		return nil, fmt.Errorf(`failed to wrap data: %w`, err)
	}

	return keygen.ByteWithSaltAndCount{
		ByteKey: encrypted,
		Salt:    salt,
		Count:   count,
	}, nil
}

// KeyEncryptAESGCMKW encrypts the CEK using AES GCM key wrap
func KeyEncryptAESGCMKW(cek []byte, _ string, sharedkey []byte) (keygen.ByteSource, error) {
	block, err := aes.NewCipher(sharedkey)
	if err != nil {
		return nil, fmt.Errorf(`failed to create new AES cipher: %w`, err)
	}

	aesgcm, err := cryptocipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf(`failed to create new GCM wrap: %w`, err)
	}

	iv := make([]byte, aesgcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, iv)
	if err != nil {
		return nil, fmt.Errorf(`failed to get random iv: %w`, err)
	}

	encrypted := aesgcm.Seal(nil, iv, cek, nil)
	tag := encrypted[len(encrypted)-aesgcm.Overhead():]
	ciphertext := encrypted[:len(encrypted)-aesgcm.Overhead()]

	return keygen.ByteWithIVAndTag{
		ByteKey: ciphertext,
		IV:      iv,
		Tag:     tag,
	}, nil
}
