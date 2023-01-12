// Package router contains the interface for how the OPA Server can add handlers and serve http requests
package router

import (
	"context"
	"net/http"
)

// RouterI defines the expected interface that the OPA Server, Runtime and Plugin use.
type RouterI interface {
	http.Handler
	// Handle registers a handler with an http method and path.
	//
	// The way path is interpreted must be compatible with httprouter.
	Handle(method, path string, handler http.Handler)
	// Any registers a handler with any Http handler. It is short for calling Handle repeatedly with all http methods.
	Any(path string, handler http.Handler)
	// EscapePath controls how escaped paths are interpreted by the router. When true, the router must process paths "/data/a%2Fb/c%2Fd" as "/data/a%2Fb/c%2Fd"
	EscapePath(bool)
	// RedirectTrailingSlash controls how the router should behave when a route with a trailing slash is matched
	// to paths without (or vice versa) when the first is undefined but the latter is.
	RedirectTrailingSlash(bool)

	// HandleMethodNotAllowed defines a handler that is called whenever a method does not match, but there are other methods defined for the path.
	HandleMethodNotAllowed(mdlw http.Handler)
}

type routerKey int

const (
	contextVars routerKey = iota
)

// SetParams sets path parameters associated with a request on the copy of the received context.
func SetParams(ctx context.Context, params map[string]string) context.Context {
	return context.WithValue(ctx, contextVars, params)
}

// GetParams returns the parameters associated with a context.
func GetParams(ctx context.Context) map[string]string {
	value := ctx.Value(contextVars)
	if params, ok := value.(map[string]string); ok {
		return params
	}
	return map[string]string{}
}
