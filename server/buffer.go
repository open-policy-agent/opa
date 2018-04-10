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

// Buffer defines an interface that the server can call to push diagnostic
// information about policy decisions. Buffers must be able to handle
// concurrent calls.
type Buffer interface {
	// Push adds the given Info into the buffer.
	Push(*Info)

	// Iter iterates over the buffer, from oldest present Info to newest. It should
	// call fn on each Info.
	Iter(fn func(*Info))
}

// buffer stores diagnostic information.
type buffer struct {
	ring  []*Info
	size  int
	start int // The index of the next item to pop.
	end   int // The index where the next item will be inserted.
	sync.Mutex
}

// NewBoundedBuffer creates a new Buffer with maximum size n. NewBoundedBuffer
// will panic if n is not positive.
func NewBoundedBuffer(n int) Buffer {
	if n <= 0 {
		panic("size must be greater than 0")
	}

	return &buffer{ring: make([]*Info, n, n)}
}

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

func (b *buffer) Iter(fn func(*Info)) {
	b.Lock()
	defer b.Unlock()

	i := b.start
	for j := 0; j < b.size; j++ {
		fn(b.ring[i])
		i = (i + 1) % len(b.ring)
	}
}

// Info contains information describing a policy decision.
type Info struct {
	Revision   string
	DecisionID string
	RemoteAddr string
	Query      string
	Timestamp  time.Time
	Input      interface{}
	Results    *interface{}
	Error      error
	Metrics    metrics.Metrics
	Trace      []*topdown.Event
}

type diagSettings struct {
	on      bool
	explain bool
}

var (
	diagsOff = diagSettings{}
	diagsOn  = diagSettings{on: true}
	diagsAll = diagSettings{on: true, explain: true}
)
