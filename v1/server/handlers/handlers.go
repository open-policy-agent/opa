package handlers

import (
	"net/http"
	"strings"
)

// HeadMethodNotAllowedHandler returns a handler that responds with
// 405 Method Not Allowed for HEAD requests.
func HeadMethodNotAllowedHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

// TrailingSlashRedirectHandler returns a handler that redirects requests
// with a trailing slash to the same URL without the trailing slash.
func TrailingSlashRedirectHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && strings.HasSuffix(r.URL.Path, "/") {
			http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, "/"), http.StatusMovedPermanently)
			return
		}
		handler.ServeHTTP(w, r)
	})
}

// DefaultHandler returns a handler that applies both the HeadMethodNotAllowedHandler
// and TrailingSlashRedirectHandler to the provided handler.
func DefaultHandler(handler http.Handler) http.Handler {
	return HeadMethodNotAllowedHandler(TrailingSlashRedirectHandler(handler))
}
