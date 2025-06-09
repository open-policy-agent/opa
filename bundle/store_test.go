// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/storage/mock"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
)

func TestHasRootsOverlap(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		note        string
		storeRoots  map[string]*[]string
		bundleRoots map[string]*[]string
		overlaps    bool
	}{
		{
			note:        "no overlap with existing roots",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c"}},
			overlaps:    false,
		},
		{
			note:        "no overlap with existing roots multiple bundles",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c"}, "bundle3": {"d"}},
			overlaps:    false,
		},
		{
			note:        "no overlap no existing roots",
			storeRoots:  map[string]*[]string{},
			bundleRoots: map[string]*[]string{"bundle1": {"a", "b"}},
			overlaps:    false,
		},
		{
			note:        "no overlap without existing roots multiple bundles",
			storeRoots:  map[string]*[]string{},
			bundleRoots: map[string]*[]string{"bundle1": {"a", "b"}, "bundle2": {"c"}},
			overlaps:    false,
		},
		{
			note:        "overlap without existing roots multiple bundles",
			storeRoots:  map[string]*[]string{},
			bundleRoots: map[string]*[]string{"bundle1": {"a", "b"}, "bundle2": {"a", "c"}},
			overlaps:    true,
		},
		{
			note:        "overlap with existing roots",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c", "a"}},
			overlaps:    true,
		},
		{
			note:        "overlap with existing roots multiple bundles",
			storeRoots:  map[string]*[]string{"bundle1": {"a", "b"}},
			bundleRoots: map[string]*[]string{"bundle2": {"c", "a"}, "bundle3": {"a"}},
			overlaps:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			mockStore := mock.New()
			txn := storage.NewTransactionOrDie(ctx, mockStore, storage.WriteParams)

			for name, roots := range tc.storeRoots {
				err := WriteManifestToStore(ctx, mockStore, txn, name, Manifest{Roots: roots})
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
			}

			bundles := map[string]*Bundle{}
			for name, roots := range tc.bundleRoots {
				bundles[name] = &Bundle{
					Manifest: Manifest{
						Roots: roots,
					},
				}
			}

			mockStore.AssertValid(t)
		})
	}
}

func TestActivate_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note              string
		module            string
		customRegoVersion ast.RegoVersion
		expErrs           []string
	}{
		// default rego-version
		{
			note: "v0 module, no v1 parse-time violations",
			module: `package test
					p[x] { 
						x = "a" 
					}`,
		},
		{
			note: "v0 module, v1 parse-time violations",
			module: `package test

					contains[x] { 
						x = "a" 
					}`,
		},

		// cross-rego-version
		{
			note: "rego.v1 import, no v1 parse-time violations",
			module: `package test
					import rego.v1

					p contains x if { 
						x = "a" 
					}`,
		},
		{
			note: "rego.v1 import, v1 parse-time violations",
			module: `package test
					import rego.v1

					p contains x { 
						x = "a" 
					}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
			},
		},

		// NOT default rego-version
		{
			note: "v1 module",
			module: `package test

					p contains x if { 
						x = "a" 
					}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
			},
		},

		// custom rego-version
		{
			note: "v1 module, v1 custom rego-version",
			module: `package test

					p contains x if { 
						x = "a" 
					}`,
			customRegoVersion: ast.RegoV1,
		},
		{
			note: "v1 module, v1 custom rego-version, v1 parse-time violations",
			module: `package test

					p contains x { 
						x = "a" 
					}`,
			customRegoVersion: ast.RegoV1,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := context.Background()
			store := mock.New()
			txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
			compiler := ast.NewCompiler().WithDefaultRegoVersion(ast.RegoV0CompatV1)
			m := metrics.New()

			bundleName := "bundle1"
			modulePath := "test/policy.rego"

			// We want to make assert that the default rego-version is used, which it is when a module is erased from storage and we don't know what version it has.
			// Therefore, we add a module to the store, which is the replaced by the Activate() call, causing an erase.
			if err := store.UpsertPolicy(ctx, txn, fmt.Sprintf("%s/%s", bundleName, modulePath), []byte(tc.module)); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			newModule := `package test`
			bundles := map[string]*Bundle{
				bundleName: {
					Manifest: Manifest{
						Roots: &[]string{"test"},
					},
					Modules: []ModuleFile{
						{
							Path:   modulePath,
							Raw:    []byte(newModule),
							Parsed: ast.MustParseModule(newModule),
						},
					},
				},
			}

			opts := ActivateOpts{
				Ctx:      ctx,
				Txn:      txn,
				Store:    store,
				Compiler: compiler,
				Metrics:  m,
				Bundles:  bundles,
			}

			if tc.customRegoVersion != ast.RegoUndefined {
				opts.ParserOptions.RegoVersion = tc.customRegoVersion
			}

			err := Activate(&opts)

			if len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("Expected error but got nil for test: %s", tc.note)
				}
				for _, expErr := range tc.expErrs {
					if err := err.Error(); !strings.Contains(err, expErr) {
						t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
					}
				}
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDeactivate_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note              string
		module            string
		customRegoVersion ast.RegoVersion
		expErrs           []string
	}{
		// default rego-version
		{
			note: "v0 module, no v1 parse-time violations",
			module: `package test
					p[x] { 
						x = "a" 
					}`,
		},
		{
			note: "v0 module, v1 parse-time violations",
			module: `package test

					contains[x] { 
						x = "a" 
					}`,
		},

		// cross-rego-version
		{
			note: "rego.v1 import, no v1 parse-time violations",
			module: `package test
					import rego.v1

					p contains x if { 
						x = "a" 
					}`,
		},
		{
			note: "rego.v1 import, v1 parse-time violations",
			module: `package test
					import rego.v1

					p contains x { 
						x = "a" 
					}`,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
			},
		},

		// NOT default rego-version
		{
			note: "v1 module",
			module: `package test

					p contains x if { 
						x = "a" 
					}`,
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
			},
		},

		// custom rego-version
		{
			note: "v1 module, v1 custom rego-version",
			module: `package test

					p contains x if { 
						x = "a" 
					}`,
			customRegoVersion: ast.RegoV1,
		},
		{
			note: "v1 module, v1 custom rego-version, v1 parse-time violations",
			module: `package test

					p contains x { 
						x = "a" 
					}`,
			customRegoVersion: ast.RegoV1,
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := context.Background()
			store := mock.New()
			txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

			bundleName := "bundle1"
			modulePath := "test/policy.rego"

			// We want to make assert that the default rego-version is used, which it is when a module is erased from storage and we don't know what version it has.
			// Therefore, we add a module to the store, which is the replaced by the Activate() call, causing an erase.
			if err := store.UpsertPolicy(ctx, txn, fmt.Sprintf("%s/%s", bundleName, modulePath), []byte(tc.module)); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			opts := DeactivateOpts{
				Ctx:   ctx,
				Txn:   txn,
				Store: store,
				BundleNames: map[string]struct{}{
					fmt.Sprintf("%s/%s", bundleName, modulePath): {},
				},
			}

			if tc.customRegoVersion != ast.RegoUndefined {
				opts.ParserOptions.RegoVersion = tc.customRegoVersion
			}

			err := Deactivate(&opts)

			if len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("Expected error but got nil for test: %s", tc.note)
				}
				for _, expErr := range tc.expErrs {
					if err := err.Error(); !strings.Contains(err, expErr) {
						t.Fatalf("Expected error to contain:\n\n%s\n\nbut got:\n\n%s", expErr, err)
					}
				}
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
		})
	}
}
