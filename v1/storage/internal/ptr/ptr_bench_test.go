// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ptr_test

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/internal/ptr"
)

// BenchmarkValuePtr/using_pool-16         	11424187	       103.8 ns/op	      64 B/op	       4 allocs/op
// BenchmarkValuePtr/2_interned_terms-16   	11856872	        98.18 ns/op	      32 B/op	       2 allocs/op
// BenchmarkValuePtr/4_interned_terms-16   	13462396	        86.73 ns/op	       0 B/op	       0 allocs/op
func BenchmarkValuePtr(b *testing.B) {
	val := ast.MustInterfaceToValue(map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": map[string]any{"d": "e"},
			},
		},
	})
	path := storage.Path{"a", "b", "c", "d"}

	b.Run("using pool", func(b *testing.B) {
		for b.Loop() {
			if _, err := ptr.ValuePtr(val, path); err != nil {
				b.Fatal(err)
			}
		}
	})

	ast.InternStringTerm("a", "b")

	// Make sure that we handle a mix of interned and non-interned terms.
	b.Run("2 interned terms", func(b *testing.B) {
		for b.Loop() {
			if _, err := ptr.ValuePtr(val, path); err != nil {
				b.Fatal(err)
			}
		}
	})

	ast.InternStringTerm("c", "d")

	b.Run("4 interned terms", func(b *testing.B) {
		for b.Loop() {
			if _, err := ptr.ValuePtr(val, path); err != nil {
				b.Fatal(err)
			}
		}
	})
}
