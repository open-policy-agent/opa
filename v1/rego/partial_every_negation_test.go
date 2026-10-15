// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego_test

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/v1/rego"
)

// Partial evaluation used to panic when the query negated a rule whose body
// holds an 'every' over a part of the input left unknown. The negation is
// saved in a support rule now, so for any concrete input, evaluating the
// partial result must give the same answer as evaluating the original
// policy directly.
// See https://github.com/open-policy-agent/opa/issues/9383.
func TestPartialEvalNegatedEveryEquivalence(t *testing.T) {
	policy := `package p

allow if not blocked

blocked if {
	every x in input.xs {
		x == 1
	}
}
`

	// The inputs from the issue report: 'xs' missing, and five values for it.
	inputs := []string{
		`{}`,
		`{"xs": []}`,
		`{"xs": [1]}`,
		`{"xs": [2]}`,
		`{"xs": [1, 2]}`,
		`{"xs": [1, 1]}`,
	}

	ctx := t.Context()

	pq, err := rego.New(
		rego.Query("data.p.allow"),
		rego.Unknowns([]string{"input"}),
		rego.Module("p.rego", policy),
	).Partial(ctx)
	if err != nil {
		t.Fatalf("unexpected error on partial evaluation: %v", err)
	}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			var input any
			if err := json.Unmarshal([]byte(in), &input); err != nil {
				t.Fatalf("unexpected error on input parse: %v", err)
			}

			directRS, err := rego.New(
				rego.Query("data.p.allow"),
				rego.Module("p.rego", policy),
				rego.Input(input),
			).Eval(ctx)
			if err != nil {
				t.Fatalf("unexpected error on direct evaluation: %v", err)
			}

			// The partial result is defined if any of its queries is defined.
			partialDefined := false
			for _, q := range pq.Queries {
				opts := []func(*rego.Rego){rego.ParsedQuery(q), rego.Input(input)}
				for _, m := range pq.Support {
					opts = append(opts, rego.ParsedModule(m))
				}
				rs, err := rego.New(opts...).Eval(ctx)
				if err != nil {
					t.Fatalf("unexpected error on partial result evaluation: %v", err)
				}
				if len(rs) > 0 {
					partialDefined = true
					break
				}
			}

			if (len(directRS) > 0) != partialDefined {
				t.Fatalf("expected partial result defined == %v for input %s, got %v (queries: %v, support: %v)",
					len(directRS) > 0, in, partialDefined, pq.Queries, pq.Support)
			}
		})
	}
}
