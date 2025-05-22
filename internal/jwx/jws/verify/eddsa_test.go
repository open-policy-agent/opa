package verify

import (
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
)

func TestEdDSAVerify(t *testing.T) {
	type dummyStruct struct {
		dummy1 int
		dummy2 float64
	}
	dummy := &dummyStruct{1, 3.4}
	t.Run("EdDSA Verifier Creation Error", func(t *testing.T) {
		_, err := newEdDSA(jwa.HS256)
		if err == nil {
			t.Fatal("EdDSA Verifier Object creation should fail")
		}
	})
	t.Run("EdDSA Verifier Sign Error", func(t *testing.T) {
		pVerifier, err := newEdDSA(jwa.EdDSA)
		if err != nil {
			t.Fatalf("Signer creation failure: %v", jwa.EdDSA)
		}
		err = pVerifier.Verify([]byte("payload"), []byte("signature"), dummy)
		if err == nil {
			t.Fatal("EdDSA Verification should fail")
		}
		err = pVerifier.Verify([]byte("payload"), []byte("signature"), nil)
		if err == nil {
			t.Fatal("EdDSA Verification should fail")
		}
	})
}
