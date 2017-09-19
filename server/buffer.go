// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"sync"
	"time"

	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/topdown"
)

// Buffer defines the interface for types that can be used as the diagnostic buffer
// within the OPA server. Buffers must be be able to handle concurrent calls.
type Buffer interface {
	// Push adds the given Info into the buffer.
	Push(*Info)

	// Iter iterates over the buffer, from oldest present Info to newest. It should
	// call fn on each Info.
	Iter(fn func(*Info))
}

// buffer stores diagnostic information.
type buffer struct {
	ring []*Info

	size int

	start int // The index of the next item to pop.
	end   int // The index where the next item will be inserted.

	sync.Mutex
}

// NewBoundedBuffer creates a new Buffer with maximum size n. NewBoundedBuffer will panic if n is not
// positive.
func NewBoundedBuffer(n int) Buffer {
	if n <= 0 {
		panic("size must be greater than 0")
	}

	return &buffer{ring: make([]*Info, n, n)}
}

// Push inserts the provided Info into the buffer. If the buffer is full, the oldest item
// is deleted to make room.
func (b *buffer) Push(i *Info) {
	b.Lock()
	defer b.Unlock()

	b.ring[b.end] = i

	b.end = (b.end + 1) % len(b.ring)

	switch b.size {
	case len(b.ring): // We erased something, move the start up.
		b.start = (b.start + 1) % len(b.ring)
	default:
		b.size++
	}
}

// Iter iterates over the buffer, calling fn for every element in it.
func (b *buffer) Iter(fn func(*Info)) {
	b.Lock()
	defer b.Unlock()

	i := b.start
	for j := 0; j < b.size; j++ {
		fn(b.ring[i])
		i = (i + 1) % len(b.ring)
	}
}

// Info stores diagnostic information about the evaluation of a query.
type Info struct {
	DecisionID string
	Query      string
	Timestamp  time.Time
	Input      interface{}
	Results    *interface{}
	Error      error
	Metrics    metrics.Metrics
	Trace      []*topdown.Event
}

// newInfo creates an returns a new Info.
func newInfo(decisionID, query string, input interface{}, results *interface{}) *Info {
	return &Info{
		DecisionID: decisionID,
		Query:      query,
		Timestamp:  time.Now(),
		Input:      input,
		Results:    results,
	}
}

// withMetrics sets the metrics for the Info struct, returning it for convenience.
func (i *Info) withMetrics(m metrics.Metrics) *Info {
	i.Metrics = m
	return i
}

// withTracer sets the trace buffer for the Info struct, returning it for convenience.
func (i *Info) withTrace(t []*topdown.Event) *Info {
	i.Trace = t
	return i
}

func (i *Info) withError(err error) *Info {
	i.Error = err
	return i
}

// settings represents how an Info instance should be configured.
type settings struct {
	on      bool
	metrics bool
	explain bool
}

var (
	// diagsOff represnts that diagnostics are turned off.
	diagsOff = settings{}

	// diagsFull represents that diagnostics are all the way on.
	diagsFull = settings{on: true, metrics: true, explain: true}
)
