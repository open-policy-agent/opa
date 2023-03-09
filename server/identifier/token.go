package identifier

import (
	"net/http"
	"regexp"
)

// TokenBased extracts Bearer tokens from the request.
type TokenBased struct {
	inner http.Handler
}

// NewTokenBased returns a new TokenBased object.
func NewTokenBased(inner http.Handler) *TokenBased {
	return &TokenBased{
		inner: inner,
	}
}

var bearerTokenRegexp = regexp.MustCompile(`^Bearer\s+(\S+)$`)

func (h *TokenBased) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	value := r.Header.Get("Authorization")
	if len(value) > 0 {
		match := bearerTokenRegexp.FindStringSubmatch(value)
		if len(match) > 0 {
			r = SetIdentity(r, match[1])
		}
	}

	h.inner.ServeHTTP(w, r)
}
