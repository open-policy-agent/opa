package dsigsecp256k1

import (
	"crypto"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lestrrat-go/dsig"
)

const ECDSAWithSecp256k1AndSHA256 = "ECDSA_WITH_SECP256K1_AND_SHA256"

// init adds secp256k1 support when the dsig_secp256k1 build tag is used.
func init() {
	// Register ES256K (secp256k1 + SHA256) support using the new API
	err := dsig.RegisterAlgorithm(ECDSAWithSecp256k1AndSHA256, dsig.AlgorithmInfo{
		Family: dsig.ECDSA,
		Meta: dsig.ECDSAFamilyMeta{
			Hash: crypto.SHA256,
		},
	})
	if err != nil {
		panic("failed to register secp256k1 algorithm: " + err.Error())
	}
}

// secp256k1Curve returns the secp256k1 curve.
func Curve() *secp256k1.KoblitzCurve {
	return secp256k1.S256()
}
