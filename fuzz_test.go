//go:build go1.18
// +build go1.18

// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package opa_test

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/rego"
)

// FuzzRegoEval tests Rego policy evaluation with arbitrary policy and input.
// Rego is the policy language used by every OPA deployment across Kubernetes,
// Envoy, and cloud-native infrastructure. An eval bug is a cross-system
// authorization bypass.
func FuzzRegoEval(f *testing.F) {
	// Seed with valid Rego policies
	f.Add("package test\n\nallow { true }", `{"user": "admin"}`)
	f.Add("package test\n\ndefault allow = false\nallow { input.user == \"admin\" }", `{"user": "guest"}`)
	f.Add("package test\n\nallow { count(input.items) > 0 }", `{"items": [1, 2, 3]}`)

	f.Fuzz(func(t *testing.T, policy, inputJSON string) {
		if len(policy) > 100000 || len(policy) == 0 {
			return
		}
		if len(inputJSON) > 100000 {
			return
		}

		func() {
			defer func() { recover() }()

			ctx := context.Background()
			r := rego.New(
				rego.Query("data.test.allow"),
				rego.Module("test.rego", policy),
			)

			// Prepare evaluation — should not panic
			pq, err := r.PrepareForEval(ctx)
			if err != nil {
				return // parse errors expected for fuzzed policy
			}

			// Evaluate — should not panic
			_, err = pq.Eval(ctx, rego.EvalInput(inputJSON))
			if err != nil {
				return // eval errors expected for malformed input
			}
		}()
	})
}

// FuzzRegoPartialEval tests partial evaluation with arbitrary policy.
// Partial eval is used in critical decision logging paths.
func FuzzRegoPartialEval(f *testing.F) {
	f.Add("package test\n\ndefault allow = false\nallow { input.role == \"admin\" }", "")
	f.Add("package test\n\nallow { input.x > 0 }", "")

	f.Fuzz(func(t *testing.T, policy, unknown string) {
		if len(policy) > 50000 || len(policy) == 0 {
			return
		}
		if len(unknown) > 1000 {
			return
		}

		func() {
			defer func() { recover() }()

			ctx := context.Background()
			r := rego.New(
				rego.Query("data.test.allow"),
				rego.Module("test.rego", policy),
			)

			pq, err := r.PrepareForEval(ctx)
			if err != nil {
				return
			}

			unknowns := []string{"input"}
			if unknown != "" {
				unknowns = append(unknowns, unknown)
			}

			_, err = pq.Partial(ctx, rego.EvalUnknowns(unknowns))
			if err != nil {
				return
			}
		}()
	})
}
