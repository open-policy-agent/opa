// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import v1 "github.com/open-policy-agent/opa/v1/util"

// Traversal defines a basic interface to perform traversals.
type Traversal = v1.Traversal

// Equals should return true if node "u" equals node "v".
type Equals = v1.Equals

// Iter should return true to indicate stop.
type Iter = v1.Iter

// DFS performs a depth first traversal calling f for each node starting from u.
// If f returns true, traversal stops and DFS returns true.
func DFS(t Traversal, f Iter, u T) bool {
	return v1.DFS(t, f, u)
}

// BFS performs a breadth first traversal calling f for each node starting from
// u. If f returns true, traversal stops and BFS returns true.
func BFS(t Traversal, f Iter, u T) bool {
	return v1.BFS(t, f, u)
}

// DFSPath returns a path from node a to node z found by performing
// a depth first traversal. If no path is found, an empty slice is returned.
func DFSPath(t Traversal, eq Equals, a, z T) []T {
	return v1.DFSPath(t, eq, a, z)
}
