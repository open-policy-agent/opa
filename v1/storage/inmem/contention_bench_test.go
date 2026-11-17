// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem_test

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

// genAuthzData builds a synthetic dataset shaped like a typical authorization
// store: a large `users` object keyed by id, each with roles and attributes,
// plus a `resources` object. Size scales with nUsers.
func genAuthzData(nUsers, nRoles int) map[string]any {
	users := make(map[string]any, nUsers)
	for i := range nUsers {
		roles := make([]any, nRoles)
		for j := range nRoles {
			roles[j] = fmt.Sprintf("role-%d-%d", i, j)
		}
		users[strconv.Itoa(i)] = map[string]any{
			"roles": roles,
			"attrs": map[string]any{
				"team":   fmt.Sprintf("team-%d", i%64),
				"region": fmt.Sprintf("region-%d", i%8),
				"active": true,
			},
		}
	}
	resources := make(map[string]any, nUsers/10)
	for i := range nUsers / 10 {
		resources[strconv.Itoa(i)] = map[string]any{
			"owner": strconv.Itoa(i),
			"tags":  []any{"tag-a", "tag-b"},
		}
	}
	return map[string]any{"users": users, "resources": resources}
}

// newContentionStore also returns the JSON size of the data in MB. Report it
// after the timed loop: b.ResetTimer discards metrics reported before it.
func newContentionStore(b *testing.B, nUsers int, astValues bool) (storage.Store, float64) {
	b.Helper()
	data := genAuthzData(nUsers, 4)
	bs, err := json.Marshal(data)
	if err != nil {
		b.Fatal(err)
	}
	sizeMB := float64(len(bs)) / (1 << 20)
	if astValues {
		return inmem.NewFromObjectWithOpts(data, inmem.OptReturnASTValuesOnRead(true)), sizeMB
	}
	return inmem.NewFromObjectWithOpts(data, inmem.OptRoundTripOnWrite(false)), sizeMB
}

// readerPool runs n goroutines that each loop over one read transaction at a
// time, recording each one's latency while measuring. Latency includes time
// blocked in NewTransaction behind a pending writer, as a REST client sees it.
type readerPool struct {
	stop      chan struct{}
	wg        sync.WaitGroup
	completed atomic.Int64
	measuring atomic.Bool

	mu    sync.Mutex
	holds []time.Duration
	err   error
}

// startReaders returns once every reader has finished at least one
// transaction, so the writer is timed against readers in steady state.
func startReaders(n int, read func(rng *rand.Rand) error) *readerPool {
	p := &readerPool{stop: make(chan struct{})}
	for i := range n {
		p.wg.Add(1)
		go func(seed uint64) {
			defer p.wg.Done()
			rng := rand.New(rand.NewPCG(seed, 0))
			var local []time.Duration
			defer func() {
				p.mu.Lock()
				p.holds = append(p.holds, local...)
				p.mu.Unlock()
			}()
			for {
				select {
				case <-p.stop:
					return
				default:
				}
				t0 := time.Now()
				if err := read(rng); err != nil {
					p.mu.Lock()
					p.err = err
					p.mu.Unlock()
					return
				}
				if p.measuring.Load() {
					local = append(local, time.Since(t0))
				}
				p.completed.Add(1)
			}
		}(uint64(i))
	}
	for p.completed.Load() < int64(n) && p.firstErr() == nil {
		time.Sleep(time.Millisecond)
	}
	p.measuring.Store(true)
	return p
}

func (p *readerPool) firstErr() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

func (p *readerPool) finish() ([]time.Duration, error) {
	close(p.stop)
	p.wg.Wait()
	return p.holds, p.err
}

