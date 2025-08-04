package jwebb

import (
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/tokens"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/cipher"
	"github.com/lestrrat-go/jwx/v3/jwe/internal/content_crypt"
)

// ContentEncryptionIsSupported checks if the content encryption algorithm is supported
func ContentEncryptionIsSupported(alg string) bool {
	switch alg {
	case tokens.A128GCM, tokens.A192GCM, tokens.A256GCM,
		tokens.A128CBC_HS256, tokens.A192CBC_HS384, tokens.A256CBC_HS512:
		return true
	default:
		return false
	}
}

// CreateContentCipher creates a content encryption cipher for the given algorithm string
func CreateContentCipher(alg string) (content_crypt.Cipher, error) {
	if !ContentEncryptionIsSupported(alg) {
		return nil, fmt.Errorf(`invalid content cipher algorithm (%s)`, alg)
	}

	cipher, err := cipher.NewAES(alg)
	if err != nil {
		return nil, fmt.Errorf(`failed to build content cipher for %s: %w`, alg, err)
	}

	return cipher, nil
}
