// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	v1 "github.com/open-policy-agent/opa/v1/util"
)

// Compare returns 0 if a equals b, -1 if a is less than b, and 1 if b is than a.
//
// For comparison between values of different types, the following ordering is used:
// nil < bool < int, float64 < string < []interface{} < map[string]interface{}. Slices and maps
// are compared recursively. If one slice or map is a subset of the other slice or map
// it is considered "less than". Nil is always equal to nil.
func Compare(a, b interface{}) int {
	return v1.Compare(a, b)
}
