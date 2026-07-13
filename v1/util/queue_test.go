// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"slices"
	"testing"
)

func TestLIFO(t *testing.T) {

	lifo := NewLIFO(1, 2, 3, 4)

	if lifo.Size() != 4 {
		t.Fatalf("Expected LIFO size == 4 but got: %v", lifo.Size())
	}

	for i := 4; i >= 1; i-- {
		x, ok := lifo.Peek()
		if !ok || x != i {
			t.Fatalf("Expected peek() == %v but got: %v (ok=%v)", i, x, ok)
		}
		x, ok = lifo.Pop()
		if !ok || x != i {
			t.Fatalf("Expected pop() == %v but got: %v (ok=%v)", i, x, ok)
		}
	}

	x, ok := lifo.Peek()
	if ok || x != nil {
		t.Fatalf("Expected peek() == nil, false but got: %v (ok=%v)", x, ok)
	}

	x, ok = lifo.Pop()
	if ok || x != nil {
		t.Fatalf("Expected pop() == nil, false but got: %v (ok=%v)", x, ok)
	}

	for i := 4; i >= 1; i-- {
		lifo.Push(i)
		x, ok = lifo.Peek()
		if !ok || x != i {
			t.Fatalf("Expected peek() == %v but got: %v (ok=%v)", i, x, ok)
		}
	}

}

func TestFIFO(t *testing.T) {
	fifo := NewFIFO(1, 2, 3, 4)

	if fifo.Size() != 4 {
		t.Fatalf("Expected FIFO size == 1 but got: %v", fifo.Size())
	}

	for i := 1; i <= 4; i++ {
		x, ok := fifo.Peek()
		if !ok || x != i {
			t.Fatalf("Expected peek() == %v but got: %v (ok=%v)", i, x, ok)
		}
		x, ok = fifo.Pop()
		if !ok || x != i {
			t.Fatalf("Expected pop() == %v but got: %v (ok=%v)", i, x, ok)
		}
	}

	x, ok := fifo.Peek()
	if ok || x != nil {
		t.Fatalf("Expected peek() == nil, false but got: %v (ok=%v)", x, ok)
	}

	x, ok = fifo.Pop()
	if ok || x != nil {
		t.Fatalf("Expected pop() == nil, false but got: %v (ok=%v)", x, ok)
	}

	for i := 1; i <= 4; i++ {
		fifo.Push(i)
		x, ok = fifo.Peek()
		if !ok || x != 1 {
			t.Fatalf("Expected peek() == %v but got: %v (ok=%v)", 1, x, ok)
		}
	}

}

func TestSliceStack(t *testing.T) {
	var s SliceStack[int]

	s.Push(1)
	s.Push(2)
	s.Push(3)

	if s.Len() != 3 {
		t.Fatalf("expected len 3, got %d", s.Len())
	}
	if !slices.Equal(s.Slice(), []int{1, 2, 3}) {
		t.Fatalf("expected [1 2 3], got %v", s.Slice())
	}
	if v := s.Peek(); v != 3 {
		t.Fatalf("expected Peek() == 3, got %d", v)
	}
	if p := s.PeekPtr(); *p != 3 {
		t.Fatalf("expected *PeekPtr() == 3, got %d", *p)
	} else {
		*p = 30
	}
	if v := s.Pop(); v != 30 {
		t.Fatalf("expected Pop() == 30, got %d", v)
	}
	if s.Len() != 2 {
		t.Fatalf("expected len 2 after Pop, got %d", s.Len())
	}
	if v := s.Pop(); v != 2 {
		t.Fatalf("expected Pop() == 2, got %d", v)
	}
	if v := s.Pop(); v != 1 {
		t.Fatalf("expected Pop() == 1, got %d", v)
	}
	if s.Len() != 0 {
		t.Fatalf("expected len 0, got %d", s.Len())
	}
}

func TestGroupStack(t *testing.T) {
	var g GroupStack[int]

	g.PushGroup(nil)
	g.Push(1)
	g.Push(2)

	if g.Len() != 1 {
		t.Fatalf("expected 1 group, got %d", g.Len())
	}
	if got := g.PeekGroup(); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("expected top group [1 2], got %v", got)
	}

	// A second group shadows the first for element operations.
	g.PushGroup([]int{10})
	g.Push(20)
	if got := g.PeekGroup(); !slices.Equal(got, []int{10, 20}) {
		t.Fatalf("expected top group [10 20], got %v", got)
	}

	g.Pop() // removes 20 from the top group
	if got := g.PeekGroup(); !slices.Equal(got, []int{10}) {
		t.Fatalf("expected top group [10] after Pop, got %v", got)
	}

	if got := g.PopGroup(); !slices.Equal(got, []int{10}) {
		t.Fatalf("expected PopGroup() == [10], got %v", got)
	}

	// The first group is intact after popping the second.
	if got := g.PeekGroup(); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("expected top group [1 2] restored, got %v", got)
	}
	if g.Len() != 1 {
		t.Fatalf("expected 1 group after PopGroup, got %d", g.Len())
	}
}

// TestGroupStackClearsVacatedSlots guards against popped values being retained
// by the backing arrays at either level of the stack.
func TestGroupStackClearsVacatedSlots(t *testing.T) {
	var g GroupStack[*int]

	a, b := 1, 2
	g.PushGroup(nil)
	g.Push(&a)
	g.Push(&b)

	g.Pop() // removes &b from the top group
	// Reach past the shrunk length into the backing array's vacated slot.
	top := g.groups.PeekPtr()
	if slot := (*top)[:cap(*top)][len(*top)]; slot != nil {
		t.Fatalf("expected vacated element slot to be nil, got %v", slot)
	}

	g.Push(&b)
	popped := g.PopGroup() // removes the whole top group
	// The outer backing array must not retain the popped group.
	outer := g.groups.Slice()
	if slot := outer[:cap(outer)][len(outer)]; slot != nil {
		t.Fatalf("expected vacated group slot to be nil, got %v", slot)
	}
	_ = popped
}
