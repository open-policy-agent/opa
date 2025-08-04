//go:build jwx_es256k
// +build jwx_es256k

package jwa

var secp256k1Algorithm = NewEllipticCurveAlgorithm("secp256k1")

// This constant is only available if compiled with jwx_es256k build tag
func Secp256k1() EllipticCurveAlgorithm {
	return secp256k1Algorithm
}

func init() {
	RegisterEllipticCurveAlgorithm(secp256k1Algorithm)
}
