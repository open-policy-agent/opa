// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package metrics

import (
	"container/heap"
	"math"
	"math/rand/v2"
	"slices"
	"sync"
	"time"
)

// rescaleThreshold is how often priorities are rescaled so that the
// exponential weights stay within float64 range.
const rescaleThreshold = time.Hour

// expDecaySample is a forward-decaying priority reservoir, see Cormode et
// al's "Forward Decay: A Practical Time Decay Model for Streaming Systems"
// (http://dimacs.rutgers.edu/~graham/pubs/papers/fwddecay.pdf). It keeps a
// fixed number of values, biased towards recent ones. The algorithm and its
// parameters follow github.com/rcrowley/go-metrics' ExpDecaySample, which
// OPA used before that project was archived.
type expDecaySample struct {
	mtx    sync.Mutex
	alpha  float64
	size   int
	count  int64
	t0, t1 time.Time
	values sampleHeap
}

type weightedValue struct {
	k float64 // priority
	v int64
}

// sampleHeap is a min-heap of weighted values ordered by priority.
type sampleHeap []weightedValue

func (h sampleHeap) Len() int           { return len(h) }
func (h sampleHeap) Less(i, j int) bool { return h[i].k < h[j].k }
func (h sampleHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *sampleHeap) Push(x any)        { *h = append(*h, x.(weightedValue)) }
func (h *sampleHeap) Pop() any {
	old := *h
	n := len(old) - 1
	x := old[n]
	*h = old[:n]
	return x
}

func newExpDecaySample(size int, alpha float64) *expDecaySample {
	now := time.Now()
	return &expDecaySample{
		alpha:  alpha,
		size:   size,
		t0:     now,
		t1:     now.Add(rescaleThreshold),
		values: make(sampleHeap, 0, size),
	}
}

func (s *expDecaySample) Update(v int64) {
	s.update(time.Now(), v)
}

func (s *expDecaySample) update(t time.Time, v int64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.count++
	if len(s.values) == s.size {
		heap.Pop(&s.values)
	}
	heap.Push(&s.values, weightedValue{
		k: math.Exp(t.Sub(s.t0).Seconds()*s.alpha) / rand.Float64(),
		v: v,
	})

	if t.After(s.t1) {
		scale := math.Exp(-s.alpha * t.Sub(s.t0).Seconds())
		for i := range s.values {
			s.values[i].k *= scale
		}
		// Scaling every priority by the same factor preserves heap order.
		s.t0 = t
		s.t1 = t.Add(rescaleThreshold)
	}
}

// snapshot returns the total number of updates and a copy of the values
// currently held in the reservoir.
func (s *expDecaySample) snapshot() (int64, []int64) {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	values := make([]int64, len(s.values))
	for i, wv := range s.values {
		values[i] = wv.v
	}
	return s.count, values
}

// percentiles returns the requested percentiles of values, interpolating
// between neighbouring values. The values are sorted in place.
func percentiles(values []int64, ps []float64) []float64 {
	scores := make([]float64, len(ps))
	size := len(values)
	if size == 0 {
		return scores
	}
	slices.Sort(values)
	for i, p := range ps {
		pos := p * float64(size+1)
		switch {
		case pos < 1.0:
			scores[i] = float64(values[0])
		case pos >= float64(size):
			scores[i] = float64(values[size-1])
		default:
			lower := float64(values[int(pos)-1])
			upper := float64(values[int(pos)])
			scores[i] = lower + (pos-math.Floor(pos))*(upper-lower)
		}
	}
	return scores
}

func mean(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum int64
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

func stdDev(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := mean(values)
	var sum float64
	for _, v := range values {
		d := float64(v) - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(values)))
}
