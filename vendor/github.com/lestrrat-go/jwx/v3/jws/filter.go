package jws

import (
	"github.com/lestrrat-go/jwx/v3/transform"
)

// HeaderFilter is an interface that allows users to filter JWS header fields.
// It provides two methods: Filter and Reject; Filter returns a new header with only
// the fields that match the filter criteria, while Reject returns a new header with
// only the fields that DO NOT match the filter.
//
// EXPERIMENTAL: This API is experimental and its interface and behavior is
// subject to change in future releases. This API is not subject to semver
// compatibility guarantees.
type HeaderFilter interface {
	Filter(header Headers) (Headers, error)
	Reject(header Headers) (Headers, error)
}

// StandardHeadersFilter returns a HeaderFilter that filters out standard JWS header fields.
//
// You can use this filter to create headers that either only have standard fields
// or only custom fields.
//
// If you need to configure the filter more precisely, consider
// using the HeaderNameFilter directly.
func StandardHeadersFilter() HeaderFilter {
	return stdHeadersFilter
}

var stdHeadersFilter = NewHeaderNameFilter(stdHeaderNames...)

// NewHeaderNameFilter creates a new HeaderNameFilter with the specified field names.
func NewHeaderNameFilter(names ...string) HeaderFilter {
	return transform.NewNameBasedFilter[Headers](names...)
}