// measureCommits times b.N single-value writes, reporting only the Commit
// call: that is where the writer waits for in-flight readers to drain.
func measureCommits(b *testing.B, store storage.Store) []time.Duration {
	b.Helper()
	ctx := b.Context()
	writePath := storage.Path{"users", "0", "attrs", "updated"}
	commits := make([]time.Duration, 0, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txn, err := store.NewTransaction(ctx, storage.WriteParams)
		if err != nil {
			b.Fatal(err)
		}
		if err := store.Write(ctx, txn, storage.AddOp, writePath, strconv.Itoa(i)); err != nil {
			b.Fatal(err)
		}
		t0 := time.Now()
		if err := store.Commit(ctx, txn); err != nil {
			b.Fatal(err)
		}
		commits = append(commits, time.Since(t0))
	}
	b.StopTimer()
	return commits
}

func reportDurations(b *testing.B, prefix string, ds []time.Duration) {
	b.Helper()
	if len(ds) == 0 {
		return
	}
	slices.Sort(ds)
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	b.ReportMetric(ms(sum/time.Duration(len(ds))), prefix+"-avg-ms")
	b.ReportMetric(ms(ds[len(ds)*99/100]), prefix+"-p99-ms")
	b.ReportMetric(ms(ds[len(ds)-1]), prefix+"-max-ms")
}

// BenchmarkPutUnderReaders measures commit latency for a single writer while
// readers hold raw store read transactions open for a fixed sleep. It isolates
// the store lock from evaluation cost.
//
// Run: go test ./v1/storage/inmem/ -run '^$' -bench BenchmarkPutUnderReaders -benchtime=3s
func BenchmarkPutUnderReaders(b *testing.B) {
	cases := []struct {
		name    string
		nUsers  int
		readers int
		hold    time.Duration
		ast     bool
	}{
		{"users130k/0readers/hold0", 130000, 0, 0, false},
		{"users130k/50readers/hold10ms", 130000, 50, 10 * time.Millisecond, false},
		{"users130k/200readers/hold10ms", 130000, 200, 10 * time.Millisecond, false},
		{"users130k/200readers/hold10ms/AST", 130000, 200, 10 * time.Millisecond, true},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			store, sizeMB := newContentionStore(b, tc.nUsers, tc.ast)
			ctx := b.Context()

			pool := startReaders(tc.readers, func(rng *rand.Rand) error {
				txn, err := store.NewTransaction(ctx)
				if err != nil {
					return err
				}
				defer store.Abort(ctx, txn)
				_, _ = store.Read(ctx, txn, storage.Path{"users", strconv.Itoa(rng.IntN(tc.nUsers))})
				time.Sleep(tc.hold)
				return nil
			})

			commits := measureCommits(b, store)
			holds, err := pool.finish()
			if err != nil {
				b.Fatal(err)
			}
			reportDurations(b, "commit", commits)
			reportDurations(b, "reader", holds)
			b.ReportMetric(sizeMB, "data-MB")
		})
	}
}

// lookupPolicy reads one user by key, like a typical point authorization check.
const lookupPolicy = `package authz

default allow := false

allow if {
	user := data.users[input.user]
	user.attrs.active
	some role in user.roles
	startswith(role, "role-")
}
`

// scanPolicy iterates every user on each request, so the read transaction
// stays open for as long as the full scan takes.
const scanPolicy = `package authz

default allow := false

allow if {
	user := data.users[input.user]
	teammates := [id | some id, u in data.users; u.attrs.team == user.attrs.team]
	count(teammates) > 0
}
`

