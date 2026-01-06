// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package bundle_test

import (
	"context"
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/disk"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/test"
)

type customBundleActivator struct {
	activator   *bundle.DefaultActivator
	Files       map[string]map[string][]byte
	BundleNames []string
}

func (cba *customBundleActivator) Activate(opts *bundle.ActivateOpts) error {
	cba.activator = &bundle.DefaultActivator{}
	cba.BundleNames = util.Keys(opts.Bundles)
	if cba.Files == nil {
		cba.Files = make(map[string]map[string][]byte, len(cba.BundleNames))
	}
	for k, v := range opts.Bundles {
		cba.Files[k] = make(map[string][]byte, len(v.Raw))
		for _, r := range v.Raw {
			cba.Files[k][r.Path] = r.Value
		}
	}
	return cba.activator.Activate(opts)
}

// Warning: This test modifies package variables, and as
// a result, cannot be run in parallel with other tests.
func TestRegisterBundleActivatorWithStore(t *testing.T) {
	getInmemStore := func() storage.Store {
		return inmem.NewFromObject(map[string]any{})
	}

	// Top-level variables, shared between tests.
	// These are only needed to make disk storage tests use an external dir name and context value.
	var dir string
	var ctx context.Context
	getDiskStore := func() storage.Store {
		s, err := disk.New(ctx, logging.NewNoOpLogger(), nil, disk.Options{Dir: dir})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}

	buf := archive.MustWriteTarGz([][2]string{
		{"/.manifest", `{"revision": "abcd"}`},
		{"/data.json", `{"a": 1}`},
		{"/x.rego", `package foo`},
	})
	b, err := bundle.NewReader(buf).IncludeManifestInData(true).Read()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		activator bundle.Activator
		storeFunc func() storage.Store
		note      string
		disk      bool
	}{
		{
			note: "package init default activator, default store",
		},
		{
			note:      "default activator, default store",
			activator: &bundle.DefaultActivator{},
		},
		{
			note:      "default activator, inmem store",
			activator: &bundle.DefaultActivator{},
			storeFunc: getInmemStore,
		},
		{
			note:      "custom activator, inmem store",
			activator: &customBundleActivator{},
			storeFunc: getInmemStore,
		},
		{
			note:      "package init default activator, disk store",
			storeFunc: getDiskStore,
			disk:      true,
		},
		{
			note:      "default activator, disk store",
			activator: &bundle.DefaultActivator{},
			storeFunc: getDiskStore,
			disk:      true,
		},
		{
			note:      "custom activator, disk store",
			activator: &customBundleActivator{},
			storeFunc: getDiskStore,
			disk:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var store storage.Store
			ctx = t.Context()

			// Plumb in the bundle store if func provided.
			if tc.storeFunc != nil {
				bundle.RegisterStoreFunc(tc.storeFunc)
				// Create temp folder when using disk storage.
				if tc.disk {
					rootDir, cleanup, err := test.MakeTempFS("", "opa_test", make(map[string]string))
					dir = rootDir // Set top-level var for the storage function to pick up.
					if err != nil {
						panic(err)
					}
					defer cleanup()
				}
				store = bundle.BundleExtStore()
			} else {
				store = getInmemStore() // The default store.
			}

			// Register our custom bundle activator if provided.
			if tc.activator != nil {
				bundle.RegisterActivator("example-activator", tc.activator)
				bundle.RegisterDefaultBundleActivator("example-activator")
			}

			txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
			bundles := map[string]*bundle.Bundle{"example": &b}
			opts := &bundle.ActivateOpts{
				Ctx:      ctx,
				Store:    store,
				Txn:      txn,
				Compiler: ast.NewCompiler(),
				Metrics:  metrics.New(),
				Bundles:  bundles,
			}
			if tc.activator != nil {
				opts.Plugin = "example-activator"
			}

			if err := bundle.Activate(opts); err != nil {
				t.Fatal(err)
			}

			// If using the custom bundle activator, inspect contents.
			if cba, ok := tc.activator.(*customBundleActivator); ok {
				expNames := []string{"example"}
				actNames := cba.BundleNames
				if slices.Compare(expNames, actNames) != 0 {
					t.Fatalf("wrong bundle names. expected: %v, got: %v", expNames, actNames)
				}
				for k, v := range bundles {
					actFilenames := util.Keys(cba.Files[k])
					expFilenames := make([]string, len(v.Raw))
					for _, r := range v.Raw {
						expFilenames = append(expFilenames, r.Path)
					}
					if slices.Compare(expFilenames, actFilenames) != 0 {
						t.Fatalf("wrong bundle file names. expected: %v, got: %v", expFilenames, actFilenames)
					}
				}
			}
		})
	}
}
