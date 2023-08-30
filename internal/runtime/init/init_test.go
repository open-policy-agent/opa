// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package init

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
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

	for _, useMemoryFS := range []bool{false, true} {
		for _, tc := range tests {
			t.Run(tc.note, func(t *testing.T) {
				test.WithTestFS(tc.fs, useMemoryFS, func(rootDir string, fsys fs.FS) {
					paths := []string{}

					for _, fileName := range tc.loadParams {
						paths = append(paths, filepath.Join(rootDir, fileName))
					}
					// Create a new store and perform a file load/insert sequence.
					store := inmem.New()

					err := storage.Txn(ctx, store, storage.WriteParams, func(txn storage.Transaction) error {
						loaded, err := LoadPaths(paths, nil, tc.asBundle, nil, true, false, nil, fsys)
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
}

func TestLoadTarGzsInBundleAndNonBundleMode(t *testing.T) {

	type bundleInfo struct {
		fileName  string
		files     [][2]string
		expBundle bundle.Bundle
	}

	bundle1TarGz := bundleInfo{
		fileName: "bundle1.tar.gz",
		files: [][2]string{
			{"/a/data.json", `{"foo": "bar1", "x": {"y": {"z": [1]}}}`},
			{"/a/.manifest", `{"roots": ["a"]}`},
		},
		expBundle: bundle.Bundle{
			Manifest: bundle.Manifest{
				Roots: &[]string{"a"},
			},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"foo": "bar1",
					"x": map[string]interface{}{
						"y": map[string]interface{}{
							"z": []interface{}{json.Number("1")},
						},
					},
				},
			},
		},
	}

	bundle2TarGz := bundleInfo{
		fileName: "bundle2.tar.gz",
		files: [][2]string{
			{"/b/data.json", `{"foo": "bar2", "x": {"y": {"z": [1]}}}`},
			{"/b/.manifest", `{"roots": ["b"]}`},
		},
		expBundle: bundle.Bundle{
			Manifest: bundle.Manifest{
				Roots: &[]string{"b"},
			},
			Data: map[string]interface{}{
				"b": map[string]interface{}{
					"foo": "bar2",
					"x": map[string]interface{}{
						"y": map[string]interface{}{
							"z": []interface{}{json.Number("1")},
						},
					},
				},
			},
		},
	}

	bundle1Folder := map[string]string{
		"/bundle1/a/data.json": `{"foo1": "bar2", "x": {"y": {"z": [2]}}}`,
		"/bundle1/a/.manifest": `{"roots": ["a"]}`,
		"/bundle1/a/foo.rego":  `package a.b.y`,
	}

	modulePath := "/bundle1/a/foo.rego"
	module := `package a.b.y`

	bundle1FolderInfo := bundleInfo{
		fileName: "bundle1",
		expBundle: bundle.Bundle{
			Manifest: bundle.Manifest{
				Roots: &[]string{"a"},
			},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"foo1": "bar2",
					"x": map[string]interface{}{
						"y": map[string]interface{}{
							"z": []interface{}{json.Number("2")},
						},
					},
				},
			},
			Modules: []bundle.ModuleFile{
				{
					URL:    modulePath,
					Path:   modulePath,
					Parsed: ast.MustParseModule(module),
					Raw:    []byte(module),
				},
			},
		},
	}

	tests := []struct {
		note            string
		bundleInfoTC    []bundleInfo
		folderContent   map[string]string
		expectedBundles int
		expectedModules int
		asBundle        bool
	}{
		{
			note:          "load multiple bundles. one tar.gz and one folder. Bundle mode is true",
			folderContent: bundle1Folder,
			bundleInfoTC: []bundleInfo{
				bundle1TarGz,
				bundle1FolderInfo,
			},
			expectedBundles: 2,
			expectedModules: 0,
			asBundle:        true,
		},
		{
			note:          "load multiple bundles. one tar.gz and one folder. Bundle mode is false",
			folderContent: bundle1Folder,
			bundleInfoTC: []bundleInfo{
				bundle1TarGz,
				bundle1FolderInfo,
			},
			expectedBundles: 1,
			expectedModules: 1,
			asBundle:        false,
		},
		{
			note:          "load multiple bundles. two tar.gz and one folder. Bundle mode is true",
			folderContent: bundle1Folder,
			bundleInfoTC: []bundleInfo{
				bundle1TarGz,
				bundle2TarGz,
				bundle1FolderInfo,
			},
			expectedBundles: 3,
			expectedModules: 0,
			asBundle:        true,
		},
		{
			note:          "load multiple bundles. two tar.gz and one folder. Bundle mode is false",
			folderContent: bundle1Folder,
			bundleInfoTC: []bundleInfo{
				bundle1TarGz,
				bundle2TarGz,
				bundle1FolderInfo,
			},
			expectedBundles: 2,
			expectedModules: 1,
			asBundle:        false,
		},
		{
			note:          "load just one folder. Bundle mode is true",
			folderContent: bundle1Folder,
			bundleInfoTC: []bundleInfo{
				bundle1FolderInfo,
			},
			expectedBundles: 1,
			expectedModules: 0,
			asBundle:        true,
		},
		{
			note:          "load just one folder. Bundle mode is false",
			folderContent: bundle1Folder,
			bundleInfoTC: []bundleInfo{
				bundle1FolderInfo,
			},
			expectedBundles: 0,
			expectedModules: 1,
			asBundle:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			test.WithTempFS(tc.folderContent, func(rootDir string) {
				paths := []string{}
				for _, bdlInfo := range tc.bundleInfoTC {
					if strings.HasSuffix(bdlInfo.fileName, ".tar.gz") {

						// Create the tar gz files temporarily
						buf := archive.MustWriteTarGz(bdlInfo.files)
						bundleFile := filepath.Join(rootDir, bdlInfo.fileName)
						out, err := os.Create(bundleFile)
						if err != nil {
							t.Fatalf("Unexpected error: %v", err)
						}
						_, err = out.Write(buf.Bytes())
						if err != nil {
							t.Fatalf("Unexpected error: %v", err)
						}
					}

					paths = append(paths, filepath.Join(rootDir, bdlInfo.fileName))
				}

				loaded, err := LoadPaths(paths, nil, tc.asBundle, nil, true, false, nil, nil)
				if err != nil {
					t.Fatal("Failed LoadPaths ", err)
				}
				if tc.expectedBundles != len(loaded.Bundles) {
					t.Fatalf("Expected %d bundles, got %d", tc.expectedBundles, len(loaded.Bundles))
				}
				if tc.expectedModules != len(loaded.Files.Modules) {
					t.Fatalf("Expected %d modules, got %d", tc.expectedModules, len(loaded.Files.Modules))
				}

				// Testing the content
				for path, actual := range loaded.Bundles {
					for _, bdlInfo := range tc.bundleInfoTC {
						if strings.HasSuffix(path, bdlInfo.fileName) {
							var buf bytes.Buffer
							if err := bundle.NewWriter(&buf).Write(bdlInfo.expBundle); err != nil {
								t.Fatal(err)
							}

							expected, err := bundle.NewReader(&buf).Read()
							if err != nil {
								t.Fatal(err)
							}

							// adjusting the URL and Path due to /tmp/ path
							if len(bdlInfo.expBundle.Modules) > 0 {
								expected.Modules[0].URL = rootDir + expected.Modules[0].URL
								expected.Modules[0].Path = rootDir + expected.Modules[0].Path
							}

							if !expected.Equal(*actual) {
								t.Fatalf("\nExpected: %+v\nGot: %+v", expected, actual)
							}
						}
					}
				}
			})
		})
	}

}

func TestWalkPaths(t *testing.T) {
	files := map[string]string{
		"/bundle1/a/data.json":   `{"foo": "bar1", "x": {"y": {"z": [1]}}}`,
		"/bundle1/a/policy.rego": `package example.foo`,
		"/bundle1/a/.manifest":   `{"roots": ["a"]}`,
		"/bundle2/b/data.json":   `{"foo": "bar2"}`,
		"/bundle2/b/policy.rego": `package authz`,
		"/bundle2/b/.manifest":   `{"roots": ["b"]}`,
	}

	test.WithTempFS(files, func(rootDir string) {

		paths := []string{}
		paths = append(paths, filepath.Join(rootDir, "bundle1"))
		paths = append(paths, filepath.Join(rootDir, "bundle2"))

		// bundle mode
		loaded, err := WalkPaths(paths, nil, true)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if len(loaded.BundlesLoader) != len(paths) {
			t.Fatalf("Expected %v bundle loaders but got %v", len(paths), len(loaded.BundlesLoader))
		}

		// check files
		result := []string{}
		for _, bl := range loaded.BundlesLoader {
			for {
				f, err := bl.DirectoryLoader.NextFile()
				if err == io.EOF {
					break
				}

				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				result = append(result, f.Path())

				if _, ok := files[strings.TrimPrefix(f.URL(), rootDir)]; !ok {
					t.Fatalf("unexpected file %v", f.Path())
				}
			}
		}

		if len(result) != len(files) {
			t.Fatalf("Expected %v files across bundles but got %v", len(files), len(result))
		}

		// non-bundle mode
		loaded, err = WalkPaths(paths, nil, false)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if len(loaded.FileDescriptors) != len(files) {
			t.Fatalf("Expected %v files across directories but got %v", len(files), len(loaded.FileDescriptors))
		}

		for _, d := range loaded.FileDescriptors {
			path := path.Join(d.Root, d.Path)
			path = strings.TrimPrefix(path, rootDir)
			if _, ok := files[path]; !ok {
				t.Fatalf("unexpected file %v", path)
			}
		}
	})
}

func TestLoadPathsBundleModeWithFilter(t *testing.T) {
	files := map[string]string{
		"a/data.json":      `{"foo": "not-bar"}`,
		"policy.rego":      "package foo\n p = 1",
		"policy_test.rego": "package foo\n test_p { p }",
		"a/.manifest":      `{"roots": ["a", "foo"]}`,
	}

	test.WithTempFS(files, func(rootDir string) {

		paths := []string{rootDir}

		// bundle mode
		loaded, err := LoadPaths(paths, func(abspath string, info os.FileInfo, depth int) bool {
			return loader.GlobExcludeName("*_test.rego", 1)(abspath, info, depth)
		}, true, nil, true, false, nil, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if len(loaded.Bundles) != len(paths) {
			t.Fatalf("Expected %v bundle loaders but got %v", len(paths), len(loaded.Bundles))
		}

		b, ok := loaded.Bundles[rootDir]
		if !ok {
			t.Fatalf("expected bundle %v", rootDir)
		}

		expected := 1
		if len(b.Modules) != expected {
			t.Fatalf("expected %v module but got %v", expected, len(b.Modules))
		}
	})
}