// BenchmarkPutUnderEval measures commit latency for a single writer while
// readers run prepared Rego queries, each inside its own read transaction the
// way the REST server does for POST /v1/data. Reader hold time comes from real
// evaluation instead of a sleep.
//
// Run: go test ./v1/storage/inmem/ -run '^$' -bench BenchmarkPutUnderEval -benchtime=20x
func BenchmarkPutUnderEval(b *testing.B) {
	cases := []struct {
		name    string
		policy  string
		nUsers  int
		readers int
		ast     bool
	}{
		{"users130k/lookup/200readers", lookupPolicy, 130000, 200, false},
		{"users130k/lookup/200readers/AST", lookupPolicy, 130000, 200, true},
		{"users130k/scan/50readers", scanPolicy, 130000, 50, false},
		{"users130k/scan/50readers/AST", scanPolicy, 130000, 50, true},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			store, sizeMB := newContentionStore(b, tc.nUsers, tc.ast)
			ctx := b.Context()

			pq, err := rego.New(
				rego.Query("data.authz.allow"),
				rego.Module("authz.rego", tc.policy),
				rego.Store(store),
			).PrepareForEval(ctx)
			if err != nil {
				b.Fatal(err)
			}

			pool := startReaders(tc.readers, func(rng *rand.Rand) error {
				txn, err := store.NewTransaction(ctx)
				if err != nil {
					return err
				}
				defer store.Abort(ctx, txn)
				input := map[string]any{"user": strconv.Itoa(rng.IntN(tc.nUsers))}
				rs, err := pq.Eval(ctx, rego.EvalTransaction(txn), rego.EvalInput(input))
				if err != nil {
					return err
				}
				if len(rs) != 1 || rs[0].Expressions[0].Value != true {
					return fmt.Errorf("unexpected result: %v", rs)
				}
				return nil
			})

			commits := measureCommits(b, store)
			holds, err := pool.finish()
			if err != nil {
				b.Fatal(err)
			}
			reportDurations(b, "commit", commits)
			reportDurations(b, "reader", holds)
			b.ReportMetric(sizeMB, "data-MB")
		})
	}
}

// conflictPolicy is a set of rules that share a prefix with the data
// being written, which is what makes the server's per-PUT conflict check read
// that prefix back.
func conflictPolicy(rules int) string {
	var b strings.Builder
	b.WriteString("package rootapp.authz\nimport rego.v1\n")
	for i := range rules {
		fmt.Fprintf(&b, "p%d if { true }\n", i)
	}
	return b.String()
}

// BenchmarkConflictCheck times the path-conflict check a PUT runs after Write
// and before Commit. The data and the rules share the rootapp prefix, and no
// readers are running.
//
// Run: go test ./v1/storage/inmem/ -run '^$' -bench BenchmarkConflictCheck -benchtime=5x
func BenchmarkConflictCheck(b *testing.B) {
	const rules = 40
	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{
		"authz.rego": ast.MustParseModule(conflictPolicy(rules)),
	})
	if compiler.Failed() {
		b.Fatal(compiler.Errors)
	}

	cases := []struct {
		name string
		ast  bool
	}{
		{"Go", false},
		{"AST", true},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			data := map[string]any{"rootapp": genAuthzData(130000, 4)}
			var store storage.Store
			var sizeMB float64
			if bs, err := json.Marshal(data); err == nil {
				sizeMB = float64(len(bs)) / (1 << 20)
			}
			if tc.ast {
				store = inmem.NewFromObjectWithOpts(data, inmem.OptReturnASTValuesOnRead(true))
			} else {
				store = inmem.NewFromObjectWithOpts(data, inmem.OptRoundTripOnWrite(false))
			}
			ctx := b.Context()
			writePath := storage.Path{"rootapp", "users", "0", "attrs", "updated"}
			var checks []time.Duration

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				txn, err := store.NewTransaction(ctx, storage.WriteParams)
				if err != nil {
					b.Fatal(err)
				}
				if err := store.Write(ctx, txn, storage.AddOp, writePath, strconv.Itoa(i)); err != nil {
					b.Fatal(err)
				}
				t0 := time.Now()
				if errs := ast.CheckPathConflicts(compiler, storage.NonEmpty(ctx, store, txn)); len(errs) > 0 {
					b.Fatal(errs)
				}
				checks = append(checks, time.Since(t0))
				store.Abort(ctx, txn)
			}
			b.StopTimer()
			reportDurations(b, "check", checks)
			b.ReportMetric(sizeMB, "data-MB")
			b.ReportMetric(float64(rules), "rules")
		})
	}
}
