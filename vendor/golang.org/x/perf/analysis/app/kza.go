// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import "math"

// TODO: This all assumes that data is sampled at a regular interval
// and there are no missing values. It could be generalized to accept
// missing values (perhaps represented by NaN), or generalized much
// further by accepting (t, x) pairs and a vector of times at which to
// evaluate the filter (and an arbitrary window size). I would have to
// figure out how that affects the difference array in KZA.

// TODO: These can generate a lot of garbage. Perhaps the caller
// should pass in the target slice? Or these should just overwrite the
// input array and leave it to the caller to copy if necessary?

// MovingAverage performs a moving average (MA) filter of xs with
// window size m. m must be a positive odd integer.
//
// Note that this is filter is often described in terms of the half
// length of the window (m-1)/2.
func MovingAverage(xs []float64, m int) []float64 {
	if m <= 0 || m%2 != 1 {
		panic("m must be a positive, odd integer")
	}
	ys := make([]float64, len(xs))
	sum, n := 0.0, 0
	for l, i, r := -m, -(m-1)/2, 0; i < len(ys); l, i, r = l+1, i+1, r+1 {
		if l >= 0 {
			sum -= xs[l]
			n--
		}
		if r < len(xs) {
			sum += xs[r]
			n++
		}
		if i >= 0 {
			ys[i] = sum / float64(n)
		}
	}
	return ys
}

// KolmogorovZurbenko performs a Kolmogorov-Zurbenko (KZ) filter of xs
// with window size m and k iterations. m must be a positive odd
// integer. k must be positive.
func KolmogorovZurbenko(xs []float64, m, k int) []float64 {
	// k is typically small, and MA is quite efficient, so just do
	// the iterated moving average rather than bothering to
	// compute the binomial coefficient kernel.
	for i := 0; i < k; i++ {
		// TODO: Generate less garbage.
		xs = MovingAverage(xs, m)
	}
	return xs
}

// AdaptiveKolmogorovZurbenko performs an adaptive Kolmogorov-Zurbenko
// (KZA) filter of xs using an initial window size m and k iterations.
// m must be a positive odd integer. k must be positive.
//
// See Zurbenko, et al. 1996: Detecting discontinuities in time series
// of upper air data: Demonstration of an adaptive filter technique.
// Journal of Climate, 9, 3548–3560.
func AdaptiveKolmogorovZurbenko(xs []float64, m, k int) []float64 {
	// Perform initial KZ filter.
	z := KolmogorovZurbenko(xs, m, k)

	// Compute differenced values.
	q := (m - 1) / 2
	d := make([]float64, len(z)+1)
	maxD := 0.0
	for i := q; i < len(z)-q; i++ {
		d[i] = math.Abs(z[i+q] - z[i-q])
		if d[i] > maxD {
			maxD = d[i]
		}
	}

	if maxD == 0 {
		// xs is constant, so no amount of filtering will do
		// anything. Avoid dividing 0/0 below.
		return xs
	}

	// Compute adaptive filter.
	ys := make([]float64, len(xs))
	for t := range ys {
		dPrime := d[t+1] - d[t]
		f := 1 - d[t]/maxD

		qt := q
		if dPrime <= 0 {
			// Zurbenko doesn't specify what to do with
			// the fractional part of qt and qh, so we
			// interpret this as summing all points of xs
			// between qt and qh.
			qt = int(math.Ceil(float64(q) * f))
		}
		if t-qt < 0 {
			qt = t
		}

		qh := q
		if dPrime >= 0 {
			qh = int(math.Floor(float64(q) * f))
		}
		if t+qh >= len(xs) {
			qh = len(xs) - t - 1
		}

		sum := 0.0
		for i := t - qt; i <= t+qh; i++ {
			sum += xs[i]
		}
		// Zurbenko divides by qh+qt, but this undercounts the
		// number of terms in the sum by 1.
		ys[t] = sum / float64(qh+qt+1)
	}

	return ys
}
