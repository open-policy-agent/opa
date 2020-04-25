// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package init

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
	"github.com/open-policy-agent/opa/version"
)

func TestInit(t *testing.T) {

	mod1 := `package a.b.c

import data.a.foo

p = true { foo = "bar" }
p = true { 1 = 2 }`

	mod2 := `package b.c.d

import data.b.foo

p = true { foo = "bar" }
p = true { 1 = 2 }`

	tests := []struct {
		note         string
		fs           map[string]string
		loadParams   []string
		expectedData map[string]string
		expectedMods []string
		asBundle     bool
	}{
		{
			note: "load files",
			fs: map[string]string{
				"datafile":   `{"foo": "bar", "x": {"y": {"z": [1]}}}`,
				"policyFile": mod1,
			},
			loadParams: []string{"datafile", "policyFile"},
			expectedData: map[string]string{
				"/foo": "bar",
			},
			expectedMods: []string{mod1},
			asBundle:     false,
		},
		{
			note: "load bundle",
			fs: map[string]string{
				"datafile":    `{"foo": "bar", "x": {"y": {"z": [1]}}}`, // Should be ignored
				"data.json":   `{"foo": "not-bar"}`,
				"policy.rego": mod1,
			},
			loadParams: []string{"/"},
			expectedData: map[string]string{
				"/foo": "not-bar",
			},
			expectedMods: []string{mod1},
			asBundle:     true,
		},
		{
			note: "load multiple bundles",
			fs: map[string]string{
				"/bundle1/a/data.json":   `{"foo": "bar1", "x": {"y": {"z": [1]}}}`, // Should be ignored
				"/bundle1/a/policy.rego": mod1,
				"/bundle1/a/.manifest":   `{"roots": ["a"]}`,
				"/bundle2/b/data.json":   `{"foo": "bar2"}`,
				"/bundle2/b/policy.rego": mod2,
				"/bundle2/b/.manifest":   `{"roots": ["b"]}`,
			},
			loadParams: []string{"bundle1", "bundle2"},
			expectedData: map[string]string{
				"/a/foo": "bar1",
				"/b/foo": "bar2",
			},
			expectedMods: []string{mod1, mod2},
			asBundle:     true,
		},
		{
			note: "preserve OPA version",
			fs: map[string]string{
				"/root/system/version/data.json": `{"version": "XYZ"}`, // Should be overwritten
			},
			loadParams: []string{"root"},
			expectedData: map[string]string{
				"/system/version/version": version.Version,
			},
			asBundle: true,
		},
	}

	ctx := context.Background()

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.fs, func(rootDir string) {

				paths := []string{}

				for _, fileName := range tc.loadParams {
					paths = append(paths, filepath.Join(rootDir, fileName))
				}

				// Create a new store and perform a file load/insert sequence.
				store := inmem.New()

				err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {

					loaded, err := LoadPaths(paths, nil, tc.asBundle)
					if err != nil {
						return err
					}

					_, err = InsertAndCompile(ctx, InsertAndCompileOptions{
						Store:     store,
						Txn:       txn,
						Files:     loaded.Files,
						Bundles:   loaded.Bundles,
						MaxErrors: -1,
					})

					return err
				})

				if err != nil {
					t.Fatal(err)
				}

				// Verify the loading was successful as expected.
				txn := storage.NewTransactionOrDie(ctx, store)

				for storePath, expected := range tc.expectedData {
					node, err := store.Read(ctx, txn, storage.MustParsePath(storePath))
					if util.Compare(node, expected) != 0 || err != nil {
						t.Fatalf("Expected %v but got %v (err: %v)", expected, node, err)
					}
				}

				ids, err := store.ListPolicies(ctx, txn)
				if err != nil {
					t.Fatal(err)
				}

				if len(tc.expectedMods) != len(ids) {
					t.Fatalf("Expected %d modules, got %d", len(tc.expectedMods), len(ids))
				}

				actualMods := map[string]struct{}{}
				for _, id := range ids {
					result, err := store.GetPolicy(ctx, txn, id)
					if err != nil {
						t.Fatalf("Unexpected error: %s", err)
					}
					actualMods[string(result)] = struct{}{}
				}

				for _, expectedMod := range tc.expectedMods {
					if _, found := actualMods[expectedMod]; !found {
						t.Fatalf("Expected %v but got: %v", expectedMod, actualMods)
					}
				}

				_, err = store.Read(ctx, txn, storage.MustParsePath("/system/version"))
				if err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}
