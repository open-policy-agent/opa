//go:build jwx_es256k

package jwsbb

import (
	dsigsecp256k1 "github.com/lestrrat-go/dsig-secp256k1"
)

const es256k = "ES256K"

func init() {
	// Add ES256K mapping when this build tag is enabled
	jwsToDsigAlgorithm[es256k] = dsigsecp256k1.ECDSAWithSecp256k1AndSHA256
}
