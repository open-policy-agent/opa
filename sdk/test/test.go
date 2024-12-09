package test

import (
	"github.com/open-policy-agent/opa/ast"
	v1 "github.com/open-policy-agent/opa/v1/sdk/test"
)

// MockBundle sets a bundle named file on the test server containing the given
// policies.
func MockBundle(file string, policies map[string]string) func(*Server) error {
	return v1.MockBundle(file, policies)
}

// MockOCIBundle prepares the server to allow serving "/v2" OCI responses from the supplied policies
// Ref parameter must be in the form of <registry>/<org>/<repo>:<tag> that will be used in detecting future calls
func MockOCIBundle(ref string, policies map[string]string) func(*Server) error {
	return v1.MockOCIBundle(ref, policies)
}

// Ready provides a channel that the server will use to gate readiness. The
// caller can provide this channel to prevent the server from becoming ready.
// The server will response with HTTP 500 responses until ready. The caller
// should close the channel to indicate readiness.
func Ready(ch chan struct{}) func(*Server) error {
	return v1.Ready(ch)
}

// Server provides a mock HTTP server for testing the SDK and integrations.
type Server = v1.Server

// MustNewServer returns a new Server for test purposes or panics if an error occurs.
func MustNewServer(opts ...func(*Server) error) *Server {
	return v1.MustNewServer(setRegoVersion(opts)...)
}

// NewServer returns a new Server for test purposes.
func NewServer(opts ...func(*Server) error) (*Server, error) {
	return v1.NewServer(setRegoVersion(opts)...)
}

func RawBundles(raw bool) func(*Server) error {
	return v1.RawBundles(raw)
}

// ParserOptions sets the ast.ParserOptions to use when parsing modules when preparing bundles.
func ParserOptions(popts ast.ParserOptions) func(*Server) error {
	return v1.ParserOptions(popts)
}

func setRegoVersion(opts []func(*Server) error) []func(*v1.Server) error {
	cpy := make([]func(*v1.Server) error, 0, len(opts)+1)
	cpy = append(cpy, opts...)

	// Sets rego-version to default (v0) if not set.
	// Must be last in list of options.
	cpy = append(cpy, func(s *v1.Server) error {
		if popts := s.ParserOptions(); popts.RegoVersion == ast.RegoUndefined {
			popts.RegoVersion = ast.DefaultRegoVersion
			return ParserOptions(popts)(s)
		}
		return nil
	})

	return cpy
}
