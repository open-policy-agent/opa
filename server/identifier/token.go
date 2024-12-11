package identifier

import (
	"net/http"

	v1 "github.com/open-policy-agent/opa/v1/server/identifier"
)

// TokenBased extracts Bearer tokens from the request.
type TokenBased = v1.TokenBased

// NewTokenBased returns a new TokenBased object.
func NewTokenBased(inner http.Handler) *TokenBased {
	return v1.NewTokenBased(inner)
}
