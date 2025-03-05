package util

import (
	"cmp"
	"slices"
)

// Keys returns a slice of keys from any map.
func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	return r
}

// KeysSorted returns a slice of keys from any map, sorted in ascending order.
func KeysSorted[M ~map[K]V, K cmp.Ordered, V any](m M) []K {
	r := make([]K, 0, len(m))
	for k := range m {
		r = append(r, k)
	}
	slices.Sort(r)
	return r
}

// Values returns a slice of values from any map. Copied from golang.org/x/exp/maps.
func Values[M ~map[K]V, K comparable, V any](m M) []V {
	r := make([]V, 0, len(m))
	for _, v := range m {
		r = append(r, v)
	}
	return r
}

// KeysRecursive returns a set of string keys from any map or slice,
// including nested maps and slices.
func KeysRecursive(x any, keys map[string]struct{}) map[string]struct{} {
	switch x := x.(type) {
	case map[string]any:
		for k := range x {
			keys[k] = struct{}{}
			v := x[k]
			keys = KeysRecursive(v, keys)
		}
	case []any:
		for i := range x {
			keys = KeysRecursive(x[i], keys)
		}
	case []map[string]any:
		for i := range x {
			keys = KeysRecursive(x[i], keys)
		}
	}
	return keys
}
