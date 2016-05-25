// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"reflect"
	"testing"
)

type testTraversal struct {
	g       map[int]map[int]struct{}
	visited map[int]struct{}
}

func (t *testTraversal) Edges(x T) []T {
	r := []T{}
	for v := range t.g[x.(int)] {
		r = append(r, v)
	}
	return r
}

func (t *testTraversal) Equals(a, b T) bool {
	return a.(int) == b.(int)
}

func (t *testTraversal) Visited(x T) bool {
	_, ok := t.visited[x.(int)]
	t.visited[x.(int)] = struct{}{}
	return ok
}

func TestGraphDFS(t *testing.T) {

	g := map[int]map[int]struct{}{
		1: map[int]struct{}{
			2: struct{}{},
		},
		2: map[int]struct{}{
			3: struct{}{},
			4: struct{}{},
		},
		3: map[int]struct{}{
			2: struct{}{},
		},
		4: map[int]struct{}{
			1: struct{}{},
		},
	}

	t1 := &testTraversal{g, map[int]struct{}{}}
	p1 := DFS(t1, 1, 2)

	if !reflect.DeepEqual(p1, []T{1, 2}) {
		t.Errorf("Expected DFS(1,2) to equal {1,2} but got: %v", p1)
	}

	t2 := &testTraversal{g, map[int]struct{}{}}
	p2 := DFS(t2, 1, 4)

	if !reflect.DeepEqual(p2, []T{1, 2, 4}) {
		t.Errorf("Expected DFS(1,4) to equal {1,2,4} but got: %v", p2)
	}

	t3 := &testTraversal{g, map[int]struct{}{}}
	p3 := DFS(t3, 1, 0xdeadbeef)
	if len(p3) != 0 {
		t.Errorf("Expected DFS(1,0xdeadbeef to be empty but got: %v", p3)
	}

}
