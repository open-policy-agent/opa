package verify

import (
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
)

func TestRSAVerify(t *testing.T) {
	type dummyStruct struct {
		dummy1 int
		dummy2 float64
	}
	dummy := &dummyStruct{1, 3.4}
	t.Run("RSA Verifier Creation Error", func(t *testing.T) {
		_, err := newRSA(jwa.HS256)
		if err == nil {
			t.Fatal("ECDSA Verifier Object creation should fail")
		}
	})
	t.Run("RSA Verifier Sign Error", func(t *testing.T) {
		pVerifier, err := newRSA(jwa.PS512)
		if err != nil {
			t.Fatalf("Signer creation failure: %v", jwa.ES512)
		}
		err = pVerifier.Verify([]byte("payload"), []byte("signature"), dummy)
		if err == nil {
			t.Fatal("RSA Verification should fail")
		}
		err = pVerifier.Verify([]byte("payload"), []byte("signature"), nil)
		if err == nil {
			t.Fatal("RSA Verification should fail")
		}
	})
}
