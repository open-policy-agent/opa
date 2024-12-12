// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	v1 "github.com/open-policy-agent/opa/v1/util"
)

// T is a concise way to refer to T.
type T = v1.T

// HashMap represents a key/value map.
type HashMap = v1.HashMap

// NewHashMap returns a new empty HashMap.
func NewHashMap(eq func(T, T) bool, hash func(T) int) *HashMap {
	return v1.NewHashMap(eq, hash)
}
