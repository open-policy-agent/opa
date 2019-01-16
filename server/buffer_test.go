// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestBufferAutoErase(t *testing.T) {
	buf := NewBoundedBuffer(30)
	for i := 0; i < 100; i++ {
		buf.Push(&Info{Path: fmt.Sprint(i)})
	}

	var i uint64 = 70
	buf.Iter(func(in *Info) {
		if int(i-70) > 30 {
			t.Fatal("Ran off the end of the buffer")
		}

		if in.Path != fmt.Sprint(i) {
			t.Fatalf("Expected query to be %d, got %s", i, in.Path)
		}
		i++
	})
}

func TestBufferRandom(t *testing.T) {
	rand.Seed(time.Now().Unix())
	for i := 0; i < 13; i++ {
		testBufferRandomSize(t, int((1 << uint(i))))
	}
}

var (
	threshold int = 1e3
	ops       int = 1e5
)

func testBufferRandomSize(t *testing.T, cap int) {
	var head, cur uint64
	var size int

	var pushes, iters uint

	buf := NewBoundedBuffer(cap)
	for i := 0; i < ops; i++ {
		switch r := rand.Intn(threshold + 1); {
		case r < threshold:
			pushes++
			buf.Push(&Info{Path: fmt.Sprint(cur)})

			cur++
			if size == cap {
				head++
			} else {
				size++
			}
		default:
			// Rarely, test that the iteration is working correctly.
			// Testing often leads to N^2 runtime for the tests, which is too
			// slow, so we only iter with very low probability.
			//
			// Occurs with probability 1/(threshold+1).
			iters++
			j := head
			buf.Iter(func(i *Info) {
				if int(j-head) > size {
					t.Fatal("Ran off the end of the buffer")
				}

				if i.Path != fmt.Sprint(j) {
					t.Fatalf("Expected query to be %d, got %s", j, i.Path)
				}
				j++
			})
		}
	}

	t.Logf("\nPushes: %d (%f%%)\nIters:  %d (%f%%)",
		pushes, 100*float64(pushes)/float64(ops),
		iters, 100*float64(iters)/float64(ops),
	)
}
