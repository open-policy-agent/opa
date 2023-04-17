package verify

import "testing"

func TestVerifyErrors(t *testing.T) {

	t.Run("Invalid alg", func(t *testing.T) {
		_, err := New("dummy")
		if err == nil {
			t.Fatal("Verifier creation should have failed")
		}
	})
}
