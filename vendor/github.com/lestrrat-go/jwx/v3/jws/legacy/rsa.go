package legacy

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"fmt"

	"github.com/lestrrat-go/jwx/v3/internal/keyconv"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws/internal/keytype"
)

var rsaSigners = make(map[jwa.SignatureAlgorithm]*rsaSigner)
var rsaVerifiers = make(map[jwa.SignatureAlgorithm]*rsaVerifier)

func init() {
	data := map[jwa.SignatureAlgorithm]struct {
		Hash crypto.Hash
		PSS  bool
	}{
		jwa.RS256(): {
			Hash: crypto.SHA256,
		},
		jwa.RS384(): {
			Hash: crypto.SHA384,
		},
		jwa.RS512(): {
			Hash: crypto.SHA512,
		},
		jwa.PS256(): {
			Hash: crypto.SHA256,
			PSS:  true,
		},
		jwa.PS384(): {
			Hash: crypto.SHA384,
			PSS:  true,
		},
		jwa.PS512(): {
			Hash: crypto.SHA512,
			PSS:  true,
		},
	}

	for alg, item := range data {
		rsaSigners[alg] = &rsaSigner{
			alg:  alg,
			hash: item.Hash,
			pss:  item.PSS,
		}
		rsaVerifiers[alg] = &rsaVerifier{
			alg:  alg,
			hash: item.Hash,
			pss:  item.PSS,
		}
	}
}

type rsaSigner struct {
	alg  jwa.SignatureAlgorithm
	hash crypto.Hash
	pss  bool
}

func NewRSASigner(alg jwa.SignatureAlgorithm) Signer {
	return rsaSigners[alg]
}

func (rs *rsaSigner) Algorithm() jwa.SignatureAlgorithm {
	return rs.alg
}

func (rs *rsaSigner) Sign(payload []byte, key any) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf(`missing private key while signing payload`)
	}

	signer, ok := key.(crypto.Signer)
	if ok {
		if !keytype.IsValidRSAKey(key) {
			return nil, fmt.Errorf(`cannot use key of type %T to generate RSA based signatures`, key)
		}
	} else {
		var privkey rsa.PrivateKey
		if err := keyconv.RSAPrivateKey(&privkey, key); err != nil {
			return nil, fmt.Errorf(`failed to retrieve rsa.PrivateKey out of %T: %w`, key, err)
		}
		signer = &privkey
	}

	h := rs.hash.New()
	if _, err := h.Write(payload); err != nil {
		return nil, fmt.Errorf(`failed to write payload to hash: %w`, err)
	}
	if rs.pss {
		return signer.Sign(rand.Reader, h.Sum(nil), &rsa.PSSOptions{
			Hash:       rs.hash,
			SaltLength: rsa.PSSSaltLengthEqualsHash,
		})
	}
	return signer.Sign(rand.Reader, h.Sum(nil), rs.hash)
}

type rsaVerifier struct {
	alg  jwa.SignatureAlgorithm
	hash crypto.Hash
	pss  bool
}

func NewRSAVerifier(alg jwa.SignatureAlgorithm) Verifier {
	return rsaVerifiers[alg]
}

func (rv *rsaVerifier) Verify(payload, signature []byte, key any) error {
	if key == nil {
		return fmt.Errorf(`missing public key while verifying payload`)
	}

	var pubkey rsa.PublicKey
	if cs, ok := key.(crypto.Signer); ok {
		cpub := cs.Public()
		switch cpub := cpub.(type) {
		case rsa.PublicKey:
			pubkey = cpub
		case *rsa.PublicKey:
			pubkey = *cpub
		default:
			return fmt.Errorf(`failed to retrieve rsa.PublicKey out of crypto.Signer %T`, key)
		}
	} else {
		if err := keyconv.RSAPublicKey(&pubkey, key); err != nil {
			return fmt.Errorf(`failed to retrieve rsa.PublicKey out of %T: %w`, key, err)
		}
	}

	h := rv.hash.New()
	if _, err := h.Write(payload); err != nil {
		return fmt.Errorf(`failed to write payload to hash: %w`, err)
	}

	if rv.pss {
		return rsa.VerifyPSS(&pubkey, rv.hash, h.Sum(nil), signature, nil)
	}
	return rsa.VerifyPKCS1v15(&pubkey, rv.hash, h.Sum(nil), signature)
}
