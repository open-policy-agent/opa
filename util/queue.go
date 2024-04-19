// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

// LIFO represents a simple LIFO queue.
type LIFO[T any] struct {
	top  *queueNode[T]
	size int
}

type queueNode[T any] struct {
	v    T
	next *queueNode[T]
}

// NewLIFO returns a new LIFO queue containing elements ts starting with the
// left-most argument at the bottom.
func NewLIFO[T any](ts ...T) *LIFO[T] {
	s := new(LIFO[T])
	for i := range ts {
		s.Push(ts[i])
	}
	return s
}

// Push adds a new element onto the LIFO.
func (s *LIFO[T]) Push(t T) {
	node := &queueNode[T]{v: t, next: s.top}
	s.top = node
	s.size++
}

// Peek returns the top of the LIFO. If LIFO is empty, returns nil, false.
func (s *LIFO[T]) Peek() (T, bool) {
	if s.top == nil {
		return empty[T](), false
	}
	return s.top.v, true
}

// Pop returns the top of the LIFO and removes it. If LIFO is empty returns
// nil, false.
func (s *LIFO[T]) Pop() (T, bool) {
	if s.top == nil {
		return empty[T](), false
	}
	node := s.top
	s.top = node.next
	s.size--
	return node.v, true
}

// Size returns the size of the LIFO.
func (s *LIFO[T]) Size() int {
	return s.size
}

// FIFO represents a simple FIFO queue.
type FIFO[T any] struct {
	front *queueNode[T]
	back  *queueNode[T]
	size  int
}

// NewFIFO returns a new FIFO queue containing elements ts starting with the
// left-most argument at the front.
func NewFIFO[T any](ts ...T) *FIFO[T] {
	s := new(FIFO[T])
	for i := range ts {
		s.Push(ts[i])
	}
	return s
}

// Push adds a new element onto the LIFO.
func (s *FIFO[T]) Push(t T) {
	node := &queueNode[T]{v: t, next: nil}
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
func (s *FIFO[T]) Peek() (T, bool) {
	if s.front == nil {
		return empty[T](), false
	}
	return s.front.v, true
}

// Pop returns the top of the LIFO and removes it. If LIFO is empty returns
// nil, false.
func (s *FIFO[T]) Pop() (T, bool) {
	if s.front == nil {
		return empty[T](), false
	}
	node := s.front
	s.front = node.next
	s.size--
	return node.v, true
}

// Size returns the size of the LIFO.
func (s *FIFO[T]) Size() int {
	return s.size
}

func empty[T any]() T {
	var empty T
	return empty
}
