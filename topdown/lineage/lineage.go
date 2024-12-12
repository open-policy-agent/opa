// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package lineage

import (
	"github.com/open-policy-agent/opa/topdown"
	v1 "github.com/open-policy-agent/opa/v1/topdown/lineage"
)

// Debug contains everything in the log.
func Debug(trace []*topdown.Event) []*topdown.Event {
	return v1.Debug(trace)
}

// Full returns a filtered trace that contains everything except Unify ops
func Full(trace []*topdown.Event) (result []*topdown.Event) {
	return v1.Full(trace)
}

// Notes returns a filtered trace that contains Note events and context to
// understand where the Note was emitted.
func Notes(trace []*topdown.Event) []*topdown.Event {
	return v1.Notes(trace)
}

// Fails returns a filtered trace that contains Fail events and context to
// understand where the Fail occurred.
func Fails(trace []*topdown.Event) []*topdown.Event {
	return v1.Fails(trace)
}

// Filter will filter a given trace using the specified filter function. The
// filtering function should return true for events that should be kept, false
// for events that should be filtered out.
func Filter(trace []*topdown.Event, filter func(*topdown.Event) bool) (result []*topdown.Event) {
	return v1.Filter(trace, filter)
}
