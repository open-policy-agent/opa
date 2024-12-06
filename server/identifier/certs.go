package identifier

import (
	"crypto/x509"
	"net/http"

	v1 "github.com/open-policy-agent/opa/v1/server/identifier"
)

// ClientCertificates returns the ClientCertificates of the caller associated with ctx.
func ClientCertificates(r *http.Request) ([]*x509.Certificate, bool) {
	return v1.ClientCertificates(r)
}

// SetClientCertificates returns a new http.Request with the ClientCertificates set to v.
func SetClientCertificates(r *http.Request, v []*x509.Certificate) *http.Request {
	return v1.SetClientCertificates(r, v)
}
