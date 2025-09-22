//go:build jwx_es256k
// +build jwx_es256k

package jws

import (
	"github.com/lestrrat-go/jwx/v3/jwa"
)

func init() {
	// Register ES256K to EC algorithm family
	addAlgorithmForKeyType(jwa.EC(), jwa.ES256K())
}
