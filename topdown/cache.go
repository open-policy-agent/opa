// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	v1 "github.com/open-policy-agent/opa/v1/topdown"
)

// VirtualCache defines the interface for a cache that stores the results of
// evaluated virtual documents (rules).
// The cache is a stack of frames, where each frame is a mapping from references
// to values.
type VirtualCache = v1.VirtualCache

func NewVirtualCache() VirtualCache {
	return v1.NewVirtualCache()
}
