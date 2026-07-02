// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package failtracer_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/server/failtracer"
	"github.com/open-policy-agent/opa/v1/topdown"
)

func evtFromExpr(e string) topdown.Event {
	return topdown.Event{
		Op:   topdown.FailOp,
		Node: ast.MustParseExpr(e),
	}
}

func TestHints(t *testing.T) {
	for _, tc := range []struct {
		note     string
		evts     []topdown.Event
		unknowns []string
		exp      []failtracer.Hint
	}{
		{
			note: "simple typo, two-part ref",
			evts: []topdown.Event{
				evtFromExpr(`__local1__ = input.fruit.price`),
			},
			unknowns: []string{"input.fruits"},
			exp: []failtracer.Hint{
				{Message: "input.fruit.price undefined, did you mean input.fruits.price?"},
			},
		},
		{
			note: "failing function call",
			evts: []topdown.Event{
				evtFromExpr(`data.policy.udf(__local1__)`),
			},
			unknowns: []string{"input.fruits"},
			exp:      nil,
		},
		{
			note: "all of input unknown, ignored",
			evts: []topdown.Event{
				evtFromExpr(`__local1__ = input.fruit.price`),
			},
			unknowns: []string{"input"},
			exp:      nil,
		},
		{
			note: "simple typo, short ref",
			evts: []topdown.Event{
				evtFromExpr(`__local1__ = input.prize`),
			},
			unknowns: []string{"input.price"},
			exp: []failtracer.Hint{
				{Message: "input.prize undefined, did you mean input.price?"},
			},
		},
		{
			note: "large distance, no hint",
			evts: []topdown.Event{
				evtFromExpr(`__local1__ = input.fruit.price`),
			},
			unknowns: []string{"input.baskets"},
			exp:      nil,
		},
		{
			note: "simple typo, multiple fail events",
			evts: []topdown.Event{
				evtFromExpr(`__local1__ = input.fruit.price`),
				evtFromExpr(`__local2__ = input.fruit.colour`),
			},
			unknowns: []string{"input.fruits"},
			exp: []failtracer.Hint{
				{Message: "input.fruit.price undefined, did you mean input.fruits.price?"},
				{Message: "input.fruit.colour undefined, did you mean input.fruits.colour?"},
			},
		},
		{
			note: "same typo, multiple fail events",
			evts: []topdown.Event{
				evtFromExpr(`__local1__ = input.fruit.price`),
				evtFromExpr(`__local2__ = input.fruit.price`),
			},
			unknowns: []string{"input.fruits"},
			exp: []failtracer.Hint{
				{Message: "input.fruit.price undefined, did you mean input.fruits.price?"},
			},
		},
	} {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			ft := failtracer.New()
			for i := range tc.evts {
				ft.TraceEvent(tc.evts[i])
			}
			unk := make([]ast.Ref, len(tc.unknowns))
			for i := range tc.unknowns {
				unk[i] = ast.MustParseRef(tc.unknowns[i])
			}

			hints := ft.Hints(unk)
			if diff := cmp.Diff(tc.exp, hints, cmpopts.IgnoreFields(failtracer.Hint{}, "Location")); diff != "" {
				t.Errorf("unexpected hints (-want, +got):\n%s", diff)
			}
		})
	}
}
