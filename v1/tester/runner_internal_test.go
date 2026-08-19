// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tester

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/open-policy-agent/opa/v1/cover"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/util/test"
)

// TestRunnerCoverageNonDeterministicBuiltin checks that nondeterministic
// builtins don't change the supplementary coverage diff: the shared cache must
// make the index/early-exit passes replay the baseline's results. It lives in
// package tester so it can set the unexported Runner.seed field.
func TestRunnerCoverageNonDeterministicBuiltin(t *testing.T) {
	// The baseline draws the first seed (0 -> rand.intn(_,2)==0, branch
	// skipped). The shared cache makes the supplementary passes replay it;
	// the later seeds (-> 1, branch taken) are only drawn if a pass
	// recomputes rand - the divergence this guards against.
	seed := seedBytes(0, 1, 1)

	files := map[string]string{
		"/policy.rego": `package p

r if {
	rand.intn("k", 2) == 1
	dispatched
}

dispatched if true`,
		"/policy_test.rego": `package p

test_r if { not r }`,
	}

	ctx := t.Context()

	test.WithTempFS(files, func(d string) {
		modules, store, err := Load([]string{d}, nil)
		if err != nil {
			t.Fatal(err)
		}

		txn := storage.NewTransactionOrDie(ctx, store)
		defer store.Abort(ctx, txn)

		cov := cover.New()
		runner := NewRunner().
			SetStore(store).
			SetModules(modules).
			SetCoverageQueryTracer(cov)
		runner.seed = bytes.NewReader(seed)

		ch, err := runner.RunTests(ctx, txn)
		if err != nil {
			t.Fatal(err)
		}
		for r := range ch {
			if r.Error != nil {
				t.Fatalf("unexpected test error: %v", r.Error)
			}
		}

		report := cov.Report(modules)
		for file, fr := range report.Files {
			for _, rng := range fr.NotCovered {
				if len(rng.Kinds) > 0 {
					t.Errorf("%s:%d-%d incorrectly tagged %v; nondeterministic builtin results must be shared across coverage passes",
						file, rng.Start.Row, rng.End.Row, rng.Kinds)
				}
			}
		}
	})
}

// TestRunnerCoverageRuns checks that SetCoverageRuns controls which
// supplementary passes run, and therefore which Kinds are reported. It lives in
// package tester so it can inspect the runner via the exported setters.
func TestRunnerCoverageRuns(t *testing.T) {
	files := map[string]string{
		// early_exit's second definition is skipped once the first matches;
		// index_excluded's body is index-excluded for input {"a": "z"}.
		"/policy.rego": `package p

early_exit if { true }
early_exit if { early_exit_dep }

early_exit_dep if { true }

index_excluded if { input.a == "read" }`,
		"/policy_test.rego": `package p

test_early_exit if { early_exit }
test_index_excluded if { not index_excluded with input as {"a": "z"} }`,
	}

	kindsFor := func(t *testing.T, runs []cover.Kind, tracerFirst bool) map[cover.Kind]int {
		t.Helper()
		counts := map[cover.Kind]int{}
		test.WithTempFS(files, func(d string) {
			modules, store, err := Load([]string{d}, nil)
			if err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			txn := storage.NewTransactionOrDie(ctx, store)
			defer store.Abort(ctx, txn)

			cov := cover.New()
			runner := NewRunner().
				SetStore(store).
				SetModules(modules)
			// The two coverage setters must be order-independent, like the
			// other fluent setters, so exercise both orderings.
			if tracerFirst {
				runner = runner.SetCoverageQueryTracer(cov).SetCoverageRuns(runs)
			} else {
				runner = runner.SetCoverageRuns(runs).SetCoverageQueryTracer(cov)
			}
			ch, err := runner.RunTests(ctx, txn)
			if err != nil {
				t.Fatal(err)
			}
			for r := range ch {
				if r.Error != nil {
					t.Fatalf("unexpected test error: %v", r.Error)
				}
			}

			for _, fr := range cov.Report(modules).Files {
				for _, rng := range fr.NotCovered {
					for _, k := range rng.Kinds {
						counts[k]++
					}
				}
			}
		})
		return counts
	}

	cases := map[string]struct {
		runs      []cover.Kind
		wantIndex bool
		wantEarly bool
	}{
		"both": {
			runs:      []cover.Kind{cover.KindIndexExcluded, cover.KindEarlyExit},
			wantIndex: true,
			wantEarly: true,
		},
		"index only": {
			runs:      []cover.Kind{cover.KindIndexExcluded},
			wantIndex: true,
			wantEarly: false,
		},
		"early exit only": {
			runs:      []cover.Kind{cover.KindEarlyExit},
			wantIndex: false,
			wantEarly: true,
		},
		"none": {
			runs:      nil,
			wantIndex: false,
			wantEarly: false,
		},
	}

	for name, tc := range cases {
		for _, tracerFirst := range []bool{false, true} {
			name := name
			if tracerFirst {
				name += " (tracer set first)"
			}
			t.Run(name, func(t *testing.T) {
				counts := kindsFor(t, tc.runs, tracerFirst)
				if got := counts[cover.KindIndexExcluded] > 0; got != tc.wantIndex {
					t.Errorf("index_excluded present = %v, want %v (counts: %v)", got, tc.wantIndex, counts)
				}
				if got := counts[cover.KindEarlyExit] > 0; got != tc.wantEarly {
					t.Errorf("early_exit present = %v, want %v (counts: %v)", got, tc.wantEarly, counts)
				}
			})
		}
	}
}

func seedBytes(seeds ...int64) []byte {
	buf := make([]byte, 8*len(seeds))
	for i, s := range seeds {
		binary.BigEndian.PutUint64(buf[i*8:], uint64(s))
	}
	return buf
}
