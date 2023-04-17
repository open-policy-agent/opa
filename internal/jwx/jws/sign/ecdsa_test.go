package sign

import (
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
)

func TestECDSASign(t *testing.T) {
	type dummyStruct struct {
		dummy1 int
		dummy2 float64
	}
	dummy := &dummyStruct{1, 3.4}
	t.Run("ECDSA Creation Error", func(t *testing.T) {
		_, err := newECDSA(jwa.HS256)
		if err == nil {
			t.Fatal("ECDSA Object creation should fail")
		}
	})
	t.Run("ECDSA Sign Error", func(t *testing.T) {
		signer, err := newECDSA(jwa.ES512)
		if err != nil {
			t.Fatalf("Signer creation failure: %v", jwa.ES512)
		}
		_, err = signer.Sign([]byte("payload"), dummy)
		if err == nil {
			t.Fatal("HMAC Object creation should fail")
		}
		_, err = signer.Sign([]byte("payload"), []byte(""))
		if err == nil {
			t.Fatal("HMAC Object creation should fail")
		}
	})
}
