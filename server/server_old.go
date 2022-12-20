//go:build usegorillamux

package server

import (
	"net/http"

	"github.com/gorilla/mux"
	muxproto "github.com/open-policy-agent/opa/mux"
)

// WithRouter sets the mux.Router to attach OPA's HTTP API routes onto. If a
// router is not supplied, the server will create it's own.
func (s *Server) WithRouter(router *mux.Router) *Server {
	s.router = &gorillaWrapper{router: router}
	return s
}

// gorillaWrapper is a private struct used for implementing muxproto.Router for gorilla/mux.Router.
type gorillaWrapper struct {
	router *mux.Router
}

var _ muxproto.Router = (*gorillaWrapper)(nil)

func (g *gorillaWrapper) Handle(path string, h http.Handler) muxproto.Route {
	return &gorillaRouteWrapper{inner: g.router.Handle(path, h)}
}

func (g *gorillaWrapper) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	g.router.ServeHTTP(w, req)
}

// gorillaRouteWrapper is a private struct used for implementing muxproto.Route for gorilla/mux.Route.
type gorillaRouteWrapper struct {
	inner *mux.Route
}

var _ muxproto.Route = (*gorillaRouteWrapper)(nil)

func (g *gorillaRouteWrapper) Methods(methods ...string) muxproto.Route {
	g.inner.Methods(methods...)
	return g
}
