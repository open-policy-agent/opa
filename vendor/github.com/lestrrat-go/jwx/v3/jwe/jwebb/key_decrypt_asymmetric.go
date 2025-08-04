package jwebb

import (
	"crypto"
	"crypto/aes"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"hash"

	"github.com/lestrrat-go/jwx/v3/internal/keyconv"
	"github.com/lestrrat-go/jwx/v3/internal/tokens"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/concatkdf"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/keygen"
)

func contentEncryptionKeySize(ctalg string) (uint32, error) {
	switch ctalg {
	case tokens.A128GCM:
		return tokens.KeySize16, nil
	case tokens.A192GCM:
		return tokens.KeySize24, nil
	case tokens.A256GCM:
		return tokens.KeySize32, nil
	case tokens.A128CBC_HS256:
		return tokens.KeySize32, nil
	case tokens.A192CBC_HS384:
		return tokens.KeySize48, nil
	case tokens.A256CBC_HS512:
		return tokens.KeySize64, nil
	default:
		return 0, fmt.Errorf(`unsupported content encryption algorithm %s`, ctalg)
	}
}

func KeyEncryptionECDHESKeySize(alg, ctalg string) (string, uint32, bool, error) {
	switch alg {
	case tokens.ECDH_ES:
		keysize, err := contentEncryptionKeySize(ctalg)
		if err != nil {
			return "", 0, false, err
		}
		return ctalg, keysize, false, nil
	case tokens.ECDH_ES_A128KW:
		return alg, tokens.KeySize16, true, nil
	case tokens.ECDH_ES_A192KW:
		return alg, tokens.KeySize24, true, nil
	case tokens.ECDH_ES_A256KW:
		return alg, tokens.KeySize32, true, nil
	default:
		return "", 0, false, fmt.Errorf(`unsupported key encryption algorithm %s`, alg)
	}
}

func DeriveECDHES(alg string, apu, apv []byte, privkeyif, pubkeyif any, keysize uint32) ([]byte, error) {
	pubinfo := make([]byte, 4)
	binary.BigEndian.PutUint32(pubinfo, keysize*tokens.BitsPerByte)

	var privkey *ecdh.PrivateKey
	var pubkey *ecdh.PublicKey
	if err := keyconv.ECDHPrivateKey(&privkey, privkeyif); err != nil {
		return nil, fmt.Errorf(`jwebb.DeriveECDHES: %w`, err)
	}
	if err := keyconv.ECDHPublicKey(&pubkey, pubkeyif); err != nil {
		return nil, fmt.Errorf(`jwebb.DeriveECDHES: %w`, err)
	}

	zBytes, err := privkey.ECDH(pubkey)
	if err != nil {
		return nil, fmt.Errorf(`jwebb.DeriveECDHES: unable to determine Z: %w`, err)
	}
	kdf := concatkdf.New(crypto.SHA256, []byte(alg), zBytes, apu, apv, pubinfo, []byte{})
	key := make([]byte, keysize)
	if _, err := kdf.Read(key); err != nil {
		return nil, fmt.Errorf(`jwebb.DeriveECDHES: failed to read kdf: %w`, err)
	}

	return key, nil
}

func KeyDecryptECDHESKeyWrap(_, enckey []byte, alg string, apu, apv []byte, privkey, pubkey any, keysize uint32) ([]byte, error) {
	key, err := DeriveECDHES(alg, apu, apv, privkey, pubkey, keysize)
	if err != nil {
		return nil, fmt.Errorf(`failed to derive ECDHES encryption key: %w`, err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf(`failed to create cipher for ECDH-ES key wrap: %w`, err)
	}

	return Unwrap(block, enckey)
}

func KeyDecryptECDHES(_, _ []byte, alg string, apu, apv []byte, privkey, pubkey any, keysize uint32) ([]byte, error) {
	key, err := DeriveECDHES(alg, apu, apv, privkey, pubkey, keysize)
	if err != nil {
		return nil, fmt.Errorf(`failed to derive ECDHES encryption key: %w`, err)
	}
	return key, nil
}

// RSA key decryption functions

func KeyDecryptRSA15(_, enckey []byte, privkeyif any, keysize int) ([]byte, error) {
	var privkey *rsa.PrivateKey
	if err := keyconv.RSAPrivateKey(&privkey, privkeyif); err != nil {
		return nil, fmt.Errorf(`jwebb.KeyDecryptRSA15: %w`, err)
	}

	// Perform some input validation.
	expectedlen := privkey.PublicKey.N.BitLen() / tokens.BitsPerByte
	if expectedlen != len(enckey) {
		// Input size is incorrect, the encrypted payload should always match
		// the size of the public modulus (e.g. using a 2048 bit key will
		// produce 256 bytes of output). Reject this since it's invalid input.
		return nil, fmt.Errorf(
			"input size for key decrypt is incorrect (expected %d, got %d)",
			expectedlen,
			len(enckey),
		)
	}

	// Generate a random CEK of the required size
	bk, err := keygen.Random(keysize * tokens.RSAKeyGenMultiplier)
	if err != nil {
		return nil, fmt.Errorf(`failed to generate key`)
	}
	cek := bk.Bytes()

	// Use a defer/recover pattern to handle potential panics from DecryptPKCS1v15SessionKey
	defer func() {
		// DecryptPKCS1v15SessionKey sometimes panics on an invalid payload
		// because of an index out of bounds error, which we want to ignore.
		// This has been fixed in Go 1.3.1 (released 2014/08/13), the recover()
		// only exists for preventing crashes with unpatched versions.
		// See: https://groups.google.com/forum/#!topic/golang-dev/7ihX6Y6kx9k
		// See: https://code.google.com/p/go/source/detail?r=58ee390ff31602edb66af41ed10901ec95904d33
		_ = recover()
	}()

	// When decrypting an RSA-PKCS1v1.5 payload, we must take precautions to
	// prevent chosen-ciphertext attacks as described in RFC 3218, "Preventing
	// the Million Message Attack on Cryptographic Message Syntax". We are
	// therefore deliberately ignoring errors here.
	_ = rsa.DecryptPKCS1v15SessionKey(rand.Reader, privkey, enckey, cek)

	return cek, nil
}

func KeyDecryptRSAOAEP(_, enckey []byte, alg string, privkeyif any) ([]byte, error) {
	var privkey *rsa.PrivateKey
	if err := keyconv.RSAPrivateKey(&privkey, privkeyif); err != nil {
		return nil, fmt.Errorf(`jwebb.KeyDecryptRSAOAEP: %w`, err)
	}

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

	return rsa.DecryptOAEP(hash, rand.Reader, privkey, enckey, []byte{})
}
