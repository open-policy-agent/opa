// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stats

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"
)

func testDiscreteCDF(t *testing.T, name string, dist DiscreteDist) {
	// Build the expected CDF out of the PMF.
	l, h := dist.Bounds()
	s := dist.Step()
	want := map[float64]float64{l - 0.1: 0, h: 1}
	sum := 0.0
	for x := l; x < h; x += s {
		sum += dist.PMF(x)
		want[x] = sum
		want[x+s/2] = sum
	}

	testFunc(t, name, dist.CDF, want)
}

func testInvCDF(t *testing.T, dist Dist, bounded bool) {
	inv := InvCDF(dist)
	name := fmt.Sprintf("InvCDF(%+v)", dist)
	cdfName := fmt.Sprintf("CDF(%+v)", dist)

	// Test bounds.
	vals := map[float64]float64{-0.01: nan, 1.01: nan}
	if !bounded {
		vals[0] = -inf
		vals[1] = inf
	}
	testFunc(t, name, inv, vals)

	if bounded {
		lo, hi := inv(0), inv(1)
		vals := map[float64]float64{
			lo - 0.01: 0, lo: 0,
			hi: 1, hi + 0.01: 1,
		}
		testFunc(t, cdfName, dist.CDF, vals)
		if got := dist.CDF(lo + 0.01); !(got > 0) {
			t.Errorf("%s(0)=%v, but %s(%v)=0", name, lo, cdfName, lo+0.01)
		}
		if got := dist.CDF(hi - 0.01); !(got < 1) {
			t.Errorf("%s(1)=%v, but %s(%v)=1", name, hi, cdfName, hi-0.01)
		}
	}

	// Test points between.
	vals = map[float64]float64{}
	for _, p := range vecLinspace(0, 1, 11) {
		if p == 0 || p == 1 {
			continue
		}
		x := inv(p)
		vals[x] = x
	}
	testFunc(t, fmt.Sprintf("InvCDF(CDF(%+v))", dist),
		func(x float64) float64 {
			return inv(dist.CDF(x))
		},
		vals)
}

// aeq returns true if expect and got are equal to 8 significant
// figures (1 part in 100 million).
func aeq(expect, got float64) bool {
	if expect < 0 && got < 0 {
		expect, got = -expect, -got
	}
	return expect*0.99999999 <= got && got*0.99999999 <= expect
}

func testFunc(t *testing.T, name string, f func(float64) float64, vals map[float64]float64) {
	xs := make([]float64, 0, len(vals))
	for x := range vals {
		xs = append(xs, x)
	}
	sort.Float64s(xs)

	for _, x := range xs {
		want, got := vals[x], f(x)
		if math.IsNaN(want) && math.IsNaN(got) || aeq(want, got) {
			continue
		}
		var label string
		if strings.Contains(name, "%v") {
			label = fmt.Sprintf(name, x)
		} else {
			label = fmt.Sprintf("%s(%v)", name, x)
		}
		t.Errorf("want %s=%v, got %v", label, want, got)
	}
}

// vecLinspace returns num values spaced evenly between lo and hi,
// inclusive. If num is 1, this returns an array consisting of lo.
func vecLinspace(lo, hi float64, num int) []float64 {
	res := make([]float64, num)
	if num == 1 {
		res[0] = lo
		return res
	}
	for i := 0; i < num; i++ {
		res[i] = lo + float64(i)*(hi-lo)/float64(num-1)
	}
	return res
}
