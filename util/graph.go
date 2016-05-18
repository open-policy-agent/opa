// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

// DFSTraversal defines the basic interface required to perform a depth
// first traveral.
type DFSTraversal interface {
	Edges(T) []T
	Equals(T, T) bool
	Visited(T) bool
}

// DFS returns a path from node a to node z found by performing
// a depth first traversal. If no path is found, an empty slice is returned.
func DFS(t DFSTraversal, a, z T) []T {
	p := dfsRecursive(t, a, z, []T{})
	for i := len(p)/2 - 1; i >= 0; i-- {
		o := len(p) - i - 1
		p[i], p[o] = p[o], p[i]
	}
	return p
}

func dfsRecursive(t DFSTraversal, u, z T, path []T) []T {
	if t.Visited(u) {
		return path
	}
	for _, v := range t.Edges(u) {
		if t.Equals(v, z) {
			path = append(path, z)
			path = append(path, u)
			return path
		}
		if p := dfsRecursive(t, v, z, path); len(p) > 0 {
			path = append(p, u)
			return path
		}
	}
	return path
}
