// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import (
	"fmt"
	"strings"
)

// Tracer defines the interface for tracing evaluation.
type Tracer interface {

	// Enabled returns true if the tracer is enabled.
	Enabled() bool

	// Trace emits a message if the tracer is enabled.
	Trace(ctx *TopDownContext, f string, a ...interface{})
}

// StdoutTracer writes trace messages to stdout.
type StdoutTracer struct{}

// Enabled always returns true.
func (t *StdoutTracer) Enabled() bool { return true }

// Trace writes a trace message to stdout.
func (t *StdoutTracer) Trace(ctx *TopDownContext, f string, a ...interface{}) {
	var padding string
	i := 0
	for ; ctx != nil; ctx = ctx.Previous {
		padding += strings.Repeat(" ", ctx.Index+i)
		i++
	}
	f = padding + f + "\n"
	fmt.Printf(f, a...)
}
