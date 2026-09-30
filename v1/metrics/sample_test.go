// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package metrics

import (
	"math"
	"slices"
	"testing"
	"time"
)

func TestHistogramValue(t *testing.T) {
	h := newHistogram()
	for i := int64(1); i <= 10; i++ {
		h.Update(i)
	}

	got := h.Value().(map[string]any)
	exp := map[string]any{
		"count":  int64(10),
		"min":    int64(1),
		"max":    int64(10),
		"mean":   5.5,
		"stddev": math.Sqrt(8.25),
		"median": 5.5,
		"75%":    8.25,
		"90%":    9.9,
		"95%":    10.0,
		"99%":    10.0,
		"99.9%":  10.0,
		"99.99%": 10.0,
	}
	if len(got) != len(exp) {
		t.Fatalf("expected %d keys, got %d: %v", len(exp), len(got), got)
	}
	for k, e := range exp {
		switch e := e.(type) {
		case int64:
			if got[k] != e {
				t.Errorf("%s: expected %v, got %v (%T)", k, e, got[k], got[k])
			}
		case float64:
			g, ok := got[k].(float64)
			if !ok || math.Abs(g-e) > 1e-9 {
				t.Errorf("%s: expected %v, got %v (%T)", k, e, got[k], got[k])
			}
		}
	}
}

func TestHistogramValueEmpty(t *testing.T) {
	got := newHistogram().Value().(map[string]any)
	for _, k := range []string{"count", "min", "max"} {
		if got[k] != int64(0) {
			t.Errorf("%s: expected int64(0), got %v (%T)", k, got[k], got[k])
		}
	}
	for _, k := range []string{"mean", "stddev", "median", "75%", "90%", "95%", "99%", "99.9%", "99.99%"} {
		if got[k] != 0.0 {
			t.Errorf("%s: expected 0.0, got %v (%T)", k, got[k], got[k])
		}
	}
}

func TestExpDecaySampleReservoirSize(t *testing.T) {
	s := newExpDecaySample(100, 0.99)
	for i := range int64(1000) {
		s.Update(i)
	}

	count, values := s.snapshot()
	if count != 1000 {
		t.Errorf("expected count 1000, got %d", count)
	}
	if len(values) != 100 {
		t.Fatalf("expected 100 values, got %d", len(values))
	}
	for _, v := range values {
		if v < 0 || v >= 1000 {
			t.Errorf("unexpected value %d", v)
		}
	}
}

func TestExpDecaySampleRescale(t *testing.T) {
	s := newExpDecaySample(2, 0.001)
	start := s.t0
	s.update(start, 1)
	s.update(start.Add(time.Minute), 2)

	// Remember the relative order of the priorities before rescaling.
	before := slices.Clone(s.values)
	slices.SortFunc(before, func(a, b weightedValue) int { return int(a.v - b.v) })

	s.update(start.Add(rescaleThreshold+time.Second), 3)

	if exp := start.Add(rescaleThreshold + time.Second); !s.t0.Equal(exp) {
		t.Errorf("expected t0 %v, got %v", exp, s.t0)
	}
	if exp := s.t0.Add(rescaleThreshold); !s.t1.Equal(exp) {
		t.Errorf("expected t1 %v, got %v", exp, s.t1)
	}
	for i := 1; i < len(s.values); i++ {
		if s.values[(i-1)/2].k > s.values[i].k {
			t.Fatalf("heap order broken after rescale: %v", s.values)
		}
	}
	for _, wv := range s.values {
		if math.IsInf(wv.k, 0) || math.IsNaN(wv.k) || wv.k <= 0 {
			t.Errorf("unexpected priority after rescale: %v", wv)
		}
		for _, b := range before {
			if b.v == wv.v {
				scale := math.Exp(-0.001 * (rescaleThreshold + time.Second).Seconds())
				if math.Abs(wv.k-b.k*scale) > 1e-9*b.k {
					t.Errorf("value %d: expected priority %v, got %v", wv.v, b.k*scale, wv.k)
				}
			}
		}
	}
}

func TestPercentiles(t *testing.T) {
	tests := []struct {
		note   string
		values []int64
		ps     []float64
		exp    []float64
	}{
		{
			note: "empty",
			ps:   []float64{0.5, 0.99},
			exp:  []float64{0, 0},
		},
		{
			note:   "single value",
			values: []int64{7},
			ps:     []float64{0.01, 0.5, 0.99},
			exp:    []float64{7, 7, 7},
		},
		{
			note:   "unsorted input is interpolated",
			values: []int64{4, 1, 3, 2},
			ps:     []float64{0.1, 0.5, 0.75, 0.9},
			exp:    []float64{1, 2.5, 3.75, 4},
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			if got := percentiles(tc.values, tc.ps); !slices.Equal(got, tc.exp) {
				t.Errorf("expected %v, got %v", tc.exp, got)
			}
		})
	}
}
