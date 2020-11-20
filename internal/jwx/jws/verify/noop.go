package verify

func newNOOPVerifier() (*NOOPVerifier, error) {
	return &NOOPVerifier{}, nil
}

// Verify does not verify the payload
func (n NOOPVerifier) Verify(payload []byte, signature []byte, key interface{}) error {
	return nil
}
