package content_crypt //nolint:golint

import (
	"fmt"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/cipher"
)

func (c Generic) Algorithm() jwa.ContentEncryptionAlgorithm {
	return c.alg
}

func (c Generic) Encrypt(cek, plaintext, aad []byte) ([]byte, []byte, []byte, error) {
	iv, encrypted, tag, err := c.cipher.Encrypt(cek, plaintext, aad)
	if err != nil {
		return nil, nil, nil, fmt.Errorf(`failed to crypt content: %w`, err)
	}

	return iv, encrypted, tag, nil
}

func (c Generic) Decrypt(cek, iv, ciphertext, tag, aad []byte) ([]byte, error) {
	return c.cipher.Decrypt(cek, iv, ciphertext, tag, aad)
}

func NewGeneric(alg jwa.ContentEncryptionAlgorithm) (*Generic, error) {
	c, err := cipher.NewAES(alg.String())
	if err != nil {
		return nil, fmt.Errorf(`aes crypt: failed to create content cipher: %w`, err)
	}

	return &Generic{
		alg:     alg,
		cipher:  c,
		keysize: c.KeySize(),
		tagsize: 16,
	}, nil
}

func (c Generic) KeySize() int {
	return c.keysize
}
