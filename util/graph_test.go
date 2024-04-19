// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"reflect"
	"testing"
)

type testTraversal struct {
	g       map[int][]int
	visited map[int]struct{}
	ordered []int
	stop    *int
}

func newTestTraversal(g map[int][]int) *testTraversal {
	return &testTraversal{
		g:       g,
		visited: map[int]struct{}{},
		ordered: nil,
		stop:    nil,
	}
}

func (t *testTraversal) Edges(x int) []int {
	var r []int
	r = append(r, t.g[x]...)
	return r
}

func (t *testTraversal) Equals(a, b int) bool {
	return a == b
}

func (t *testTraversal) Iter(x int) bool {
	t.ordered = append(t.ordered, x)
	return t.stop != nil && *t.stop == x
}

func (t *testTraversal) Visited(x int) bool {
	_, ok := t.visited[x]
	t.visited[x] = struct{}{}
	return ok
}

func TestDFSStop(t *testing.T) {
	g := map[int][]int{
		1: {2, 3},
		2: {4, 5},
		3: {6, 7},
		6: {2},
	}

	t1 := newTestTraversal(g)
	stop := 6
	t1.stop = &stop

	stopped := DFS[int](t1, t1.Iter, 1)

	if !stopped {
		t.Fatalf("Expected DFS to stop but got: %v", t1.ordered)
	}

	expected := []int{1, 3, 7, 6}

	if !reflect.DeepEqual(expected, t1.ordered) {
		t.Fatalf("Expected DFS ordering %v but got: %v", expected, t1.ordered)
	}
}

func TestBFSStop(t *testing.T) {
	g := map[int][]int{
		1: {2, 3},
		2: {4, 5},
		3: {6, 7},
		6: {2},
	}

	t1 := newTestTraversal(g)
	stop := 4
	t1.stop = &stop

	stopped := BFS[int](t1, t1.Iter, 1)

	if !stopped {
		t.Fatalf("Expected DFS to stop but got: %v", t1.ordered)
	}

	expected := []int{1, 2, 3, 4}

	if !reflect.DeepEqual(expected, t1.ordered) {
		t.Fatalf("Expected DFS ordering %v but got: %v", expected, t1.ordered)
	}
}

func TestDFS(t *testing.T) {
	g := map[int][]int{
		1: {2, 3},
		2: {4, 5},
		3: {6, 7},
		6: {2},
	}

	t1 := newTestTraversal(g)

	stopped := DFS[int](t1, t1.Iter, 1)
	if stopped {
		t.Fatalf("Did not expect traversal to stop")
	}

	expected := []int{1, 3, 7, 6, 2, 5, 4}

	if !reflect.DeepEqual(expected, t1.ordered) {
		t.Fatalf("Expected DFS ordering %v but got: %v", expected, t1.ordered)
	}
}

func TestBFS(t *testing.T) {
	g := map[int][]int{
		1: {2, 3},
		2: {4, 5},
		3: {6, 7},
		6: {2},
	}

	t1 := newTestTraversal(g)

	stopped := BFS[int](t1, t1.Iter, 1)
	if stopped {
		t.Fatalf("Did not expect traversal to stop")
	}

	expected := []int{1, 2, 3, 4, 5, 6, 7}

	if !reflect.DeepEqual(expected, t1.ordered) {
		t.Fatalf("Expected DFS ordering %v but got: %v", expected, t1.ordered)
	}

}

func TestDFSPath(t *testing.T) {

	g := map[int][]int{
		1: {2},
		2: {3, 4},
		3: {2},
		4: {1},
	}

	t1 := newTestTraversal(g)
	p1 := DFSPath[int](t1, t1.Equals, 1, 2)

	if !reflect.DeepEqual(p1, []int{1, 2}) {
		t.Errorf("Expected DFS(1,2) to equal {1,2} but got: %v", p1)
	}

	t2 := newTestTraversal(g)
	p2 := DFSPath[int](t2, t2.Equals, 1, 4)

	if !reflect.DeepEqual(p2, []int{1, 2, 4}) {
		t.Errorf("Expected DFS(1,4) to equal {1,2,4} but got: %v", p2)
	}

	t3 := newTestTraversal(g)
	p3 := DFSPath[int](t3, t3.Equals, 1, 0xadbeef)
	if len(p3) != 0 {
		t.Errorf("Expected DFS(1,0xadbeef to be empty but got: %v", p3)
	}

}
