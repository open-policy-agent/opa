// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import "testing"

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
