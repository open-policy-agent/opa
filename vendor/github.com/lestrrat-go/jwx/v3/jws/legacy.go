package jws

import (
	"fmt"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws/legacy"
)

func enableLegacySigners() {
	for _, alg := range []jwa.SignatureAlgorithm{jwa.HS256(), jwa.HS384(), jwa.HS512()} {
		if err := RegisterSigner(alg, func(alg jwa.SignatureAlgorithm) SignerFactory {
			return SignerFactoryFn(func() (Signer, error) {
				return legacy.NewHMACSigner(alg), nil
			})
		}(alg)); err != nil {
			panic(fmt.Sprintf("RegisterSigner failed: %v", err))
		}
		if err := RegisterVerifier(alg, func(alg jwa.SignatureAlgorithm) VerifierFactory {
			return VerifierFactoryFn(func() (Verifier, error) {
				return legacy.NewHMACVerifier(alg), nil
			})
		}(alg)); err != nil {
			panic(fmt.Sprintf("RegisterVerifier failed: %v", err))
		}
	}

	for _, alg := range []jwa.SignatureAlgorithm{jwa.RS256(), jwa.RS384(), jwa.RS512(), jwa.PS256(), jwa.PS384(), jwa.PS512()} {
		if err := RegisterSigner(alg, func(alg jwa.SignatureAlgorithm) SignerFactory {
			return SignerFactoryFn(func() (Signer, error) {
				return legacy.NewRSASigner(alg), nil
			})
		}(alg)); err != nil {
			panic(fmt.Sprintf("RegisterSigner failed: %v", err))
		}
		if err := RegisterVerifier(alg, func(alg jwa.SignatureAlgorithm) VerifierFactory {
			return VerifierFactoryFn(func() (Verifier, error) {
				return legacy.NewRSAVerifier(alg), nil
			})
		}(alg)); err != nil {
			panic(fmt.Sprintf("RegisterVerifier failed: %v", err))
		}
	}
	for _, alg := range []jwa.SignatureAlgorithm{jwa.ES256(), jwa.ES384(), jwa.ES512(), jwa.ES256K()} {
		if err := RegisterSigner(alg, func(alg jwa.SignatureAlgorithm) SignerFactory {
			return SignerFactoryFn(func() (Signer, error) {
				return legacy.NewECDSASigner(alg), nil
			})
		}(alg)); err != nil {
			panic(fmt.Sprintf("RegisterSigner failed: %v", err))
		}
		if err := RegisterVerifier(alg, func(alg jwa.SignatureAlgorithm) VerifierFactory {
			return VerifierFactoryFn(func() (Verifier, error) {
				return legacy.NewECDSAVerifier(alg), nil
			})
		}(alg)); err != nil {
			panic(fmt.Sprintf("RegisterVerifier failed: %v", err))
		}
	}

	if err := RegisterSigner(jwa.EdDSA(), SignerFactoryFn(func() (Signer, error) {
		return legacy.NewEdDSASigner(), nil
	})); err != nil {
		panic(fmt.Sprintf("RegisterSigner failed: %v", err))
	}
	if err := RegisterVerifier(jwa.EdDSA(), VerifierFactoryFn(func() (Verifier, error) {
		return legacy.NewEdDSAVerifier(), nil
	})); err != nil {
		panic(fmt.Sprintf("RegisterVerifier failed: %v", err))
	}
}

func legacySignerFor(alg jwa.SignatureAlgorithm) (Signer, error) {
	muSigner.Lock()
	s, ok := signers[alg]
	if !ok {
		v, err := NewSigner(alg)
		if err != nil {
			muSigner.Unlock()
			return nil, fmt.Errorf(`failed to create payload signer: %w`, err)
		}
		signers[alg] = v
		s = v
	}
	muSigner.Unlock()

	return s, nil
}
