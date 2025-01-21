// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package compile

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/util/test"
)

func TestCompilerDefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note           string
		module         string
		expRegoVersion ast.RegoVersion
		expErrs        []string
	}{
		{
			note: "v0", // Default rego-version
			module: `
			package test

			p[x] {
				x = "a"
			}`,
			expRegoVersion: ast.RegoV0,
		},
		{
			note: "import rego.v1",
			module: `
			package test
			import rego.v1

			p contains x if {
				x = "a"
			}`,
			expRegoVersion: ast.RegoV0,
		},
		{
			note: "v1", // NOT default rego-version
			module: `
			package test

			p contains x if {
				x = "a"
			}`,
			expRegoVersion: ast.RegoV1,
			expErrs: []string{
				"test.rego:4: rego_parse_error: var cannot be used for rule name",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.module,
			}

			for _, useMemoryFS := range []bool{false, true} {
				test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

					compiler := New().
						WithFS(fsys).
						WithPaths(root)

					err := compiler.Build(context.Background())

					if len(tc.expErrs) > 0 {
						if err == nil {
							t.Fatal("expected error, got none")
						}
						for _, expErr := range tc.expErrs {
							if !strings.Contains(err.Error(), expErr) {
								t.Fatalf("expected error to contain:\n\n%s\n\ngot:\n\n%v", expErr, err)
							}
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}

						// Verify result is just bundle load.
						exp, err := loader.NewFileLoader().WithFS(fsys).AsBundle(root)
						if err != nil {
							panic(err)
						}

						err = exp.FormatModules(false)
						if err != nil {
							t.Fatal(err)
						}

						if !compiler.Bundle().Equal(*exp) {
							t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", compiler.Bundle(), exp)
						}
					}
				})
			}
		})
	}
}

func TestCompilerLoadAsBundleWithBundleRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		files   map[string]string
		expErrs []string
	}{
		{
			note: "No bundle rego version (default version)",
			files: map[string]string{
				".manifest": `{}`,
				"test.rego": `package test
import rego.v1
p[1] if {
	input.x == 2
}`,
			},
		},
		{
			note: "v0 bundle rego version",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"test.rego": `package test
p[1] {
	input.x == 2
}`,
			},
		},
		{
			note: "v0 bundle rego version, missing keyword imports",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"test.rego": `package test
p contains 1 if {
	input.x == 2
}`,
			},
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: number cannot be used for rule name",
			},
		},
		{
			note: "v1 bundle rego version",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"test.rego": `package test
p contains 1 if {
	input.x == 2
}`,
			},
		},
		{
			note: "v1 bundle rego version, no keywords",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"test.rego": `package test
p[1] {
	input.x == 2
}`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1 bundle rego version, duplicate imports",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"test.rego": `package test
import data.foo
import data.foo

p contains 1 if {
	input.x == 2
}`,
			},
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		// file overrides
		{
			note: "v0 bundle rego version, v1 file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"test1.rego": `package test
p["A"] {
	input.x == 1
}`,
				"test2.rego": `package test
p contains "B" if {
	input.x == 2
}`,
			},
		},
		{
			note: "v0 bundle rego version, v1 file override, missing file",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"test1.rego": `package test
p["A"] {
	input.x == 1
}`,
			},
		},
		{
			note: "v0 bundle rego version, v1 file override, no keywords",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"test1.rego": `package test
p["A"] {
	input.x == 1
}`,
				"test2.rego": `package test
p["B"] {
	input.x == 2
}`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v0 bundle rego version, v1 file override, duplicate imports",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"*/test2.rego": 1
	}
}`,
				"test1.rego": `package test
p["A"] {
	input.x == 1
}`,
				"test2.rego": `package test
import data.foo
import data.foo

p contains "B" if {
	input.x == 2
}`,
			},
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note: "v1 bundle rego version, v0 file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"*/test1.rego": 0
	}
}`,
				"test1.rego": `package test
