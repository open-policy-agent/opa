// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package logs

import (
	"testing"
)

// Converting a decision event to AST, paid once per logged decision.
func BenchmarkBenchlabEventAST(b *testing.B) {
	BenchmarkEventAST(b)
}

// Masking with no rule to apply, and with one that erases a field.
func BenchmarkBenchlabMaskingNop(b *testing.B) {
	BenchmarkMaskingNop(b)
}

func BenchmarkBenchlabMaskingErase(b *testing.B) {
	BenchmarkMaskingErase(b)
}
