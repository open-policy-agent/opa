// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package format

import (
	"testing"
)

func BenchmarkBenchlabFormatLargePolicy(b *testing.B) {
	BenchmarkFormatLargePolicy(b)
}
