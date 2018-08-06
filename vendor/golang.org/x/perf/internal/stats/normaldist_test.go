// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stats

import (
	"fmt"
	"math"
	"testing"
)

func TestNormalDist(t *testing.T) {
	d := StdNormal

	testFunc(t, fmt.Sprintf("%+v.PDF", d), d.PDF, map[float64]float64{
		-10000: 0, // approx
		-1:     1 / math.Sqrt(2*math.Pi) * math.Exp(-0.5),
		0:      1 / math.Sqrt(2*math.Pi),
		1:      1 / math.Sqrt(2*math.Pi) * math.Exp(-0.5),
		10000:  0, // approx
	})

	testFunc(t, fmt.Sprintf("%+v.CDF", d), d.CDF, map[float64]float64{
		-10000: 0, // approx
		0:      0.5,
		10000:  1, // approx
	})

	d2 := NormalDist{Mu: 2, Sigma: 5}
	testInvCDF(t, d, false)
	testInvCDF(t, d2, false)
}
