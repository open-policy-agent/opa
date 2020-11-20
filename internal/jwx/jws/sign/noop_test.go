package sign

import (
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
)

func TestNOOPSign(t *testing.T) {

	t.Run("NOOPSigner Create", func(t *testing.T) {
		_, err := newNOOPSigner()
		if err != nil {
			t.Fatalf("Signer creation failure: %v", err)
		}
	})

	t.Run("NOOPSigner Sign", func(t *testing.T) {
		test := struct {
			payload []byte
			key     interface{}
		}{
			[]byte("hello"), 5,
		}

		signer, err := newNOOPSigner()
		if err != nil {
			t.Fatalf("Signer creation failure: %v", jwa.NoSignature)
		}

		sig, err := signer.Sign(test.payload, test.key)
		if err != nil {
			t.Fatalf("Expected Sign to succeed: %v", err)
		}
		if sig != nil {
			t.Fatalf("Expected no signature to be created")
		}
	})
}
