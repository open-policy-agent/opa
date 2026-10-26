// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package merge contains helpers to merge data structures
// frequently encountered in OPA.
package merge

import "slices"

// InterfaceMaps returns the result of merging a and b. If a and b cannot be
// merged because of conflicting key-value pairs, ok is false.
func InterfaceMaps(a map[string]any, b map[string]any) (map[string]any, bool) {

	if a == nil {
		return b, true
	}

	if hasConflicts(a, b) {
		return nil, false
	}

	return merge(a, b), true
}

// Conflict returns the path to the first (in key order) conflicting key-value
// pair found when merging b into a, if any.
func Conflict(a, b map[string]any) ([]string, bool) {
	keys := make([]string, 0, len(b))
	for k := range b {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		exist, ok := a[k]
		if !ok {
			continue
		}

		existObj, existOk := exist.(map[string]any)
		addObj, addOk := b[k].(map[string]any)
		if !existOk || !addOk {
			return []string{k}, true
		}

		if path, ok := Conflict(existObj, addObj); ok {
			return append([]string{k}, path...), true
		}
	}
	return nil, false
}

func merge(a, b map[string]any) map[string]any {

	for k := range b {

		add := b[k]
		exist, ok := a[k]
		if !ok {
			a[k] = add
			continue
		}

		existObj := exist.(map[string]any)
		addObj := add.(map[string]any)

		a[k] = merge(existObj, addObj)
	}

	return a
}

func hasConflicts(a, b map[string]any) bool {
	for k := range b {

		add := b[k]
		exist, ok := a[k]
		if !ok {
			continue
		}

		existObj, existOk := exist.(map[string]any)
		addObj, addOk := add.(map[string]any)
		if !existOk || !addOk {
			return true
		}

		if hasConflicts(existObj, addObj) {
			return true
		}
	}
	return false
}
