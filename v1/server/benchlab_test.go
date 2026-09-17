// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package server

import (
	"testing"
)

// The only handler coverage that runs in-process rather than over a socket:
// POST /v1/data, and POST /v1/compile on both translation backends.
func BenchmarkBenchlabDataPostV1Request(b *testing.B) {
	BenchmarkDataPostV1Request(b)
}

func BenchmarkBenchlabCompileHandler(b *testing.B) {
	BenchmarkCompileHandler(b)
}
