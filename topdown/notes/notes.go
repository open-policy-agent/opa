// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package notes

import (
	"github.com/open-policy-agent/opa/topdown"
)

// Filter returns a filtered trace that contains Note events contained in the
// trace. The filtered trace also includes Enter and Redo events to explain the
// lineage of the Note events.
func Filter(trace []*topdown.Event) (result []*topdown.Event) {

	qids := map[uint64]*topdown.Event{}

	for _, event := range trace {
		switch event.Op {
		case topdown.NoteOp:
			// Path will end with the Note event.
			path := []*topdown.Event{event}

			// Construct path of recorded Enter/Redo events that lead to the
			// Note event. The path is constructed in reverse order by iterating
			// backwards through the Enter/Redo events from the Note event.
			curr := qids[event.QueryID]
			var prev *topdown.Event

			for curr != nil && curr != prev {
				path = append(path, curr)
				prev = curr
				curr = qids[curr.ParentID]
			}

			// Add the path to the result, reversing it in the process.
			for i := len(path) - 1; i >= 0; i-- {
				result = append(result, path[i])
			}

			qids = map[uint64]*topdown.Event{}

		case topdown.EnterOp, topdown.RedoOp:
			if event.HasRule() || event.HasBody() {
				qids[event.QueryID] = event
			}
		}
	}

	return result
}
