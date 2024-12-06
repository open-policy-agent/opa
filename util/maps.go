package util

import v1 "github.com/open-policy-agent/opa/v1/util"

// Values returns a slice of values from any map. Copied from golang.org/x/exp/maps.
func Values[M ~map[K]V, K comparable, V any](m M) []V {
	return v1.Values(m)
}
