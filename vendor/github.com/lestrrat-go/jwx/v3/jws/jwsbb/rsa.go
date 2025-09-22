package jwsbb

import (
	"crypto"
	"crypto/rsa"
	"fmt"
	"io"

	"github.com/lestrrat-go/dsig"
)

// rsaHashToDsigAlgorithm maps RSA hash functions to dsig algorithm constants
func rsaHashToDsigAlgorithm(h crypto.Hash, pss bool) (string, error) {
	if pss {
		switch h {
		case crypto.SHA256:
			return dsig.RSAPSSWithSHA256, nil
		case crypto.SHA384:
			return dsig.RSAPSSWithSHA384, nil
		case crypto.SHA512:
			return dsig.RSAPSSWithSHA512, nil
		default:
			return "", fmt.Errorf("unsupported hash algorithm for RSA-PSS: %v", h)
		}
	} else {
		switch h {
		case crypto.SHA256:
			return dsig.RSAPKCS1v15WithSHA256, nil
		case crypto.SHA384:
			return dsig.RSAPKCS1v15WithSHA384, nil
		case crypto.SHA512:
			return dsig.RSAPKCS1v15WithSHA512, nil
		default:
			return "", fmt.Errorf("unsupported hash algorithm for RSA PKCS#1 v1.5: %v", h)
		}
	}
}

// SignRSA generates an RSA signature for the given payload using the specified private key and options.
// The raw parameter should be the pre-computed signing input (typically header.payload).
// If pss is true, RSA-PSS is used; otherwise, PKCS#1 v1.5 is used.
//
// The rr parameter is an optional io.Reader that can be used to provide randomness for signing.
// If rr is nil, it defaults to rand.Reader.
//
// This function is now a thin wrapper around dsig.SignRSA. For new projects, you should
// consider using dsig instead of this function.
func SignRSA(key *rsa.PrivateKey, payload []byte, h crypto.Hash, pss bool, rr io.Reader) ([]byte, error) {
	dsigAlg, err := rsaHashToDsigAlgorithm(h, pss)
	if err != nil {
		return nil, fmt.Errorf("jwsbb.SignRSA: %w", err)
	}

	return dsig.Sign(key, dsigAlg, payload, rr)
}

// VerifyRSA verifies an RSA signature for the given payload and header.
// This function constructs the signing input by encoding the header and payload according to JWS specification,
// then verifies the signature using the specified public key and hash algorithm.
// If pss is true, RSA-PSS verification is used; otherwise, PKCS#1 v1.5 verification is used.
//
// This function is now a thin wrapper around dsig.VerifyRSA. For new projects, you should
// consider using dsig instead of this function.
func VerifyRSA(key *rsa.PublicKey, payload, signature []byte, h crypto.Hash, pss bool) error {
	dsigAlg, err := rsaHashToDsigAlgorithm(h, pss)
	if err != nil {
		return fmt.Errorf("jwsbb.VerifyRSA: %w", err)
	}

	return dsig.Verify(key, dsigAlg, payload, signature)
}
