// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
)

type stack interface {
	topdown.QueryTracer
	Current() (int, *topdown.Event)
	Event(i int) *topdown.Event
	Next() (int, *topdown.Event)
	Result() rego.ResultSet
	Close() error
}

type debugTracer struct {
	history   []*topdown.Event
	eventChan chan *topdown.Event
	waitChan  chan bool
	enabled   bool
	resultSet rego.ResultSet
}

func newDebugTracer() *debugTracer {
	return &debugTracer{
		eventChan: make(chan *topdown.Event),
		waitChan:  make(chan bool),
		enabled:   true,
	}
}

func (dt *debugTracer) Enabled() bool {
	return dt.enabled
}

func (dt *debugTracer) TraceEvent(e topdown.Event) {
	if !dt.enabled {
		return
	}

	defer func() {
		if recover() != nil {
			dt.enabled = false
		}
	}()

	dt.eventChan <- &e
	// Block until the consumer wants another event, so that evaluation isn't "ahead"
	<-dt.waitChan
}

func (dt *debugTracer) Config() topdown.TraceConfig {
	return topdown.TraceConfig{
		PlugLocalVars: true,
	}
}

func (dt *debugTracer) Current() (int, *topdown.Event) {
	stackLength := len(dt.history)
	if stackLength > 0 {
		return stackLength - 1, dt.history[len(dt.history)-1]
	}
	return -1, nil
}

func (dt *debugTracer) Event(i int) *topdown.Event {
	if i >= 0 && i < len(dt.history) {
		return dt.history[i]
	}
	return nil
}

func (dt *debugTracer) Next() (int, *topdown.Event) {
	if !dt.enabled {
		return -1, nil
	}

	if len(dt.history) > 0 {
		// We don't need to unblock the producer for the first event.
		dt.waitChan <- true
	}

	e, ok := <-dt.eventChan
	if ok {
		dt.history = append(dt.history, e)
		return len(dt.history) - 1, e
	}
	return len(dt.history) - 1, nil
}

func (dt *debugTracer) Result() rego.ResultSet {
	return dt.resultSet
}

func (dt *debugTracer) Close() error {
	if !dt.enabled {
		return nil
	}

	dt.enabled = false
	close(dt.eventChan)
	close(dt.waitChan)
	return nil
}
