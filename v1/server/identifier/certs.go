package identifier

import (
	"context"
	"crypto/x509"
	"net/http"
)

type clientCertificatesKey string

const clientCertificates = clientCertificatesKey("org.openpolicyagent/client-certificates")

// ClientCertificates returns the ClientCertificates of the caller associated with ctx.
func ClientCertificates(r *http.Request) ([]*x509.Certificate, bool) {
	ctx := r.Context()

	certs, ok := ctx.Value(clientCertificates).([]*x509.Certificate)

	return certs, ok
}

// SetClientCertificates returns a new http.Request with the ClientCertificates set to v.
func SetClientCertificates(r *http.Request, v []*x509.Certificate) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), clientCertificates, v))
}
