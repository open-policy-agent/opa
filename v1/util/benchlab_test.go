// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.

package util_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/v1/util"
)

// util.RoundTripFast is on the path of every store write, every logged decision
// and every rego result. The three named scenarios each take a different branch:
// no-op, number conversion, and the full reflective walk.
func BenchmarkBenchlabRoundTripFast(b *testing.B) {
	b.Run("zero-allocs", func(b *testing.B) {
		act := []any{nil, false, true, "string", json.Number("1")}
		exp := slices.Clone(act)

		var cpy []any
		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range cpy {
				if err := util.RoundTripFast(&cpy[i]); err != nil {
					b.Fatal(err)
				}
			}
		}
		if !slices.Equal(exp, cpy) {
			b.Fatal("expected inputs to be unchanged")
		}
	})

	b.Run("less-allocs to json.Number", func(b *testing.B) {
		act := []any{1.1, 1000, -22}
		exp := []any{json.Number("1.1"), json.Number("1000"), json.Number("-22")}

		var cpy []any
		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range cpy {
				if err := util.RoundTripFast(&cpy[i]); err != nil {
					b.Fatal(err)
				}
			}
		}
		if !slices.Equal(exp, cpy) {
			b.Fatalf("unexpected: %v", cpy)
		}
	})

	b.Run("full-allocs collections", func(b *testing.B) {
		exp := []any{[]any{json.Number("1"), json.Number("2"), json.Number("3")}, map[string]any{"foo": "bar"}}
		act := []any{[]int{1, 2, 3}, map[string]string{"foo": "bar"}}

		var cpy []any
		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range act {
				if err := util.RoundTripFast(&cpy[i]); err != nil {
					b.Fatal(err)
				}
			}
		}
		if !reflect.DeepEqual(exp, cpy) {
			b.Fatalf("expected %v, got %v", exp, cpy)
		}
	})

	// The already-native walk, at the ends of its sweep.
	for _, n := range []int{100, 10000} {
		tree := benchNativeTree(n)

		b.Run(fmt.Sprintf("already-native tree/leaves=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				v := any(tree)
				if err := util.RoundTripFast(&v); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// The legacy RoundTrip, kept as the anchor that tells a RoundTripFast
// regression apart from one in something they share.
func BenchmarkBenchlabRoundTrip(b *testing.B) {
	b.Run("zero-allocs", func(b *testing.B) {
		act := []any{nil, false, true, "string", json.Number("1")}
		var cpy []any
		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range cpy {
				if err := util.RoundTrip(&cpy[i]); err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	b.Run("full-allocs collections", func(b *testing.B) {
		act := []any{[]int{1, 2, 3}, map[string]string{"foo": "bar"}}
		var cpy []any
		for b.Loop() {
			cpy = slices.Clone(act)
			for i := range act {
				if err := util.RoundTrip(&cpy[i]); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}

// gzip request-body decode, used by five server handlers and the authorizer.
// b.RunParallel, so expect more spread than its neighbours.
func BenchmarkBenchlabReadMaybeCompressedBody(b *testing.B) {
	BenchmarkReadMaybeCompressedBody(b)
}
