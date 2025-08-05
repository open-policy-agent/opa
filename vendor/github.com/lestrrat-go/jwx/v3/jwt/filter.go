package jwt

import (
	"github.com/lestrrat-go/jwx/v3/transform"
)

// TokenFilter is an interface that allows users to filter JWT claims.
// It provides two methods: Filter and Reject; Filter returns a new token with only
// the claims that match the filter criteria, while Reject returns a new token with
// only the claims that DO NOT match the filter.
//
// EXPERIMENTAL: This API is experimental and its interface and behavior is
// subject to change in future releases. This API is not subject to semver
// compatibility guarantees.
type TokenFilter interface {
	Filter(token Token) (Token, error)
	Reject(token Token) (Token, error)
}

// StandardClaimsFilter returns a TokenFilter that filters out standard JWT claims.
//
// You can use this filter to create tokens that either only has standard claims
// or only custom claims. If you need to configure the filter more precisely, consider
// using the ClaimNameFilter directly.
func StandardClaimsFilter() TokenFilter {
	return stdClaimsFilter
}

var stdClaimsFilter = NewClaimNameFilter(stdClaimNames...)

// NewClaimNameFilter creates a new ClaimNameFilter with the specified claim names.
func NewClaimNameFilter(names ...string) TokenFilter {
	return transform.NewNameBasedFilter[Token](names...)
}
