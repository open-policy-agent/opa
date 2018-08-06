// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"math/rand"
	"testing"
)

// Aeq returns true if expect and got are equal to 8 significant
// figures (1 part in 100 million).
func Aeq(expect, got float64) bool {
	if expect < 0 && got < 0 {
		expect, got = -expect, -got
	}
	return expect*0.99999999 <= got && got*0.99999999 <= expect
}

func TestMovingAverage(t *testing.T) {
	// Test MovingAverage against the obvious (but slow)
	// implementation.
	xs := make([]float64, 100)
	for iter := 0; iter < 10; iter++ {
		for i := range xs {
			xs[i] = rand.Float64()
		}
		m := 1 + 2*rand.Intn(100)
		ys1, ys2 := MovingAverage(xs, m), slowMovingAverage(xs, m)

		// TODO: Use stuff from mathtest.
		for i, y1 := range ys1 {
			if !Aeq(y1, ys2[i]) {
				t.Fatalf("want %v, got %v", ys2, ys1)
			}
		}
	}
}

func slowMovingAverage(xs []float64, m int) []float64 {
	ys := make([]float64, len(xs))
	for i := range ys {
		psum, n := 0.0, 0
		for j := i - (m-1)/2; j <= i+(m-1)/2; j++ {
			if 0 <= j && j < len(xs) {
				psum += xs[j]
				n++
			}
		}
		ys[i] = psum / float64(n)
	}
	return ys
}
