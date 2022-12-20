package mux

import "net/http"

type Router interface {
	http.Handler
	// Handle adds an http.Handler at a path.
	//
	// Path can be absolute or relative depending on the implementer.
	Handle(path string, h http.Handler) Route
}

type Route interface {
	Methods(methods ...string) Route
}
