package jwk

import (
	"github.com/lestrrat-go/jwx/v3/transform"
)

// KeyFilter is an interface that allows users to filter JWK key fields.
// It provides two methods: Filter and Reject; Filter returns a new key with only
// the fields that match the filter criteria, while Reject returns a new key with
// only the fields that DO NOT match the filter.
//
// EXPERIMENTAL: This API is experimental and its interface and behavior is
// subject to change in future releases. This API is not subject to semver
// compatibility guarantees.
type KeyFilter interface {
	Filter(key Key) (Key, error)
	Reject(key Key) (Key, error)
}

// NewFieldNameFilter creates a new FieldNameFilter with the specified field names.
//
// Note that because some JWK fields are associated with the type instead of
// stored as data, this filter will not be able to remove them. An example would
// be the `kty` field: it's associated with the underlying JWK key type, and will
// always be present even if you attempt to remove it.
func NewFieldNameFilter(names ...string) KeyFilter {
	return transform.NewNameBasedFilter[Key](names...)
}
