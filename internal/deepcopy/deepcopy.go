// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package deepcopy

// DeepCopy performs a recursive deep copy for nested slices/maps and
// returns the copied object. Supports []any
// and map[string]any only
func DeepCopy(val any) any {
	switch val := val.(type) {
	case []any:
		cpy := make([]any, len(val))
		for i := range cpy {
			cpy[i] = DeepCopy(val[i])
		}
		return cpy
	case map[string]any:
		return Map(val)
	default:
		return val
	}
}

func Map(val map[string]any) map[string]any {
	cpy := make(map[string]any, len(val))
	for k := range val {
		cpy[k] = DeepCopy(val[k])
	}
	return cpy
}
