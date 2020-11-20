package sign

import "github.com/open-policy-agent/opa/internal/jwx/jwa"

func newNOOPSigner() (Signer, error) {
	return &NOOPSigner{}, nil
}

// Algorithm returns the signer algorithm
func (n NOOPSigner) Algorithm() jwa.SignatureAlgorithm {
	return jwa.NoSignature
}

// Sign is a NOOP that does nothing to the provided payload
func (n NOOPSigner) Sign(payload []byte, key interface{}) ([]byte, error) {
	return nil, nil
}
