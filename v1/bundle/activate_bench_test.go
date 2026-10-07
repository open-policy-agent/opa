// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle_test

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func BenchmarkActivate(b *testing.B) {
	for _, tc := range []struct{ leaves, modules int }{{1000, 10}, {100000, 10}} {
		data := make(map[string]any, tc.leaves/10)
		for i := range tc.leaves / 10 {
			obj := make(map[string]any, 10)
			for j := range 10 {
				obj[fmt.Sprintf("k%d", j)] = fmt.Sprintf("value-%d-%d", i, j)
			}
			data[fmt.Sprintf("g%d", i)] = obj
		}

		var files []bundle.ModuleFile
		for i := range tc.modules {
			src := fmt.Sprintf("package p%d\n\nallow if input.x == %d\n", i, i)
			files = append(files, bundle.ModuleFile{
				URL:    fmt.Sprintf("p%d.rego", i),
				Path:   fmt.Sprintf("p%d.rego", i),
				Raw:    []byte(src),
				Parsed: ast.MustParseModule(src),
			})
		}

		for _, owned := range []bool{false, true} {
			b.Run(fmt.Sprintf("leaves=%d/modules=%d/owned=%v", tc.leaves, tc.modules, owned), func(b *testing.B) {
				b.ReportAllocs()
				ctx := b.Context()
				for b.Loop() {
					bun := &bundle.Bundle{
						Manifest: bundle.Manifest{Roots: &[]string{""}},
						Data:     data,
						Modules:  files,
					}
					store := inmem.New()
					params := storage.WriteParams
					params.Context = storage.NewContext()
					params.OwnedWrites = owned
					err := storage.Txn(ctx, store, params, func(txn storage.Transaction) error {
						return bundle.Activate(&bundle.ActivateOpts{
							Ctx:      ctx,
							Store:    store,
							Txn:      txn,
							TxnCtx:   params.Context,
							Compiler: ast.NewCompiler(),
							Metrics:  metrics.New(),
							Bundles:  map[string]*bundle.Bundle{"b": bun},
						})
					})
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
