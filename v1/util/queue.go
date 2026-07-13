// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

// LIFO represents a simple LIFO queue.
type LIFO struct {
	top  *queueNode
	size int
}

type queueNode struct {
	v    T
	next *queueNode
}

// NewLIFO returns a new LIFO queue containing elements ts starting with the
// left-most argument at the bottom.
func NewLIFO(ts ...T) *LIFO {
	s := &LIFO{}
	for i := range ts {
		s.Push(ts[i])
	}
	return s
}

// Push adds a new element onto the LIFO.
func (s *LIFO) Push(t T) {
	node := &queueNode{v: t, next: s.top}
	s.top = node
	s.size++
}

// Peek returns the top of the LIFO. If LIFO is empty, returns nil, false.
func (s *LIFO) Peek() (T, bool) {
	if s.top == nil {
		return nil, false
	}
	return s.top.v, true
}

// Pop returns the top of the LIFO and removes it. If LIFO is empty returns
// nil, false.
func (s *LIFO) Pop() (T, bool) {
	if s.top == nil {
		return nil, false
	}
	node := s.top
	s.top = node.next
	s.size--
	return node.v, true
}

// Size returns the size of the LIFO.
func (s *LIFO) Size() int {
	return s.size
}

// FIFO represents a simple FIFO queue.
type FIFO struct {
	front *queueNode
	back  *queueNode
	size  int
}

// NewFIFO returns a new FIFO queue containing elements ts starting with the
// left-most argument at the front.
func NewFIFO(ts ...T) *FIFO {
	s := &FIFO{}
	for i := range ts {
		s.Push(ts[i])
	}
	return s
}

// Push adds a new element onto the LIFO.
func (s *FIFO) Push(t T) {
	node := &queueNode{v: t, next: nil}
	if s.front == nil {
		s.front = node
		s.back = node
	} else {
		s.back.next = node
		s.back = node
	}
	s.size++
}

// Peek returns the top of the LIFO. If LIFO is empty, returns nil, false.
func (s *FIFO) Peek() (T, bool) {
	if s.front == nil {
		return nil, false
	}
	return s.front.v, true
}

// Pop returns the top of the LIFO and removes it. If LIFO is empty returns
// nil, false.
func (s *FIFO) Pop() (T, bool) {
	if s.front == nil {
		return nil, false
	}
	node := s.front
	s.front = node.next
	s.size--
	return node.v, true
}

// Size returns the size of the LIFO.
func (s *FIFO) Size() int {
	return s.size
}

// SliceStack is a generic LIFO stack backed by a slice.
type SliceStack[T any] struct {
	s []T
}

// Push adds v to the top of the stack.
func (s *SliceStack[T]) Push(v T) {
	s.s = append(s.s, v)
}

// Pop removes and returns the top element of the stack.
// It panics if the stack is empty.
func (s *SliceStack[T]) Pop() T {
	idx := len(s.s) - 1
	v := s.s[idx]
	var zero T
	s.s[idx] = zero // avoid retaining a reference to v in the backing array
	s.s = s.s[:idx]
	return v
}

// Peek returns the top element of the stack without removing it.
// It panics if the stack is empty.
func (s *SliceStack[T]) Peek() T {
	return s.s[len(s.s)-1]
}

// PeekPtr returns a pointer to the top element, so callers can mutate it in place.
// It panics if the stack is empty.
func (s *SliceStack[T]) PeekPtr() *T {
	return &s.s[len(s.s)-1]
}

// Slice returns the stack's underlying slice, bottom-to-top.
func (s *SliceStack[T]) Slice() []T {
	return s.s
}

// Len returns the number of elements on the stack.
func (s *SliceStack[T]) Len() int {
	return len(s.s)
}

// GroupStack is a two-level stack: a stack of groups, where each group is a
// slice of T. Whole groups are pushed and popped with PushGroup/PopGroup,
// while individual elements are pushed and popped onto the top group with
// Push/Pop. Element lookups (Peek) always target the top group.
//
// Both levels zero their vacated slots when popping, so a popped group or
// element isn't kept alive by the backing arrays.
type GroupStack[T any] struct {
	groups SliceStack[[]T]
}

// PushGroup pushes a new group onto the stack. Pass nil for an empty group.
func (g *GroupStack[T]) PushGroup(group []T) {
	g.groups.Push(group)
}

// PopGroup removes and returns the top group. It panics if there are no groups.
func (g *GroupStack[T]) PopGroup() []T {
	return g.groups.Pop()
}

// PeekGroup returns the top group without removing it. It panics if there are
// no groups.
func (g *GroupStack[T]) PeekGroup() []T {
	return g.groups.Peek()
}

// Push appends v to the top group. It panics if there are no groups.
func (g *GroupStack[T]) Push(v T) {
	top := g.groups.PeekPtr()
	*top = append(*top, v)
}

// Pop removes the top element of the top group. It panics if there are no
// groups or the top group is empty.
func (g *GroupStack[T]) Pop() {
	top := g.groups.PeekPtr()
	idx := len(*top) - 1
	var zero T
	(*top)[idx] = zero // avoid retaining a reference in the backing array
	*top = (*top)[:idx]
}

// Len returns the number of groups on the stack.
func (g *GroupStack[T]) Len() int {
	return g.groups.Len()
}
