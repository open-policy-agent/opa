//go:build !usegorillamux

package server

import (
	"github.com/open-policy-agent/opa/mux"
)

// WithRouter sets the mux.Router to attach OPA's HTTP API routes onto. If a
// router is not supplied, the server will create it's own.
func (s *Server) WithRouter(router mux.Router) *Server {
	s.router = router
	return s
}
