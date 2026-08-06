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

func seedBytes(seeds ...int64) []byte {
	buf := make([]byte, 8*len(seeds))
	for i, s := range seeds {
		binary.BigEndian.PutUint64(buf[i*8:], uint64(s))
	}
	return buf
}
