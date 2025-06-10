package jwk

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var strictKeyUsage = atomic.Bool{}
var keyUsageNames = map[string]struct{}{}
var muKeyUsageName sync.RWMutex

// RegisterKeyUsage registers a possible value that can be used for KeyUsageType.
// Normally, key usage (or the "use" field in a JWK) is either "sig" or "enc",
// but other values may be used.
//
// While this module only works with "sig" and "enc", it is possible that
// systems choose to use other values. This function allows users to register
// new values to be accepted as valid key usage types. Values are case sensitive.
//
// Furthermore, the check against registered values can be completely turned off
// by setting the global option `jwk.WithStrictKeyUsage(false)`.
func RegisterKeyUsage(v string) {
	muKeyUsageName.Lock()
	defer muKeyUsageName.Unlock()
	keyUsageNames[v] = struct{}{}
}

func UnregisterKeyUsage(v string) {
	muKeyUsageName.Lock()
	defer muKeyUsageName.Unlock()
	delete(keyUsageNames, v)
}

func init() {
	strictKeyUsage.Store(true)
	RegisterKeyUsage("sig")
	RegisterKeyUsage("enc")
}

func isValidUsage(v string) bool {
	// This function can return true if strictKeyUsage is false
	if !strictKeyUsage.Load() {
		return true
	}

	muKeyUsageName.RLock()
	defer muKeyUsageName.RUnlock()
	_, ok := keyUsageNames[v]
	return ok
}

func (k KeyUsageType) String() string {
	return string(k)
}

func (k *KeyUsageType) Accept(v interface{}) error {
	switch v := v.(type) {
	case KeyUsageType:
		if !isValidUsage(v.String()) {
			return fmt.Errorf("invalid key usage type: %q", v)
		}
		*k = v
		return nil
	case string:
		if !isValidUsage(v) {
			return fmt.Errorf("invalid key usage type: %q", v)
		}
		*k = KeyUsageType(v)
		return nil
	}

	return fmt.Errorf("invalid Go type for key usage type: %T", v)
}
