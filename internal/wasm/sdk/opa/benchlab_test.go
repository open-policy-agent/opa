// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package opa_test

import (
	"testing"
)

func BenchmarkBenchlabWasmRego(b *testing.B) {
	BenchmarkWasmRego(b)
}

// The Go counterpart of BenchmarkBenchlabWasmRego, measured alongside it so a
// move in the WASM number can be told apart from a move in eval itself.
func BenchmarkBenchlabGoRego(b *testing.B) {
	BenchmarkGoRego(b)
}

func BenchmarkBenchlabWasmCompilation(b *testing.B) {
	BenchmarkWasmCompilation(b)
}

func BenchmarkBenchlabWASMVirtualDocs(b *testing.B) {
	b.Run("total=1000/hit=1", func(b *testing.B) {
		runVirtualDocsBenchmark(b, 1000, 1)
	})
}

func BenchmarkBenchlabWASMLargeJSON(b *testing.B) {
	b.Run("10x10000", func(b *testing.B) {
		runLargeJSONBenchmark(b, 10, 10000)
	})
}
