// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package dependencies

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

// The largest of the three rule counts, named so the chart says which one.
func BenchmarkBenchlabBase(b *testing.B) {
	b.Run("50", func(b *testing.B) {
		compiler, ref := benchlabPolicy(b, 50)
		for b.Loop() {
			if _, err := Base(compiler, ref); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkBenchlabVirtual(b *testing.B) {
	b.Run("50", func(b *testing.B) {
		compiler, ref := benchlabPolicy(b, 50)
		for b.Loop() {
			if _, err := Virtual(compiler, ref); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func benchlabPolicy(b *testing.B, ruleCount int) (*ast.Compiler, ast.Ref) {
	b.Helper()
	module := ast.MustParseModule(makePolicy(ruleCount))
	compiler := ast.NewCompiler()
	if compiler.Compile(map[string]*ast.Module{"test": module}); compiler.Failed() {
		b.Fatalf("failed to compile policy: %v", compiler.Errors)
	}
	return compiler, ast.MustParseRef("data.test.main")
}
