package legacy

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/asn1"
	"fmt"
	"math/big"

	"github.com/lestrrat-go/jwx/v3/internal/ecutil"
	"github.com/lestrrat-go/jwx/v3/internal/keyconv"
	"github.com/lestrrat-go/jwx/v3/internal/pool"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jws/internal/keytype"
)

var ecdsaSigners = make(map[jwa.SignatureAlgorithm]*ecdsaSigner)
var ecdsaVerifiers = make(map[jwa.SignatureAlgorithm]*ecdsaVerifier)

func init() {
	algs := map[jwa.SignatureAlgorithm]crypto.Hash{
		jwa.ES256():  crypto.SHA256,
		jwa.ES384():  crypto.SHA384,
		jwa.ES512():  crypto.SHA512,
		jwa.ES256K(): crypto.SHA256,
	}
	for alg, hash := range algs {
		ecdsaSigners[alg] = &ecdsaSigner{
			alg:  alg,
			hash: hash,
		}
		ecdsaVerifiers[alg] = &ecdsaVerifier{
			alg:  alg,
			hash: hash,
		}
	}
}

func NewECDSASigner(alg jwa.SignatureAlgorithm) Signer {
	return ecdsaSigners[alg]
}

// ecdsaSigners are immutable.
type ecdsaSigner struct {
	alg  jwa.SignatureAlgorithm
	hash crypto.Hash
}

func (es ecdsaSigner) Algorithm() jwa.SignatureAlgorithm {
	return es.alg
}

func (es *ecdsaSigner) Sign(payload []byte, key any) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf(`missing private key while signing payload`)
	}

	h := es.hash.New()
	if _, err := h.Write(payload); err != nil {
		return nil, fmt.Errorf(`failed to write payload using ecdsa: %w`, err)
	}

	signer, ok := key.(crypto.Signer)
	if ok {
		if !keytype.IsValidECDSAKey(key) {
			return nil, fmt.Errorf(`cannot use key of type %T to generate ECDSA based signatures`, key)
		}
		switch key.(type) {
		case ecdsa.PrivateKey, *ecdsa.PrivateKey:
			// if it's a ecdsa.PrivateKey, it's more efficient to
			// go through the non-crypto.Signer route. Set ok to false
			ok = false
		}
	}

	var r, s *big.Int
	var curveBits int
	if ok {
		signed, err := signer.Sign(rand.Reader, h.Sum(nil), es.hash)
		if err != nil {
			return nil, err
		}

		var p struct {
			R *big.Int
			S *big.Int
		}
		if _, err := asn1.Unmarshal(signed, &p); err != nil {
			return nil, fmt.Errorf(`failed to unmarshal ASN1 encoded signature: %w`, err)
		}

		// Okay, this is silly, but hear me out. When we use the
		// crypto.Signer interface, the PrivateKey is hidden.
		// But we need some information about the key (its bit size).
		//
		// So while silly, we're going to have to make another call
		// here and fetch the Public key.
		// This probably means that this should be cached some where.
		cpub := signer.Public()
		pubkey, ok := cpub.(*ecdsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf(`expected *ecdsa.PublicKey, got %T`, pubkey)
		}
		curveBits = pubkey.Curve.Params().BitSize

		r = p.R
		s = p.S
	} else {
		var privkey ecdsa.PrivateKey
		if err := keyconv.ECDSAPrivateKey(&privkey, key); err != nil {
			return nil, fmt.Errorf(`failed to retrieve ecdsa.PrivateKey out of %T: %w`, key, err)
		}
		curveBits = privkey.Curve.Params().BitSize
		rtmp, stmp, err := ecdsa.Sign(rand.Reader, &privkey, h.Sum(nil))
		if err != nil {
			return nil, fmt.Errorf(`failed to sign payload using ecdsa: %w`, err)
		}
		r = rtmp
		s = stmp
	}

	keyBytes := curveBits / 8
	// Curve bits do not need to be a multiple of 8.
	if curveBits%8 > 0 {
		keyBytes++
	}

	rBytes := r.Bytes()
	rBytesPadded := make([]byte, keyBytes)
	copy(rBytesPadded[keyBytes-len(rBytes):], rBytes)

	sBytes := s.Bytes()
	sBytesPadded := make([]byte, keyBytes)
	copy(sBytesPadded[keyBytes-len(sBytes):], sBytes)

	out := append(rBytesPadded, sBytesPadded...)

	return out, nil
}

// ecdsaVerifiers are immutable.
type ecdsaVerifier struct {
	alg  jwa.SignatureAlgorithm
	hash crypto.Hash
}

func NewECDSAVerifier(alg jwa.SignatureAlgorithm) Verifier {
	return ecdsaVerifiers[alg]
}

func (v ecdsaVerifier) Algorithm() jwa.SignatureAlgorithm {
	return v.alg
}

func (v *ecdsaVerifier) Verify(payload []byte, signature []byte, key any) error {
	if key == nil {
		return fmt.Errorf(`missing public key while verifying payload`)
	}

	var pubkey ecdsa.PublicKey
	if cs, ok := key.(crypto.Signer); ok {
		cpub := cs.Public()
		switch cpub := cpub.(type) {
		case ecdsa.PublicKey:
			pubkey = cpub
		case *ecdsa.PublicKey:
			pubkey = *cpub
		default:
			return fmt.Errorf(`failed to retrieve ecdsa.PublicKey out of crypto.Signer %T`, key)
		}
	} else {
		if err := keyconv.ECDSAPublicKey(&pubkey, key); err != nil {
			return fmt.Errorf(`failed to retrieve ecdsa.PublicKey out of %T: %w`, key, err)
		}
	}

	if !pubkey.Curve.IsOnCurve(pubkey.X, pubkey.Y) {
		return fmt.Errorf(`public key used does not contain a point (X,Y) on the curve`)
	}

	r := pool.BigInt().Get()
	s := pool.BigInt().Get()
	defer pool.BigInt().Put(r)
	defer pool.BigInt().Put(s)

	keySize := ecutil.CalculateKeySize(pubkey.Curve)
	if len(signature) != keySize*2 {
		return fmt.Errorf(`invalid signature length for curve %q`, pubkey.Curve.Params().Name)
	}

	r.SetBytes(signature[:keySize])
	s.SetBytes(signature[keySize:])

	h := v.hash.New()
	if _, err := h.Write(payload); err != nil {
		return fmt.Errorf(`failed to write payload using ecdsa: %w`, err)
	}

	if !ecdsa.Verify(&pubkey, h.Sum(nil), r, s) {
		return fmt.Errorf(`failed to verify signature using ecdsa`)
	}
	return nil
}
