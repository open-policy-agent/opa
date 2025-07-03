package transform

import (
	"errors"
	"fmt"

	"github.com/lestrrat-go/blackmagic"
)

// Mappable is an interface that defines methods required when converting
// a jwx structure into a map[string]any.
//
// EXPERIMENTAL: This API is experimental and its interface and behavior is
// subject to change in future releases. This API is not subject to semver
// compatibility guarantees.
type Mappable interface {
	Get(key string, dst any) error
	Keys() []string
}

// AsMap takes the specified Mappable object and populates the map
// `dst` with the key-value pairs from the Mappable object.
// Many objects in jwe, jwk, jws, and jwt packages including
// `jwt.Token`, `jwk.Key`, `jws.Header`, etc.
//
// EXPERIMENTAL: This API is experimental and its interface and behavior is
// subject to change in future releases. This API is not subject to semver
// compatibility guarantees.
func AsMap(m Mappable, dst map[string]any) error {
	if dst == nil {
		return fmt.Errorf("transform.AsMap: destination map cannot be nil")
	}

	for _, k := range m.Keys() {
		var val any
		if err := m.Get(k, &val); err != nil {
			// Allow invalid value errors. Assume they are just nil values.
			if !errors.Is(err, blackmagic.InvalidValueError()) {
				return fmt.Errorf(`transform.AsMap: failed to get key %q: %w`, k, err)
			}
		}
		dst[k] = val
	}

	return nil
}
