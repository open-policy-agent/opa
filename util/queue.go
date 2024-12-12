// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import v1 "github.com/open-policy-agent/opa/v1/util"

// LIFO represents a simple LIFO queue.
type LIFO = v1.LIFO

// NewLIFO returns a new LIFO queue containing elements ts starting with the
// left-most argument at the bottom.
func NewLIFO(ts ...T) *LIFO {
	return v1.NewLIFO(ts...)
}

// FIFO represents a simple FIFO queue.
type FIFO = v1.FIFO

// NewFIFO returns a new FIFO queue containing elements ts starting with the
// left-most argument at the front.
func NewFIFO(ts ...T) *FIFO {
	return v1.NewFIFO(ts...)
}
