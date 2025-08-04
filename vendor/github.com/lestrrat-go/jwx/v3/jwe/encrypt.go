package jwe

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/rsa"
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/keyconv"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/content_crypt"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/keygen"
	"github.com/lestrrat-go/jwx/v3/jwe/jwebb"
)

// encrypter is responsible for taking various components to encrypt a key.
// its operation is not concurrency safe. You must provide locking yourself
//
//nolint:govet
type encrypter struct {
	apu    []byte
	apv    []byte
	ctalg  jwa.ContentEncryptionAlgorithm
	keyalg jwa.KeyEncryptionAlgorithm
	pubkey any
	rawKey any
	cipher content_crypt.Cipher
}

// newEncrypter creates a new Encrypter instance with all required parameters.
// The content cipher is built internally during construction.
//
// pubkey must be a public key in its "raw" format (i.e. something like
// *rsa.PublicKey, instead of jwk.Key)
//
// You should consider this object immutable once created.
func newEncrypter(keyalg jwa.KeyEncryptionAlgorithm, ctalg jwa.ContentEncryptionAlgorithm, pubkey any, rawKey any, apu, apv []byte) (*encrypter, error) {
	cipher, err := jwebb.CreateContentCipher(ctalg.String())
	if err != nil {
		return nil, fmt.Errorf(`failed to create content cipher: %w`, err)
	}

	return &encrypter{
		apu:    apu,
		apv:    apv,
		ctalg:  ctalg,
		keyalg: keyalg,
		pubkey: pubkey,
		rawKey: rawKey,
		cipher: cipher,
	}, nil
}

func (e *encrypter) EncryptKey(cek []byte) (keygen.ByteSource, error) {
	if ke, ok := e.pubkey.(KeyEncrypter); ok {
		encrypted, err := ke.EncryptKey(cek)
		if err != nil {
			return nil, err
		}
		return keygen.ByteKey(encrypted), nil
	}

	if jwebb.IsDirect(e.keyalg.String()) {
		sharedkey, ok := e.rawKey.([]byte)
		if !ok {
			return nil, fmt.Errorf("encrypt key: []byte is required as the key for %s (got %T)", e.keyalg, e.rawKey)
		}
		return jwebb.KeyEncryptDirect(cek, e.keyalg.String(), sharedkey)
	}

	if jwebb.IsPBES2(e.keyalg.String()) {
		password, ok := e.rawKey.([]byte)
		if !ok {
			return nil, fmt.Errorf("encrypt key: []byte is required as the password for %s (got %T)", e.keyalg, e.rawKey)
		}
		return jwebb.KeyEncryptPBES2(cek, e.keyalg.String(), password)
	}

	if jwebb.IsAESGCMKW(e.keyalg.String()) {
		sharedkey, ok := e.rawKey.([]byte)
		if !ok {
			return nil, fmt.Errorf("encrypt key: []byte is required as the key for %s (got %T)", e.keyalg, e.rawKey)
		}
		return jwebb.KeyEncryptAESGCMKW(cek, e.keyalg.String(), sharedkey)
	}

	if jwebb.IsECDHES(e.keyalg.String()) {
		_, keysize, keywrap, err := jwebb.KeyEncryptionECDHESKeySize(e.keyalg.String(), e.ctalg.String())
		if err != nil {
			return nil, fmt.Errorf(`failed to determine ECDH-ES key size: %w`, err)
		}

		// Use rawKey for ECDH-ES operations - it should contain the actual key material
		keyToUse := e.rawKey
		if keyToUse == nil {
			keyToUse = e.pubkey
		}

		// Handle ecdsa.PublicKey by value - convert to pointer
		if pk, ok := keyToUse.(ecdsa.PublicKey); ok {
			keyToUse = &pk
		}

		// Determine key type and call appropriate function
		switch key := keyToUse.(type) {
		case *ecdsa.PublicKey:
			if !keywrap {
				return jwebb.KeyEncryptECDHESECDSA(cek, e.keyalg.String(), e.apu, e.apv, key, keysize, e.ctalg.String())
			}
			return jwebb.KeyEncryptECDHESKeyWrapECDSA(cek, e.keyalg.String(), e.apu, e.apv, key, keysize, e.ctalg.String())
		case *ecdh.PublicKey:
			if !keywrap {
				return jwebb.KeyEncryptECDHESX25519(cek, e.keyalg.String(), e.apu, e.apv, key, keysize, e.ctalg.String())
			}
			return jwebb.KeyEncryptECDHESKeyWrapX25519(cek, e.keyalg.String(), e.apu, e.apv, key, keysize, e.ctalg.String())
		default:
			return nil, fmt.Errorf(`encrypt: unsupported key type for ECDH-ES: %T`, keyToUse)
		}
	}

	if jwebb.IsRSA15(e.keyalg.String()) {
		keyToUse := e.rawKey
		if keyToUse == nil {
			keyToUse = e.pubkey
		}

		// Handle rsa.PublicKey by value - convert to pointer
		if pk, ok := keyToUse.(rsa.PublicKey); ok {
			keyToUse = &pk
		}

		var pubkey *rsa.PublicKey
		if err := keyconv.RSAPublicKey(&pubkey, keyToUse); err != nil {
			return nil, fmt.Errorf(`encrypt: failed to convert to RSA public key: %w`, err)
		}

		return jwebb.KeyEncryptRSA15(cek, e.keyalg.String(), pubkey)
	}

	if jwebb.IsRSAOAEP(e.keyalg.String()) {
		keyToUse := e.rawKey
		if keyToUse == nil {
			keyToUse = e.pubkey
		}

		// Handle rsa.PublicKey by value - convert to pointer
		if pk, ok := keyToUse.(rsa.PublicKey); ok {
			keyToUse = &pk
		}

		var pubkey *rsa.PublicKey
		if err := keyconv.RSAPublicKey(&pubkey, keyToUse); err != nil {
			return nil, fmt.Errorf(`encrypt: failed to convert to RSA public key: %w`, err)
		}

		return jwebb.KeyEncryptRSAOAEP(cek, e.keyalg.String(), pubkey)
	}

	if jwebb.IsAESKW(e.keyalg.String()) {
		sharedkey, ok := e.rawKey.([]byte)
		if !ok {
			return nil, fmt.Errorf("[]byte is required as the key to encrypt %s", e.keyalg.String())
		}
		return jwebb.KeyEncryptAESKW(cek, e.keyalg.String(), sharedkey)
	}

	return nil, fmt.Errorf(`unsupported algorithm for key encryption (%s)`, e.keyalg)
}
