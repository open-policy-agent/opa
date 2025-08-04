//go:build jwx_es256k
// +build jwx_es256k

package jwk

import (
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lestrrat-go/jwx/v3/jwa"
	ourecdsa "github.com/lestrrat-go/jwx/v3/jwk/ecdsa"
)

func init() {
	ourecdsa.RegisterCurve(jwa.Secp256k1(), secp256k1.S256())
}
