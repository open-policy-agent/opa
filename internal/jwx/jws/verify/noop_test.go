package verify

import (
	"testing"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
)

func TestNOOPVerify(t *testing.T) {

	t.Run("NOOPVerifier Create", func(t *testing.T) {
		_, err := newNOOPVerifier()
		if err != nil {
			t.Fatalf("Verifier creation failure: %v", err)
		}
	})

	t.Run("NOOPVerifier Verify", func(t *testing.T) {
		test := struct {
			payload   []byte
			signature []byte
			key       interface{}
		}{
			[]byte("foo"), []byte("bar"), 5,
		}

		verifier, err := newNOOPVerifier()
		if err != nil {
			t.Fatalf("Signer creation failure: %v", jwa.NoSignature)
		}

		err = verifier.Verify(test.payload, test.signature, test.key)
		if err != nil {
			t.Fatalf("Expected verify to succeed: %v", err)
		}
	})
}
