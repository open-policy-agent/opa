// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package compile

import (
	"io/fs"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/util/test"
)

func BenchmarkBenchlabCompileDynamicPolicy(b *testing.B) {
	for _, n := range []int{1000, 5000, 10000} {
		testcase := generateDynamicPolicyBenchmarkData(n)
		test.WithTestFS(testcase, true, func(root string, fileSys fs.FS) {
			b.Run(strconv.Itoa(n), func(b *testing.B) {
				for b.Loop() {
					if err := New().WithFS(fileSys).WithPaths(root).Build(b.Context()); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

func BenchmarkBenchlabLargePartialRulePolicy(b *testing.B) {
	for _, n := range []int{1000, 5000} {
		testcase := generateLargePartialRuleBenchmarkData(n)
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			test.WithTempFS(testcase, func(root string) {
				for b.Loop() {
					if err := New().WithPaths(root).Build(b.Context()); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
