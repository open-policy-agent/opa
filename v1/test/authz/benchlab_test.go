// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package authz

import (
	"testing"
)

// OPA as an API authorization layer: 1000 tokens against 10, 100 and 1000
// paths, through a prepared query. Deny-early, deny-late and three allow
// scales.
//
// The disk arm of BenchmarkAuthzForbidAuthn is left out: it is Badger I/O and
// background compaction, two orders of magnitude slower than the inmem arm and
// measuring the store rather than the policy.
func BenchmarkBenchlabAuthzForbidAuthn(b *testing.B) {
	b.Run("inmem", func(b *testing.B) {
		runAuthzBenchmark(b, ForbidIdentity, 10)
	})
}

func BenchmarkBenchlabAuthzForbidPath(b *testing.B) {
	runAuthzBenchmark(b, ForbidPath, 10)
}

func BenchmarkBenchlabAuthzForbidMethod(b *testing.B) {
	runAuthzBenchmark(b, ForbidMethod, 10)
}

func BenchmarkBenchlabAuthzAllow10Paths(b *testing.B) {
	runAuthzBenchmark(b, Allow, 10)
}

func BenchmarkBenchlabAuthzAllow100Paths(b *testing.B) {
	runAuthzBenchmark(b, Allow, 100)
}

func BenchmarkBenchlabAuthzAllow1000Paths(b *testing.B) {
	runAuthzBenchmark(b, Allow, 1000)
}