p["A"] {
	input.x == 1
}`,
				"test2.rego": `package test
p contains "B" if {
	input.x == 2
}`,
			},
		},
		{
			note: "v1 bundle rego version, v0 file override, no import",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"*/test1.rego": 0
	}
}`,
				"test1.rego": `package test
p contains "A" if {
	input.x == 1
}`,
				"test2.rego": `package test
p contains "B" if {
	input.x == 2
}`,
			},
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
				"rego_parse_error: string cannot be used for rule name",
			},
		},
	}

	bundleTypeCases := []struct {
		note string
		tar  bool
	}{
		{
			"bundle dir", false,
		},
		{
			"bundle tar", true,
		},
	}

	for _, bundleType := range bundleTypeCases {
		for _, tc := range tests {
			ctx := context.Background()
			t.Run(fmt.Sprintf("%s, %s", bundleType.note, tc.note), func(t *testing.T) {
				files := map[string]string{}
				if bundleType.tar {
					files["bundle.tar"] = ""
				} else {
					for k, v := range tc.files {
						files[k] = v
					}
				}

				test.WithTestFS(tc.files, false, func(root string, fsys fs.FS) {
					var path string
					if bundleType.tar {
						path = filepath.Join(root, "bundle.tar.gz")
						files := make([][2]string, 0, len(tc.files))
						for k, v := range tc.files {
							files = append(files, [2]string{k, v})
						}
						buf := archive.MustWriteTarGz(files)

						bf, err := os.Create(path)
						if err != nil {
							t.Fatalf("Unexpected error: %v", err)
						}

						_, err = bf.Write(buf.Bytes())
						if err != nil {
							t.Fatalf("Unexpected error: %v", err)
						}
					} else {
						path = root
					}

					compiler := New().
						WithFS(fsys).
						WithPaths(path).
						WithAsBundle(true)

					err := compiler.Build(ctx)

					if len(tc.expErrs) > 0 {
						if err == nil {
							t.Fatal("expected error, got none")
						}
						for _, expErr := range tc.expErrs {
							if !strings.Contains(err.Error(), expErr) {
								t.Fatalf("expected error to contain:\n\n%s\n\ngot:\n\n%v", expErr, err)
							}
						}
					} else {
						if err != nil {
							t.Fatal(err)
						}
					}
				})
			})
		}
	}
}

