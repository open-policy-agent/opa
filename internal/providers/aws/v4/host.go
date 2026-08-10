package v4

import (
	"net/http"
	"strings"
)

// SanitizeHostForHeader removes default port from host and updates request.Host
func SanitizeHostForHeader(r *http.Request) {
	host := getHost(r)
	port := portOnly(host)
	if port != "" && isDefaultPort(r.URL.Scheme, port) {
		r.Host = stripPort(host)
	}
}

// Returns host from request
func getHost(r *http.Request) string {
	if r.Host != "" {
		return r.Host
	}

	return r.URL.Host
}

// Hostname returns u.Host, without any port number.
//
// If Host is an IPv6 literal with a port number, Hostname returns the
// IPv6 literal without the square brackets. IPv6 literals may include
// a zone identifier.
func stripPort(hostport string) string {
	before, _, ok := strings.Cut(hostport, ":")
	if !ok {
		return hostport
	}
	if before, _, ok := strings.Cut(hostport, "]"); ok {
		return strings.TrimPrefix(before, "[")
	}
	return before
}

// Port returns the port part of u.Host, without the leading colon.
// If u.Host doesn't contain a port, Port returns an empty string.
func portOnly(hostport string) string {
	_, after, ok := strings.Cut(hostport, ":")
	if !ok {
		return ""
	}
	if _, after, ok := strings.Cut(hostport, "]:"); ok {
		return after
	}
	if strings.Contains(hostport, "]") {
		return ""
	}
	return after
}

// Returns true if the specified URI is using the standard port
// (i.e. port 80 for HTTP URIs or 443 for HTTPS URIs)
func isDefaultPort(scheme, port string) bool {
	return port == "" ||
		(strings.EqualFold(scheme, "http") && port == "80") ||
		(strings.EqualFold(scheme, "https") && port == "443")
}