func TestCompilerBundleMergeWithBundleRegoVersion(t *testing.T) {
	regoV0 := ast.RegoV0.Int()
	regoV1 := ast.RegoV1.Int()
	regoDef := ast.RegoV0.Int()

	tests := []struct {
		note                 string
		bundles              []*bundle.Bundle
		regoVersion          ast.RegoVersion
		expGlobalRegoVersion *int
		expFileRegoVersions  map[string]int
	}{
		{
			note: "single bundle, no bundle rego version (default version)",
			bundles: []*bundle.Bundle{
				{
					Manifest: bundle.Manifest{
						Roots: &[]string{"a"},
					},
					Data:    map[string]interface{}{},
					Modules: []bundle.ModuleFile{},
				},
			},
			expGlobalRegoVersion: &regoDef,
			expFileRegoVersions:  map[string]int{},
		},
		{
			note: "single bundle, global rego version",
			bundles: []*bundle.Bundle{
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"a"},
						RegoVersion: &regoV1,
					},
					Data:    map[string]interface{}{},
					Modules: []bundle.ModuleFile{},
				},
			},
			expGlobalRegoVersion: &regoV1,
			expFileRegoVersions:  map[string]int{},
		},
		{
			note: "no global rego versions",
			bundles: []*bundle.Bundle{
				{
					Manifest: bundle.Manifest{
						Roots: &[]string{"a"},
					},
					Data:    map[string]interface{}{},
					Modules: []bundle.ModuleFile{},
				},
				{
					Manifest: bundle.Manifest{
						Roots: &[]string{"b"},
					},
					Data:    map[string]interface{}{},
					Modules: []bundle.ModuleFile{},
				},
			},
			regoVersion:          ast.RegoV1,
			expGlobalRegoVersion: &regoV1,
		},
		{
			note: "global rego versions, v1 bundles, v0 provided",
			bundles: []*bundle.Bundle{
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"a"},
						RegoVersion: &regoV1,
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							Path:         "a/test1.rego",
							URL:          "a/test1.rego",
							RelativePath: "/test1.rego",
							Raw:          []byte("package a"),
						},
					},
				},
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"b"},
						RegoVersion: &regoV1,
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							Path:         "b/test1.rego",
							URL:          "b/test1.rego",
							RelativePath: "/test1.rego",
							Raw:          []byte("package b"),
						},
					},
				},
			},
			regoVersion: ast.RegoV0,
			// global rego-version in bundles are dropped in favor of the provided rego-version
			expGlobalRegoVersion: &regoV0,
			expFileRegoVersions: map[string]int{
				"/a/test1.rego": 1,
				"/b/test1.rego": 1,
			},
		},
		{
			note: "global rego versions, v0 bundles, v1 provided",
			bundles: []*bundle.Bundle{
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"a"},
						RegoVersion: &regoV0,
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							Path:         "a/test1.rego",
							URL:          "a/test1.rego",
							RelativePath: "/test1.rego",
							Raw:          []byte("package a"),
						},
					},
				},
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"b"},
						RegoVersion: &regoV0,
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							Path:         "b/test1.rego",
							URL:          "b/test1.rego",
							RelativePath: "/test1.rego",
							Raw:          []byte("package b"),
						},
					},
				},
			},
			regoVersion: ast.RegoV1,
			// global rego-version in bundles are dropped in favor of the provided rego-version
			expGlobalRegoVersion: &regoV1,
			expFileRegoVersions: map[string]int{
				"/a/test1.rego": 0,
				"/b/test1.rego": 0,
			},
		},
		{
			note: "different global rego versions",
			bundles: []*bundle.Bundle{
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"a"},
						RegoVersion: &regoV0,
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							Path:         "a/test1.rego",
							URL:          "a/test1.rego",
							RelativePath: "/test1.rego",
							Raw:          []byte("package a"),
						},
					},
				},
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"b"},
						RegoVersion: &regoV1,
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							Path:         "b/test1.rego",
							URL:          "b/test1.rego",
							RelativePath: "/test1.rego",
							Raw:          []byte("package b"),
						},
					},
				},
			},
			regoVersion: ast.RegoV0,
			// global rego-version in bundles are dropped in favor of the provided rego-version
			expGlobalRegoVersion: &regoV0,
			expFileRegoVersions: map[string]int{
				"/b/test1.rego": 1,
			},
		},
		{
			note: "different global rego versions, per-file overrides",
			bundles: []*bundle.Bundle{
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"a"},
						RegoVersion: &regoV1,
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							Path:         "a/test1.rego",
							URL:          "a/test1.rego",
							RelativePath: "/test1.rego",
							Raw:          []byte("package a"),
						},
						{
							Path:         "a/test2.rego",
							URL:          "a/test2.rego",
							RelativePath: "/test2.rego",
							Raw:          []byte("package a"),
						},
					},
				},
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"b"},
						RegoVersion: &regoV1,
						FileRegoVersions: map[string]int{
							"/test1.rego": 0,
						},
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							// we don't expect this file to get an individual rego-version in the result, as
							// it has the same rego-version as the global rego-version
							Path:         "b/test1.rego",
							URL:          "b/test1.rego",
							RelativePath: "/test1.rego",
							Raw:          []byte("package b"),
						},
						{
							Path:         "b/test2.rego",
							URL:          "b/test2.rego",
							RelativePath: "/test2.rego",
							Raw:          []byte("package b"),
						},
					},
				},
				{
					Manifest: bundle.Manifest{
						RegoVersion: &regoV0,
						Roots:       &[]string{"c"},
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						// we don't expect these files to get individual rego-versions in the result,
						// as they have the same rego-version as the global rego-version
						{
							Path:         "c/test1.rego",
							URL:          "c/test1.rego",
							RelativePath: "test1.rego",
							Raw:          []byte("package c"),
						},
						{
							Path:         "c/test2.rego",
							URL:          "c/test2.rego",
							RelativePath: "test2.rego",
							Raw:          []byte("package c"),
						},
					},
				},
			},
			regoVersion: ast.RegoV0,
			// global rego-version in bundles are dropped in favor of the provided rego-version
			expGlobalRegoVersion: &regoV0,
			// rego-versions is expected for all modules with different rego-version than the global rego-version
			expFileRegoVersions: map[string]int{
				"/a/test1.rego": 1,
				"/a/test2.rego": 1,
				"/b/test2.rego": 1,
			},
		},
		{
			note: "glob per-file overrides",
			bundles: []*bundle.Bundle{
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"a"},
						RegoVersion: &regoV0,
						FileRegoVersions: map[string]int{
							"a/*": 1,
						},
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							Path: "a/foo/test.rego",
							URL:  "a/foo/test.rego",
							Raw:  []byte("package a"),
						},
						{
							Path: "a/bar/test.rego",
							URL:  "a/bar/test.rego",
							Raw:  []byte("package a"),
						},
						{
							Path: "a/baz/test.rego",
							URL:  "a/baz/test.rego",
							Raw:  []byte("package a"),
						},
					},
				},
				{
					Manifest: bundle.Manifest{
						Roots:       &[]string{"b"},
						RegoVersion: &regoV1,
						FileRegoVersions: map[string]int{
							// glob should not affect files with matching path in the other bundle
							"*/bar/*": 0,
						},
					},
					Data: map[string]interface{}{},
					Modules: []bundle.ModuleFile{
						{
							Path: "b/foo/test.rego",
							URL:  "b/foo/test.rego",
							Raw:  []byte("package b"),
						},
						{
							Path: "b/bar/test.rego",
							URL:  "b/bar/test.rego",
							Raw:  []byte("package b"),
						},
						{
							Path: "b/baz/test.rego",
							URL:  "b/baz/test.rego",
							Raw:  []byte("package b"),
						},
					},
				},
			},
			regoVersion:          ast.RegoV0,
			expGlobalRegoVersion: &regoV0,
			expFileRegoVersions: map[string]int{
				"/a/foo/test.rego": 1,
				"/a/bar/test.rego": 1,
				"/a/baz/test.rego": 1,
				"/b/foo/test.rego": 1,
				"/b/baz/test.rego": 1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			for _, b := range tc.bundles {
				b.Manifest.Init()
				for i, m := range b.Modules {
					b.Modules[i].Parsed = ast.MustParseModule(string(m.Raw))
				}
			}

			result, err := bundle.MergeWithRegoVersion(tc.bundles, tc.regoVersion, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			compareRegoVersions(t, tc.expGlobalRegoVersion, result.Manifest.RegoVersion)

			if !maps.Equal(tc.expFileRegoVersions, result.Manifest.FileRegoVersions) {
				t.Fatalf("expected file rego versions to be:\n\n%v\n\nbut got:\n\n%v", tc.expFileRegoVersions, result.Manifest.FileRegoVersions)
			}
		})
	}
}

func compareRegoVersions(t *testing.T, exp, act *int) {
	t.Helper()
	if exp == nil {
		if act != nil {
			t.Errorf("expected no rego version, but got %v", *act)
		}
	} else {
		if act == nil {
			t.Errorf("expected rego version to be %v, but got none", *exp)
		} else if *act != *exp {
			t.Errorf("expected rego version to be %v, but got %v", *exp, *act)
		}
	}
}
