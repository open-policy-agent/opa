package compile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/internal/ref"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/format"
	"github.com/open-policy-agent/opa/v1/ir"
	"github.com/open-policy-agent/opa/v1/loader"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestCompilerV1Module(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test

			p contains x if {
				x = "a"
			}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root)

			err := compiler.Build(t.Context())
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
		})
	}
}

func TestOrderedStringSet(t *testing.T) {
	var ss orderedStringSet
	result := ss.Append("a", "b", "b", "a", "e", "c", "e")
	if !slices.Equal(result, orderedStringSet{"a", "b", "e", "c"}) {
		t.Fatal(result)
	}
}

func TestCompilerInitErrors(t *testing.T) {

	ctx := t.Context()

	tests := []struct {
		note string
		c    *Compiler
		want error
	}{
		{
			note: "bad target",
			c:    New().WithTarget("deadbeef"),
			want: errors.New("invalid target \"deadbeef\""),
		},
		{
			note: "optimizations require entrypoint",
			c:    New().WithOptimizationLevel(1),
			want: errors.New("bundle optimizations require at least one entrypoint"),
		},
		{
			note: "wasm compilation requires at least one entrypoint",
			c:    New().WithTarget("wasm"),
			want: errors.New("wasm compilation requires at least one entrypoint"),
		},
		{
			note: "plan compilation requires at least one entrypoint",
			c:    New().WithTarget("plan"),
			want: errors.New("plan compilation requires at least one entrypoint"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			err := tc.c.Build(ctx)
			if err == nil {
				t.Fatal("expected error")
			} else if err.Error() != tc.want.Error() {
				t.Fatalf("expected %v but got %v", tc.want, err)
			}
		})
	}

}

func TestCompilerLoadError(t *testing.T) {

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(nil, useMemoryFS, func(root string, fsys fs.FS) {

			err := New().
				WithFS(fsys).
				WithPaths(path.Join(root, "does-not-exist")).
				Build(t.Context())
			if err == nil {
				t.Fatal("expected failure")
			}
		})
	}
}

func TestCompilerLoadAsBundleSuccess(t *testing.T) {

	ctx := t.Context()
	rv := strconv.Itoa(ast.DefaultRegoVersion.Int())

	files := map[string]string{
		"b1/.manifest": `{"roots": ["b1"], "rego_version": ` + rv + `}`,
		"b1/test.rego": `
			package b1.test
			import rego.v1

			p = 1`,
		"b1/data.json": `
			{"b1": {"k": "v"}}`,
		"b2/.manifest": `{"roots": ["b2"], "rego_version": ` + rv + `}`,
		"b2/data.json": `
			{"b2": {"k2": "v2"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			root1 := path.Join(root, "b1")
			root2 := path.Join(root, "b2")

			compiler := New().
				WithFS(fsys).
				WithPaths(root1, root2).
				WithAsBundle(true)

			err := compiler.Build(ctx)
			if err != nil {
				t.Fatal(err)
			}

			// Verify result is just merger of two bundles.
			a, err := loader.NewFileLoader().WithFS(fsys).AsBundle(root1)
			if err != nil {
				panic(err)
			}

			b, err := loader.NewFileLoader().WithFS(fsys).AsBundle(root2)
			if err != nil {
				panic(err)
			}

			exp, err := bundle.Merge([]*bundle.Bundle{a, b})
			if err != nil {
				panic(err)
			}

			err = exp.FormatModules(false)
			if err != nil {
				t.Fatal(err)
			}

			if !compiler.bundle.Equal(*exp) {
				t.Fatalf("expected:\n\n%v\n\nbut got:\n\n%v", exp, compiler.bundle)
			}

			expRoots := []string{"b1", "b2"}
			expManifest := bundle.Manifest{
				Roots: &expRoots,
			}
			expManifest.SetRegoVersion(ast.DefaultRegoVersion)

			if !compiler.bundle.Manifest.Equal(expManifest) {
				t.Fatalf("expected %v but got %v", compiler.bundle.Manifest, expManifest)
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
			ctx := t.Context()
			t.Run(fmt.Sprintf("%s, %s", bundleType.note, tc.note), func(t *testing.T) {
				files := map[string]string{}
				if bundleType.tar {
					files["bundle.tar"] = ""
				} else {
					maps.Copy(files, tc.files)
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
					} else if err != nil {
						t.Fatal(err)
					}
				})
			})
		}
	}
}

func TestCompilerBundleMergeWithBundleRegoVersion(t *testing.T) {
	regoV0 := ast.RegoV0.Int()
	regoV1 := ast.RegoV1.Int()
	regoDef := ast.RegoV1.Int()

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
					Data:    map[string]any{},
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
					Data:    map[string]any{},
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
					Data:    map[string]any{},
					Modules: []bundle.ModuleFile{},
				},
				{
					Manifest: bundle.Manifest{
						Roots: &[]string{"b"},
					},
					Data:    map[string]any{},
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
					Data: map[string]any{},
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
					Data: map[string]any{},
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
					Data: map[string]any{},
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
					Data: map[string]any{},
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
					Data: map[string]any{},
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
					Data: map[string]any{},
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
					Data: map[string]any{},
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
					Data: map[string]any{},
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
					Data: map[string]any{},
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
					Data: map[string]any{},
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
					Data: map[string]any{},
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

func TestCompilerLoadAsBundleMergeError(t *testing.T) {

	ctx := t.Context()

	// Omit manifests (defaulting to '') to trigger a merge error
	files := map[string]string{
		"b1/test.rego": `
			package b1.test

			p = 1`,
		"b1/data.json": `
			{"b1": {"k": "v"}}`,
		"b2/data.json": `
			{"b2": {"k2": "v2"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			root1 := path.Join(root, "b1")
			root2 := path.Join(root, "b2")

			compiler := New().
				WithFS(fsys).
				WithPaths(root1, root2).
				WithAsBundle(true)

			err := compiler.Build(ctx)
			if err == nil || err.Error() != "bundle merge failed: manifest has overlapped roots: '' and ''" {
				t.Fatal(err)
			}
		})
	}
}

func TestCompilerLoadFilesystem(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package b1.test

			p = 1`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root)

			err := compiler.Build(t.Context())
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

			if !compiler.bundle.Equal(*exp) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", compiler.bundle, exp)
			}
		})
	}
}

func TestCompilerLoadFilesystemWithEnablePrintStatementsFalse(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package test
			import rego.v1

			allow if { print(1) }
		`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").WithEntrypoints("test/allow").
				WithEnablePrintStatements(false)

			if err := compiler.Build(t.Context()); err != nil {
				t.Fatal(err)
			}

			bundle := compiler.Bundle()

			if strings.Contains(string(bundle.PlanModules[0].Raw), "internal.print") {
				t.Fatalf("output different than expected:\n\ngot: %v\n\nfound: internal.print", string(bundle.PlanModules[0].Raw))
			}
		})
	}
}

func TestCompilerLoadFilesystemWithEnablePrintStatementsTrue(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package test
			import rego.v1

			allow if { print(1) }
		`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").
				WithEntrypoints("test/allow").
				WithEnablePrintStatements(true)

			if err := compiler.Build(t.Context()); err != nil {
				t.Fatal(err)
			}

			bundle := compiler.Bundle()

			if !strings.Contains(string(bundle.PlanModules[0].Raw), "internal.print") {
				t.Fatalf("output different than expected:\n\ngot: %v\n\nmissing: internal.print", string(bundle.PlanModules[0].Raw))
			}
		})
	}
}

func TestCompilerLoadHonorsFilter(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package b1.test

			p = 1`,
		"data.json": `
			{"b1": {"k": "v"}}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithFilter(func(abspath string, _ os.FileInfo, _ int) bool {
					return strings.HasSuffix(abspath, ".json")
				})

			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.Data) > 0 {
				t.Fatal("expected no data to be loaded")
			}
		})
	}
}

func TestCompilerInputBundle(t *testing.T) {

	b := &bundle.Bundle{
		Modules: []bundle.ModuleFile{
			{
				URL:    "/foo.rego",
				Path:   "/foo.rego",
				Raw:    []byte("package test\np := 7"),
				Parsed: ast.MustParseModule("package test\np = 7"),
			},
		},
	}

	compiler := New().WithBundle(b)

	if err := compiler.Build(t.Context()); err != nil {
		t.Fatal(err)
	}

	exp := "package test\n\np := 7\n"

	if exp != string(compiler.Bundle().Modules[0].Raw) {
		t.Fatalf("expected module to have been formatted (output different than expected):\n\ngot: %v\n\nwant: %v", string(compiler.Bundle().Modules[0].Raw), exp)
	}
}

func TestCompilerInputInvalidBundle(t *testing.T) {

	b := &bundle.Bundle{
		Modules: []bundle.ModuleFile{
			{
				URL:    "/url",
				Path:   "/foo.rego",
				Raw:    []byte("package test\np = 0"),
				Parsed: ast.MustParseModule("package test\np = 0"),
			},
			{
				URL:    "/url",
				Path:   "/bar.rego",
				Raw:    []byte("package test\nq = 1"),
				Parsed: ast.MustParseModule("package test\nq = 1"),
			},
		},
	}

	compiler := New().WithBundle(b)

	if err := compiler.Build(t.Context()); err == nil {
		t.Fatal("duplicate module URL not detected")
	} else if err.Error() != "duplicate module URL: /url" {
		t.Fatal(err)
	}
}

func TestCompilerError(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package test
			import rego.v1
			default p := false
			p if { p }`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root)

			err := compiler.Build(t.Context())
			if err == nil {
				t.Fatal("expected error")
			}

			astErr, ok := err.(ast.Errors)
			if !ok || len(astErr) != 1 || astErr[0].Code != ast.RecursionErr {
				t.Fatal("unexpected error:", err)
			}
		})
	}
}

func TestCompilerOptimizationL1(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test

			default p := false
			p if { q }
			q if { input.x = data.foo }`,
		"data.json": `
			{"foo": 1}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithRegoVersion(ast.RegoV1).
				WithFS(fsys).
				WithPaths(root).
				WithOptimizationLevel(1).
				WithEntrypoints("test/p")

			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			optimizedExp := ast.MustParseModuleWithOpts(`
			package test

			default p = false
			p if { data.test.q = X; X }
			q if { input.x = 1 }
		`, ast.ParserOptions{RegoVersion: ast.RegoV1})

			// NOTE(tsandall): PE generates vars with wildcard prefix. Instead of
			// constructing the AST manually, just rewrite to the expected value
			// here. If this becomes a common pattern, we could refactor (e.g.,
			// allow caller to control var prefix, split into a reusable function,
			// etc.)
			_, err = ast.TransformVars(optimizedExp, func(x ast.Var) (ast.Value, error) {
				if x == ast.Var("X") {
					return ast.Var("$_term_1_01"), nil
				}
				return x, nil
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.Modules) != 1 {
				t.Fatalf("expected 1 module but got: %v", compiler.bundle.Modules)
			}

			if !compiler.bundle.Modules[0].Parsed.Equal(optimizedExp) {
				t.Fatalf("expected optimized module to be:\n\n%v\n\ngot:\n\n%v", optimizedExp, compiler.bundle.Modules[0])
			}
		})
	}
}

func TestCompilerOptimizationL2(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			default p := false
			p if { q }
			q if { input.x = data.foo }`,
		"data.json": `
			{"foo": 1}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithRegoVersion(ast.RegoV1).
				WithFS(fsys).
				WithPaths(root).
				WithOptimizationLevel(2).
				WithEntrypoints("test/p")

			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			prunedExp := ast.MustParseModuleWithOpts(`
				package test
				q if { input.x = data.foo }`,
				ast.ParserOptions{RegoVersion: ast.RegoV1})

			optimizedExp := ast.MustParseModuleWithOpts(`
				package test
				default p = false
				p if { input.x = 1 }`,
				ast.ParserOptions{RegoVersion: ast.RegoV1})

			if len(compiler.bundle.Modules) != 2 {
				t.Fatalf("expected two modules but got: %v", compiler.bundle.Modules)
			}

			// Note: L2 optimized ModuleFile ordering in a bundle is non-deterministic...
			if !compiler.bundle.Modules[0].Parsed.Equal(prunedExp) {
				if !compiler.bundle.Modules[0].Parsed.Equal(optimizedExp) {
					t.Fatalf("expected optimized module to be:\n\n%v\n\ngot:\n\n%v", optimizedExp, compiler.bundle.Modules[0])
				}
				if !compiler.bundle.Modules[1].Parsed.Equal(prunedExp) {
					t.Fatalf("expected pruned module to be:\n\n%v\n\ngot:\n\n%v", prunedExp, compiler.bundle.Modules[1])
				}
			} else if !compiler.bundle.Modules[1].Parsed.Equal(optimizedExp) {
				t.Fatalf("expected optimized module to be:\n\n%v\n\ngot:\n\n%v", optimizedExp, compiler.bundle.Modules[1])
			}
		})
	}
}

func TestCompilerOptimizationWithConfiguredNamespace(t *testing.T) {

	files := map[string]string{
		"test.rego": `
			package test
			import rego.v1

			p if { not q }
			q if { k[input.a]; k[input.b] }  # generate a product that is not inlined
			k = {1,2,3}
		`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithOptimizationLevel(1).
				WithEntrypoints("test/p").
				WithPartialNamespace("custom")

			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.Modules) != 2 {
				t.Fatalf("expected two modules but got: %v", len(compiler.bundle.Modules))
			}

			// The compiler will drop the rego.v1 import, so we need to affix the rego-version of the expected module to
			// v1, to have its string serialization include 'if'/'else' keywords but not the rego.v1 import.
			optimizedExp := ast.MustParseModuleWithOpts(`package custom
				__not1_0_2__ = true if { data.test.q = _; _ }`,
				ast.ParserOptions{RegoVersion: ast.RegoV1})

			if optimizedExp.String() != compiler.bundle.Modules[0].Parsed.String() {
				t.Fatalf("expected optimized module to be:\n\n%v\n\ngot:\n\n%v", optimizedExp, compiler.bundle.Modules[0])
			}

			expected := ast.MustParseModuleWithOpts(`package test
				k = {1, 2, 3} if { true }
				p = true if { not data.custom.__not1_0_2__ }
				q = true if { __local0__3 = input.a; data.test.k[__local0__3] = _; _; __local1__3 = input.b; data.test.k[__local1__3] = _; _ }`,
				ast.ParserOptions{RegoVersion: ast.RegoV1})

			if expected.String() != compiler.bundle.Modules[1].Parsed.String() {
				t.Fatalf("expected module to be:\n\n%v\n\ngot:\n\n%v", expected, compiler.bundle.Modules[1])
			}
		})
	}
}

func TestCompilerOptimizationWithGeneralRefs(t *testing.T) {
	tests := []struct {
		note       string
		entrypoint string
		files      map[string]string
		expected   []string
	}{
		{
			note: "special characters in ref term",
			files: map[string]string{
				"base.rego": `package base
allow["entity/slash"].action {
	action := "action"
	input.principal == input.entity
}`,
				"query.rego": `package query
main {
	data.base.allow[input.entity.type][input.action]
}`,
			},
			entrypoint: "query/main",
			expected: []string{
				`package base

allow["entity/slash"].action {
	input.principal = input.entity
}
`,
				`package query

main {
	__local1__1 = input.entity.type
	__local2__1 = input.action
	"entity/slash" = __local1__1
	"action" = __local2__1
	data.base.allow[__local1__1][__local2__1] = _term_1_21
	_term_1_21
}
`,
			},
		},
		{
			note:       "single rule (one key), no ref, no unknowns",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[r] {
	r := ["do", "re"][_]
}`,
			},
			expected: []string{
				`package test

p = __result__ {
	__result__ = {"do", "re"}
}
`,
			},
		},
		{
			note:       "single rule (one key), no ref, unknown in body",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[r] {
	r := ["do", "re"][_]
	input.x
}`,
			},
			expected: []string{
				`package test

p["do"] {
	input.x = _term_1_21
	_term_1_21
}

p["re"] {
	input.x = _term_1_21
	_term_1_21
}
`,
			},
		},
		{
			note:       "single rule (one key), no unknowns",
			entrypoint: "test/p/q",
			files: map[string]string{
				"test.rego": `package test
p.q[r] {
	r := ["do", "re"][_]
	input.x
}`,
			},
			expected: []string{
				`package test.p.q

do {
	input.x = _term_1_21
	_term_1_21
}

re {
	input.x = _term_1_21
	_term_1_21
}
`,
			},
		},
		{
			note:       "single rule, no unknowns",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[q][r] {
	q := ["foo", "bar"][_]
	r := ["do", "re"][_]
}`,
			},
			expected: []string{
				`package test

p = __result__ {
	__result__ = {"foo": {"do": true, "re": true}, "bar": {"do": true, "re": true}}
}
`,
			},
		},
		{
			note:       "single rule, unknown value",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[q][r] := s {
	q := ["foo", "bar"][_]
	r := ["do", "re"][_]
	s := input.x
}`,
			},
			expected: []string{
				`package test.p.foo

do = __local2__1 {
	__local2__1 = input.x
}

re = __local2__1 {
	__local2__1 = input.x
}
`,
				`package test.p.bar

do = __local2__1 {
	__local2__1 = input.x
}

re = __local2__1 {
	__local2__1 = input.x
}
`,
			},
		},
		{
			note:       "single rule, unknown key (first)",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[q][r] {
	q := input.x[_]
	r := ["do", "re"][_]
}`,
			},
			expected: []string{
				`package test

p[__local0__1].do {
	__local0__1 = input.x[_01]
}

p[__local0__1].re {
	__local0__1 = input.x[_01]
}
`,
			},
		},
		{
			note:       "single rule, unknown key (second)",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test
p[q][r] {
	q := ["foo", "bar"][_]
	r := input.x[_]
}`,
			},
			expected: []string{
				`package test.p

bar[__local1__1] = true {
	__local1__1 = input.x[_11]
}

foo[__local1__1] = true {
	__local1__1 = input.x[_11]
}
`,
			},
		},
		{
			note:       "regression test for #6338",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test

p {
	q[input.x][input.y]
}

q["foo/bar"][x] {
	x := "baz"
	input.x == 1
}`,
			},
			expected: []string{
				`package test

p {
	__local1__1 = input.x
	__local2__1 = input.y
	"foo/bar" = __local1__1
	data.test.q[__local1__1][__local2__1] = _term_1_21
	_term_1_21
}

q["foo/bar"].baz {
	input.x = 1
}
`,
			},
		},
		{
			note:       "regression test for #6339",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test

import future.keywords.in

p {
	q[input.x][input.y]
}

q[entity][action] {
	some action in ["show", "update"]
	some entity in ["pay", "roll"]
	input.z == 1
}`,
			},
			expected: []string{
				`package test

p {
	__local8__1 = input.x
	__local9__1 = input.y
	data.test.q[__local8__1][__local9__1] = _term_1_21
	_term_1_21
}
`,
				`package test.q.pay

show {
	input.z = 1
}

update {
	input.z = 1
}
`,
				`package test.q.roll

show {
	input.z = 1
}

update {
	input.z = 1
}
`,
			},
		},
		{
			note:       "not",
			entrypoint: "test/p",
			files: map[string]string{
				"test.rego": `package test

import future.keywords.in

p {
	not q[input.x][input.y]
}

q[entity][action] {
	some action in ["show", "update"]
	some entity in ["pay", "roll"]
	input.z == 1
}`,
			},
			expected: []string{
				`package partial

__not1_2_2__(__local8__1, __local9__1) {
	data.test.q[__local8__1][__local9__1] = _term_2_01
	_term_2_01
}
`,
				`package test

p {
	__local8__1 = input.x
	__local9__1 = input.y
	not data.partial.__not1_2_2__(__local8__1, __local9__1)
}
`,
				`package test.q.pay

show {
	input.z = 1
}

update {
	input.z = 1
}
`,
				`package test.q.roll

show {
	input.z = 1
}

update {
	input.z = 1
}
`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			for _, useMemoryFS := range []bool{false, true} {
				test.WithTestFS(tc.files, useMemoryFS, func(root string, fsys fs.FS) {

					caps := ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0))
					caps.Features = []string{
						ast.FeatureRefHeadStringPrefixes,
						ast.FeatureRefHeads,
					}

					compiler := New().
						// In v1, the rego.v1 import is stripped from optimized modules, but in v0 it is not.
						// Therefore, we need to tie down the test modules to a specific version.
						WithRegoVersion(ast.RegoV0).
						WithFS(fsys).
						WithPaths(root).
						WithOptimizationLevel(1).
						WithEntrypoints(tc.entrypoint).
						WithCapabilities(caps)

					err := compiler.Build(t.Context())
					if err != nil {
						t.Fatal(err)
					}

					if len(compiler.bundle.Modules) != len(tc.expected) {
						t.Fatalf("expected %v modules but got: %v:\n\n%v",
							len(tc.expected), len(compiler.bundle.Modules), modulesToString(compiler.bundle.Modules))
					}

					actual := make(map[string]struct{})
					for _, m := range compiler.bundle.Modules {
						actual[string(m.Raw)] = struct{}{}
					}

					for _, e := range tc.expected {
						if _, ok := actual[e]; !ok {
							t.Fatalf("expected to find module:\n\n%v\n\nin bundle but got:\n\n%v",
								e, modulesToString(compiler.bundle.Modules))
						}
					}
				})
			}
		})
	}
}

func TestCompilerOptimizationSupportRegoVersion(t *testing.T) {
	tests := []struct {
		note                       string
		modulesRegoVersion         ast.RegoVersion
		noKeywordsInRefsCapability bool
		regoV1ImportCapable        bool
		entrypoint                 string
		files                      map[string]string
		expected                   []string
	}{
		{
			note:                "v0 module, rego.v1 capable",
			modulesRegoVersion:  ast.RegoV0,
			regoV1ImportCapable: true,
			entrypoint:          "test/p",
			files: map[string]string{
				"test.rego": `package test
p {
    input.x == 1
}`,
			},
			expected: []string{
				`package test

import rego.v1

p if {
	input.x = 1
}
`,
			},
		},
		{
			note:                "v0 module, rego.v1 capable, rule name conflict with keyword",
			modulesRegoVersion:  ast.RegoV0,
			regoV1ImportCapable: true,
			entrypoint:          "test/contains",
			files: map[string]string{
				"test.rego": `package test
contains {
    input.x == 1
}`,
			},
			// rego.v1 import not used, since rule name conflicts with future keyword
			expected: []string{
				`package test

contains {
	input.x = 1
}
`,
			},
		},
		{
			note:                "v0 module, rego.v1 capable, import conflict with keyword",
			modulesRegoVersion:  ast.RegoV0,
			regoV1ImportCapable: true,
			entrypoint:          "test/p",
			files: map[string]string{
				"test.rego": `package test
import data.foo.contains

p {
    input.x == contains
}`,
				"foo.rego": `package foo
contains := 2 {
	input.a == input.b
}`,
			},
			// rego.v1 import used for data.test, since complete ref to data.foo.contains is used locally without original import
			expected: []string{
				`package test

import rego.v1

p if {
	data.foo.contains = input.x
}
`,
				`package foo

contains = 2 {
	input.a = input.b
}
`,
			},
		},
		{
			note:                       "v0 module, rego.v1 capable, import conflict with keyword, no capability",
			modulesRegoVersion:         ast.RegoV0,
			noKeywordsInRefsCapability: true,
			regoV1ImportCapable:        true,
			entrypoint:                 "test/p",
			files: map[string]string{
				"test.rego": `package test
import data.foo.contains

p {
    input.x == contains
}`,
				"foo.rego": `package foo
contains := 2 {
	input.a == input.b
}`,
			},
			// rego.v1 import used for data.test, since complete ref to data.foo.contains is used locally without original import
			expected: []string{
				`package test

import rego.v1

p if {
	data.foo["contains"] = input.x
}
`,
				`package foo

contains = 2 {
	input.a = input.b
}
`,
			},
		},
		{
			note:                "v0 module, rego.v1 capable, rule ref conflict with keyword",
			modulesRegoVersion:  ast.RegoV0,
			regoV1ImportCapable: true,
			entrypoint:          "test/contains",
			files: map[string]string{
				"test.rego": `package test
contains[input.x][input.y] {
    input.z == 1
}`,
			},
			// rego.v1 import not used, since leading var in rule ref conflicts with future keyword
			expected: []string{
				`package test

contains[__local0__1][__local1__1] {
	input.z = 1
	__local0__1 = input.x
	__local1__1 = input.y
}
`,
			},
		},
		{
			note:                "v0 module, not rego.v1 capable",
			modulesRegoVersion:  ast.RegoV0,
			regoV1ImportCapable: false,
			entrypoint:          "test/p",
			files: map[string]string{
				"test.rego": `package test
p {
    input.x == 1
}`,
			},
			expected: []string{
				`package test

p {
	input.x = 1
}
`,
			},
		},
		{
			note:                "v0-compat_v1 module, rego.v1 capable",
			modulesRegoVersion:  ast.RegoV0CompatV1,
			regoV1ImportCapable: true,
			entrypoint:          "test/p",
			files: map[string]string{
				"test.rego": `package test

import rego.v1

p if {
    input.x == 1
}`,
			},
			expected: []string{
				`package test

import rego.v1

p if {
	input.x = 1
}
`,
			},
		},
		{
			note:                "v1 module, rego.v1 capable",
			modulesRegoVersion:  ast.RegoV1,
			regoV1ImportCapable: true,
			entrypoint:          "test/p",
			files: map[string]string{
				"test.rego": `package test
p if {
    input.x == 1
}`,
			},
			expected: []string{
				`package test

p if {
	input.x = 1
}
`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTestFS(tc.files, true, func(root string, fsys fs.FS) {
				capabilities := ast.CapabilitiesForThisVersion()
				capabilities.Features = []string{
					ast.FeatureRefHeadStringPrefixes,
					ast.FeatureRefHeads,
				}
				if tc.modulesRegoVersion == ast.RegoV1 {
					capabilities.Features = append(capabilities.Features, ast.FeatureRegoV1)
				}
				if tc.regoV1ImportCapable {
					capabilities.Features = append(capabilities.Features, ast.FeatureRegoV1Import)
				}
				if !tc.noKeywordsInRefsCapability {
					capabilities.Features = append(capabilities.Features, ast.FeatureKeywordsInRefs)
				}

				compiler := New().
					WithCapabilities(capabilities).
					WithRegoVersion(tc.modulesRegoVersion).
					WithFS(fsys).
					WithPaths(root).
					WithOptimizationLevel(1).
					WithEntrypoints(tc.entrypoint)

				err := compiler.Build(t.Context())
				if err != nil {
					t.Fatal(err)
				}

				if len(compiler.bundle.Modules) != len(tc.expected) {
					t.Fatalf("expected %v modules but got: %v:\n\n%v",
						len(tc.expected), len(compiler.bundle.Modules), modulesToString(compiler.bundle.Modules))
				}

				actual := make(map[string]struct{})
				for _, m := range compiler.bundle.Modules {
					actual[string(m.Raw)] = struct{}{}
				}

				for _, e := range tc.expected {
					if _, ok := actual[e]; !ok {
						t.Fatalf("expected to find module:\n\n%v\n\nin bundle but got:\n\n%v",
							e, modulesToString(compiler.bundle.Modules))
					}
				}
			})
		})
	}
}

func modulesToString(modules []bundle.ModuleFile) string {
	var buf bytes.Buffer
	for i, m := range modules {
		buf.WriteString(strconv.Itoa(i))
		buf.WriteString(":\n")
		buf.Write(m.Raw)
		buf.WriteString("\n\n")
	}
	return buf.String()
}

// NOTE(sr): we override this to not depend on build tags in tests
func wasmABIVersions(vs ...ast.WasmABIVersion) *ast.Capabilities {
	caps := ast.CapabilitiesForThisVersion()
	caps.WasmABIVersions = vs
	return caps
}

func TestCompilerWasmTarget(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = 7
		q = p+1`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test/p", "test/q").
				WithCapabilities(wasmABIVersions(ast.WasmABIVersion{Version: 1}))
			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.WasmModules) == 0 {
				t.Fatal("expected to find compiled wasm module")
			}

			if len(compiler.bundle.Wasm) != 0 {
				t.Error("expected NOT to find deprecated bundle `Wasm` value")
			}

			ensureEntrypointRemoved(t, compiler.bundle, "test/p")
		})
	}
}

// If we're building a wasm bundle, and the `opa` binary we use to do that
// does not support wasm _itself_, then it shouldn't bother.
func TestCompilerWasmTargetWithCapabilitiesUnset(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = 7
		q = p+1`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test/p", "test/q")
			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestCompilerWasmTargetWithCapabilitiesMismatch(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = 7
		q = p+1`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			for note, wabis := range map[string][]ast.WasmABIVersion{
				"none":     {},
				"mismatch": {{Version: 0}, {Version: 1, Minor: 2000}},
			} {
				t.Run(note, func(t *testing.T) {
					caps := ast.CapabilitiesForThisVersion()
					caps.WasmABIVersions = wabis
					compiler := New().
						WithFS(fsys).
						WithPaths(root).
						WithTarget("wasm").
						WithEntrypoints("test/p", "test/q").
						WithCapabilities(caps)
					err := compiler.Build(t.Context())
					if err == nil {
						t.Fatal("expected err, got nil")
					}
				})
			}
		})
	}
}

func TestCompilerWasmTargetMultipleEntrypoints(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p := true`,
		"policy.rego": `package policy

		authz := true`,
		"mask.rego": `package system.log
		import rego.v1

		mask contains "/input/password"`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test/p", "policy/authz").
				WithCapabilities(wasmABIVersions(ast.WasmABIVersion{Version: 1}))
			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.WasmModules) != 1 {
				t.Fatalf("expected 1 Wasm modules, got: %d", len(compiler.bundle.WasmModules))
			}

			expManifest := bundle.Manifest{}
			expManifest.Init()
			expManifest.SetRegoVersion(ast.DefaultRegoVersion)
			expManifest.WasmResolvers = []bundle.WasmResolver{
				{
					Entrypoint: "test/p",
					Module:     "/policy.wasm",
				},
				{
					Entrypoint: "policy/authz",
					Module:     "/policy.wasm",
				},
			}

			if !compiler.bundle.Manifest.Equal(expManifest) {
				t.Fatalf("\nExpected manifest: %+v\nGot: %+v\n", expManifest, compiler.bundle.Manifest)
			}

			ensureEntrypointRemoved(t, compiler.bundle, "test/p")
			ensureEntrypointRemoved(t, compiler.bundle, "policy/authz")
		})
	}
}

func TestCompilerWasmTargetAnnotations(t *testing.T) {
	files := map[string]string{
		"test.rego": `
# METADATA
# title: My test package
package test

# METADATA
# title: My P rule
# entrypoint: true
p = true`,
		"policy.rego": `
package policy

# METADATA
# title: All my Q rules
# scope: document

# METADATA
# title: My Q rule
q = true`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test", "policy/q").
				WithRegoAnnotationEntrypoints(true)

			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.WasmModules) != 1 {
				t.Fatalf("expected 1 Wasm modules, got: %d", len(compiler.bundle.WasmModules))
			}

			expWasmResolvers := []bundle.WasmResolver{
				{
					Entrypoint: "test",
					Module:     "/policy.wasm",
				},
				{
					Entrypoint: "policy/q",
					Module:     "/policy.wasm",
					Annotations: []*ast.Annotations{
						{
							Title: "All my Q rules",
							Scope: "document",
						},
						{
							Title: "My Q rule",
							Scope: "rule",
						},
					},
				},
				{
					Entrypoint: "test/p",
					Module:     "/policy.wasm",
					Annotations: []*ast.Annotations{
						{
							Title:      "My P rule",
							Scope:      "document",
							Entrypoint: true,
						},
					},
				},
			}

			if len(expWasmResolvers) != len(compiler.bundle.Manifest.WasmResolvers) {
				t.Fatalf("\nExpected WasmResolvers:\n  %+v\nGot:\n  %+v\n", expWasmResolvers, compiler.bundle.Manifest.WasmResolvers)
			}

			for i, expWasmResolver := range expWasmResolvers {
				if !expWasmResolver.Equal(&compiler.bundle.Manifest.WasmResolvers[i]) {
					t.Fatalf("WasmResolver at index %v mismatch\n\nExpected WasmResolvers:\n  %+v\nGot:\n  %+v\n",
						i, expWasmResolvers, compiler.bundle.Manifest.WasmResolvers)
				}
			}
		})
	}
}

func TestCompilerWasmTargetEntrypointDependents(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test
			import rego.v1

			p if { q }
			q if { r }
			r := 1
			s := 2
			z if { r }`}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test/r", "test/z").
				WithCapabilities(wasmABIVersions(ast.WasmABIVersion{Version: 1}))
			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.WasmModules) != 1 {
				t.Fatalf("expected 1 Wasm modules, got: %d", len(compiler.bundle.WasmModules))
			}

			expManifest := bundle.Manifest{}
			expManifest.Init()
			expManifest.SetRegoVersion(ast.DefaultRegoVersion)
			expManifest.WasmResolvers = []bundle.WasmResolver{
				{
					Entrypoint: "test/r",
					Module:     "/policy.wasm",
				},
				{
					Entrypoint: "test/z",
					Module:     "/policy.wasm",
				},
				{
					Entrypoint: "test/p",
					Module:     "/policy.wasm",
				},
				{
					Entrypoint: "test/q",
					Module:     "/policy.wasm",
				},
			}

			if !compiler.bundle.Manifest.Equal(expManifest) {
				t.Fatalf("\nExpected manifest: %+v\nGot: %+v\n", expManifest, compiler.bundle.Manifest)
			}

			ensureEntrypointRemoved(t, compiler.bundle, "test/p")
			ensureEntrypointRemoved(t, compiler.bundle, "test/q")
			ensureEntrypointRemoved(t, compiler.bundle, "test/r")
		})
	}
}

func TestCompilerWasmTargetLazyCompile(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test
		import rego.v1

		p if { input.x = q }
		q := "foo"`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("wasm").
				WithEntrypoints("test/p").
				WithOptimizationLevel(1).
				WithCapabilities(wasmABIVersions(ast.WasmABIVersion{Version: 1}))
			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.WasmModules) == 0 {
				t.Fatal("expected to find compiled wasm module")
			}

			if _, exists := compiler.compiler.Modules["optimized/test.rego"]; !exists {
				t.Fatal("expected to find optimized module on compiler")
			}

			ensureEntrypointRemoved(t, compiler.bundle, "test/p")
		})
	}
}

func ensureEntrypointRemoved(t *testing.T, b *bundle.Bundle, e string) {
	t.Helper()
	r, err := ref.ParseDataPath(e)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	for _, mf := range b.Modules {
		for _, rule := range mf.Parsed.Rules {
			if rule.Path().Equal(r) {
				t.Errorf("expected entrypoint to be removed from rego all modules in bundle, found rule: %s in %s", rule.Path(), mf.Path)
			}
		}
	}
}

func TestCompilerPlanTarget(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = 7
		q = p+1`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").
				WithEntrypoints("test/p", "test/q")
			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.PlanModules) == 0 {
				t.Fatal("expected to find compiled plan module")
			}
		})
	}
}

func TestCompilerPlanTargetPruneUnused(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test
		import rego.v1
		p contains 1
		f(x) if { p[x] }`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").
				WithEntrypoints("test").
				WithPruneUnused(true)
			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(compiler.bundle.PlanModules) == 0 {
				t.Fatal("expected to find compiled plan module")
			}

			plan := compiler.bundle.PlanModules[0].Raw
			var policy ir.Policy

			if err := json.Unmarshal(plan, &policy); err != nil {
				t.Fatal(err)
			}
			if exp, act := 1, len(policy.Funcs.Funcs); act != exp {
				t.Fatalf("expected %d funcs, got %d", exp, act)
			}
			f := policy.Funcs.Funcs[0]
			if exp, act := "g0.data.test.p", f.Name; act != exp {
				t.Fatalf("expected func named %v, got %v", exp, act)
			}
		})
	}
}

func TestCompilerPlanTargetUnmatchedEntrypoints(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p := 7
		q := p + 1`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").
				WithEntrypoints("test/p", "test/q", "test/no")
			err := compiler.Build(t.Context())
			if err == nil {
				t.Error("expected error from unmatched entrypoint")
			}
			expectError := "entrypoint \"test/no\" does not refer to a rule or policy decision"
			if err.Error() != expectError {
				t.Errorf("expected error %s, got: %s", expectError, err.Error())
			}
		})
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithTarget("plan").
				WithEntrypoints("foo", "foo.bar", "test/no")
			err := compiler.Build(t.Context())
			if err == nil {
				t.Error("expected error from unmatched entrypoints")
			}
			expectError := "entrypoint \"foo\" does not refer to a rule or policy decision"
			if err.Error() != expectError {
				t.Errorf("expected error %s, got: %s", expectError, err.Error())
			}
		})
	}
}

func TestCompilerRegoEntrypointAnnotations(t *testing.T) {
	tests := []struct {
		note            string
		entrypoints     []string
		modules         map[string]string
		data            string
		roots           []string
		wantEntrypoints map[string]struct{}
	}{
		{
			note:        "implied document scope annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

import rego.v1

# METADATA
# entrypoint: true
p if {
	q[input.x]
}

q contains 1
q contains 2
q contains 3
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/p": {},
			},
		},
		{
			note:        "package annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
# METADATA
# entrypoint: true
package test

import rego.v1

p if {
	q[input.x]
}

q contains 1
q contains 2
q contains 3
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test": {},
			},
		},
		{
			note:        "nested rule annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

import rego.v1
import data.test.nested

p if {
	q[input.x]
	nested.p
}

q contains 1
q contains 2
q contains 3
				`,
				"test/nested.rego": `
package test.nested

import rego.v1

# METADATA
# entrypoint: true
p if {
	q[input.x]
}

q contains 1
q contains 2
q contains 3
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/nested/p": {},
			},
		},
		{
			note:        "nested package annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

import rego.v1
import data.test.nested

p if {
	q[input.x]
	nested.p
}

q contains 1
q contains 2
q contains 3
				`,
				"test/nested.rego": `
# METADATA
# entrypoint: true
package test.nested

import rego.v1

p if {
	q[input.x]
}

q contains 1
q contains 2
q contains 3
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/nested": {},
			},
		},
		{
			note:        "mixed manual entrypoints + annotation entrypoints",
			entrypoints: []string{"test/p"},
			modules: map[string]string{
				"test.rego": `
package test

import rego.v1
import data.test.nested

p if {
	q[input.x]
	nested.p
}

q contains 1
q contains 2
q contains 3
				`,
				"test/nested.rego": `
# METADATA
# entrypoint: true
package test.nested

import rego.v1

p if {
	q[input.x]
}

q contains 1
q contains 2
q contains 3
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/nested": {},
				"test/p":      {},
			},
		},
		{
			note:        "overlapping manual entrypoints + annotation entrypoints",
			entrypoints: []string{"test/p"},
			modules: map[string]string{
				"test.rego": `
package test

import rego.v1

# METADATA
# entrypoint: true
p if {
	q[input.x]
}

q contains 1
q contains 2
q contains 3
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/p": {},
			},
		},
		{
			note:        "ref head rule annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

import rego.v1

# METADATA
# entrypoint: true
a.b.c.p if {
	q[input.x]
}

q contains 1
q contains 2
q contains 3
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/a/b/c/p": {},
			},
		},
		{
			note:        "mixed ref head rule/package annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
# METADATA
# entrypoint: true
package test.a.b.c

import rego.v1

# METADATA
# entrypoint: true
d.e.f.g if {
	q[input.x]
}

q contains 1
q contains 2
q contains 3
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/a/b/c":         {},
				"test/a/b/c/d/e/f/g": {},
			},
		},
		{
			note:        "numbers in refs annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

import rego.v1

# METADATA
# entrypoint: true
a.b[1.0] if {
	q[input.x]
}

q contains 1
q contains 2
q contains 3
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/a/b/1.0": {},
			},
		},
		{
			note:        "string path with brackets annotation",
			entrypoints: []string{},
			modules: map[string]string{
				"test.rego": `
package test

import rego.v1

# METADATA
# entrypoint: true
a.b["1.0.0"].foo if {
	q[input.x]
}

q contains 1
q contains 2
q contains 3
				`,
			},
			wantEntrypoints: map[string]struct{}{
				"test/a/b/1.0.0/foo": {},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			for _, useMemoryFS := range []bool{false, true} {
				test.WithTestFS(tc.modules, useMemoryFS, func(root string, fsys fs.FS) {
					compiler := New().
						WithFS(fsys).
						WithPaths(root).
						WithTarget("plan").
						WithEntrypoints(tc.entrypoints...).
						WithRegoAnnotationEntrypoints(true).
						WithPruneUnused(true)
					err := compiler.Build(t.Context())
					if err != nil {
						t.Fatal(err)
					}

					// Ensure we have the right number of entrypoints.
					if len(compiler.entrypoints) != len(tc.wantEntrypoints) {
						t.Fatalf("Wrong number of entrypoints. Expected %v, got %v.", tc.wantEntrypoints, compiler.entrypoints)
					}

					// Ensure those entrypoints match the ones we expect.
					for _, entrypoint := range compiler.entrypoints {
						if _, found := tc.wantEntrypoints[entrypoint]; !found {
							t.Fatalf("Unexpected entrypoint '%s'", entrypoint)
						}
					}
				})
			}
		})
	}
}

func TestCompilerSetRevision(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = true`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithRevision("deadbeef")
			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if compiler.bundle.Manifest.Revision != "deadbeef" {
				t.Fatal("expected revision to be set but got:", compiler.bundle.Manifest)
			}
		})
	}
}

func TestCompilerSetMetadata(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		p = true`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			metadata := map[string]any{"OPA version": "0.36.1"}
			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithMetadata(&metadata)

			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if compiler.bundle.Manifest.Metadata["OPA version"] != "0.36.1" {
				t.Fatal("expected metadata to be set but got:", compiler.bundle.Manifest)
			}
		})
	}
}

func TestCompilerSetRoots(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test

		import data.common

		x = true`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			compiler := New().
				WithFS(fsys).
				WithPaths(root).
				WithRoots("test")

			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			if len(*compiler.bundle.Manifest.Roots) != 1 || (*compiler.bundle.Manifest.Roots)[0] != "test" {
				t.Fatal("expected roots to be set to ['test'] but got:", compiler.bundle.Manifest.Roots)
			}
		})
	}
}

func TestCompilerOutput(t *testing.T) {
	// NOTE(tsandall): must use format package here because the compiler formats.
	mod := ast.MustParseModuleWithOpts(`package test
		p { input.x = data.foo }`, ast.ParserOptions{RegoVersion: ast.RegoV0})
	files := map[string]string{
		"test.rego": string(format.MustAstWithOpts(mod, format.Opts{RegoVersion: ast.RegoV0})),
		"data.json": `{"foo": 1}`,
		".manifest": `{"rego_version": 0}`,
	}

	for _, useMemoryFS := range []bool{false, true} {
		test.WithTestFS(files, useMemoryFS, func(root string, fsys fs.FS) {

			buf := bytes.NewBuffer(nil)
			compiler := New().
				WithAsBundle(true). // To respect the manifest file.
				WithFS(fsys).
				WithPaths(root).
				WithOutput(buf)
			err := compiler.Build(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			// Check that the written bundle is expected.
			result, err := bundle.NewReader(buf).Read()
			if err != nil {
				t.Fatal(err)
			}

			exp, err := loader.NewFileLoader().WithFS(fsys).AsBundle(root)
			if err != nil {
				t.Fatal(err)
			}

			if !exp.Equal(result) {
				t.Fatalf("expected-1:\n\n%+v\n\ngot-1:\n\n%+v", *exp, result)
			}

			if !exp.Manifest.Equal(result.Manifest) {
				t.Fatalf("expected-2:\n\n%+v\n\ngot-2:\n\n%+v", exp.Manifest, result.Manifest)
			}

			// Check that the returned bundle is the expected.
			compiled := compiler.Bundle()

			if !exp.Equal(*compiled) {
				t.Fatalf("expected-3:\n\n%v\n\ngot-3:\n\n%v", *exp, *compiled)
			}

			if !exp.Manifest.Equal(compiled.Manifest) {
				t.Fatalf("expected-4:\n\n%v\n\ngot-4:\n\n%v", exp.Manifest, compiled.Manifest)
			}
		})
	}
}

func TestOptimizerNoops(t *testing.T) {
	tests := []struct {
		note        string
		entrypoints []string
		modules     map[string]string
	}{
		{
			note:        "recursive result",
			entrypoints: []string{"data.test.foo"},
			modules: map[string]string{
				"test.rego": `
					package test.foo.bar

					p if { input.x = 1 }
				`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			o := getOptimizer(tc.modules, "", tc.entrypoints, nil, "", ast.ParserOptions{AllFutureKeywords: true})
			cpy := o.bundle.Copy()
			err := o.Do(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if !o.bundle.Equal(cpy) {
				t.Fatalf("Expected no change:\n\n%v\n\nGot:\n\n%v", prettyBundle{cpy}, prettyBundle{*o.bundle})
			}
		})
	}
}

func TestOptimizerErrors(t *testing.T) {
	tests := []struct {
		note        string
		entrypoints []string
		modules     map[string]string
		wantErr     error
	}{
		{
			note:        "undefined entrypoint",
			entrypoints: []string{"data.test.p"},
			wantErr:     errors.New("undefined entrypoint data.test.p"),
		},
		{
			note:        "compile error",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test
					p if { data.test.p }
				`,
			},
			wantErr: errors.New("1 error occurred: test.rego:3: rego_recursion_error: rule data.test.p is recursive: data.test.p -> data.test.p"),
		},
		{
			note:        "partial eval error",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test
					p if { {k: v | k = ["a", "a"][_]; v = [0, 1][_] } }
				`,
			},
			wantErr: errors.New("test.rego:3: eval_conflict_error: object keys must be unique"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			o := getOptimizer(tc.modules, "", tc.entrypoints, nil, "", ast.ParserOptions{AllFutureKeywords: true})
			cpy := o.bundle.Copy()
			got := o.Do(t.Context())
			if got == nil || got.Error() != tc.wantErr.Error() {
				t.Fatalf("expected error to be %v but got %v", tc.wantErr, got)
			}
			if !o.bundle.Equal(cpy) {
				t.Fatalf("Expected no change:\n\n%v\n\nGot:\n\n%v", prettyBundle{cpy}, prettyBundle{*o.bundle})
			}
		})
	}
}

func TestOptimizerOutput(t *testing.T) {
	tests := []struct {
		note        string
		entrypoints []string
		modules     map[string]string
		data        string
		roots       []string
		namespace   string
		wantModules map[string]string
	}{
		{
			note:        "rule pruning",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p if {
						q[input.x]
					}

					q contains 1
					q contains 2
					q contains 3
				`,
			},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p = __result__ if { 1 = input.x; __result__ = true }
					p = __result__ if { 2 = input.x; __result__ = true }
					p = __result__ if { 3 = input.x; __result__ = true }
				`,
				"test.rego": `
					package test

					q contains 1
					q contains 2
					q contains 3
				`,
			},
		},
		{
			note:        "support rules",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					default p = false

					p if { q[input.x] }

					q contains 1
					q contains 2`,
			},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					default p = false

					p = true if { 1 = input.x }
					p = true if { 2 = input.x }

				`,
				"test.rego": `
					package test

					q contains 1
					q contains 2
				`,
			},
		},
		{
			note:        "support rules, ref heads",
			entrypoints: []string{"data.test.p.q.r"},
			modules: map[string]string{
				"test.rego": `
					package test

					default p.q.r = false
					p.q.r if { q[input.x] }

					q contains 1
					q contains 2`,
			},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					default p.q.r = false
					p.q.r = true if { 1 = input.x }
					p.q.r = true if { 2 = input.x }

				`,
				"test.rego": `
					package test

					q contains 1
					q contains 2
				`,
			},
		},
		{
			note:        "multiple entrypoints",
			entrypoints: []string{"data.test.p", "data.test.r", "data.test.s"},
			modules: map[string]string{
				"test.rego": `
					package test

					p if {
						q[input.x]
					}

					r if {
						q[input.x]
					}

					s if {
						q[input.x]
					}

					q contains 1
				`,
			},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p = __result__ if { 1 = input.x; __result__ = true }
				`,
				"optimized/test.1.rego": `
					package test

					r = __result__ if { 1 = input.x; __result__ = true }
				`,
				"optimized/test.2.rego": `
					package test

					s = __result__ if { 1 = input.x; __result__ = true }
				`,
				"test.rego": `
					package test

					q contains 1 if { true }
				`,
			},
		},
		{
			note:        "package pruning",
			entrypoints: []string{"data.test.foo"},
			modules: map[string]string{
				"test.rego": `
					package test.foo.bar

					p = true
				`,
			},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					foo = __result__ if { __result__ = {"bar": {"p": true}} }`,
			},
		},
		{
			note:        "entrypoint dependent integrity",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p if { q[input.x] }

					q contains x if {
						s[x]
					}

					s contains 1
					s contains 2

					t if {
						p
					}

					r if { t with q as {3} }
				`,
			},
			wantModules: map[string]string{
				"optimized/test.1.rego": `
					package test

					p = __result__ if { data.test.q[input.x]; __result__ = true }
				`,
				"optimized/test.rego": `
					package test

					q contains 1 if { true }
					q contains 2 if { true }
				`,
				"test.rego": `
					package test

					s contains 1 if { true }
					s contains 2 if { true }
					t if { p }
					r = true if { t with q as {3} }
				`,
			},
		},
		{
			note:        "output filename safety",
			entrypoints: []string{`data.test["foo bar"].p`},
			modules: map[string]string{
				"x.rego": `
					package test["foo bar"]  # package does not match safe pattern so use alt. format
					p if { q[input.x] }
					q contains 1
					q contains 2
				`,
			},
			wantModules: map[string]string{
				"optimized/partial/0/0.rego": `
					package test["foo bar"]
					p = __result__ if { 1 = input.x; __result__ = true }
					p = __result__ if { 2 = input.x; __result__ = true }
				`,
				"x.rego": `
					package test["foo bar"]

					q contains 1
					q contains 2
				`,
			},
		},
		{
			note:        "generated package namespace",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p if { not q }
					q if { k[input.a]; k[input.b] }  # generate a product that is not inlined
					k = {1,2,3}
				`,
			},
			wantModules: map[string]string{
				"optimized/partial.rego": `
					package partial

					__not1_0_2__ = true if { 1 = input.a; 1 = input.b }
					__not1_0_2__ = true if { 1 = input.a; 2 = input.b }
					__not1_0_2__ = true if { 1 = input.a; 3 = input.b }
					__not1_0_2__ = true if { 2 = input.a; 1 = input.b }
					__not1_0_2__ = true if { 2 = input.a; 2 = input.b }
					__not1_0_2__ = true if { 2 = input.a; 3 = input.b }
					__not1_0_2__ = true if { 3 = input.a; 1 = input.b }
					__not1_0_2__ = true if { 3 = input.a; 2 = input.b }
					__not1_0_2__ = true if { 3 = input.a; 3 = input.b }
				`,
				"optimized/test.rego": `
					package test

					p = __result__ if { not data.partial.__not1_0_2__; __result__ = true }
				`,
				"test.rego": `
					package test

					q = true if { k[input.a]; k[input.b] }
					k = {1, 2, 3} if { true }
				`,
			},
		},
		{
			note:        "configured package namespace",
			entrypoints: []string{"data.test.p"},
			namespace:   "custom",
			modules: map[string]string{
				"test.rego": `
					package test

					p if { not q }
					q if { k[input.a]; k[input.b] }  # generate a product that is not inlined
					k = {1,2,3}
				`,
			},
			wantModules: map[string]string{
				"optimized/custom.rego": `
					package custom

					__not1_0_2__ = true if { 1 = input.a; 1 = input.b }
					__not1_0_2__ = true if { 1 = input.a; 2 = input.b }
					__not1_0_2__ = true if { 1 = input.a; 3 = input.b }
					__not1_0_2__ = true if { 2 = input.a; 1 = input.b }
					__not1_0_2__ = true if { 2 = input.a; 2 = input.b }
					__not1_0_2__ = true if { 2 = input.a; 3 = input.b }
					__not1_0_2__ = true if { 3 = input.a; 1 = input.b }
					__not1_0_2__ = true if { 3 = input.a; 2 = input.b }
					__not1_0_2__ = true if { 3 = input.a; 3 = input.b }
				`,
				"optimized/test.rego": `
					package test

					p = __result__ if { not data.custom.__not1_0_2__; __result__ = true }
				`,
				"test.rego": `
					package test

					q = true if { k[input.a]; k[input.b] }
					k = {1, 2, 3} if { true }
				`,
			},
		},
		{
			note:        "infer unknowns from roots",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p if {
						q[x]
						data.external.users[x] == input.user
					}

					q contains "foo"
					q contains "bar"
				`,
			},
			roots: []string{"test"},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p = __result__ if { data.external.users.bar = input.user; __result__ = true }
					p = __result__ if { data.external.users.foo = input.user; __result__ = true }
				`,
				"test.rego": `
					package test

					q contains "foo"
					q contains "bar"
				`,
			},
		},
		{
			note:        "generate rules with type violations: complete doc",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p if {
						x := split(input.a, ":")
						f(x[0])
					}

					f(x) if { x == null }
					f(x) if { startswith(x, "foo") }
				`,
			},
			roots: []string{"test"},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p = __result__ if { split(input.a, ":", __local3__1); startswith(__local3__1[0], "foo"); __result__ = true }
				`,
				"test.rego": `
					package test

					f(x) if { x == null }
					f(x) if { startswith(x, "foo") }
				`,
			},
		},
		{
			note:        "generate rules with type violations: partial set",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p contains msg if {
						x := split(input.a, ":")
						f(x[0])
						msg := "test string"
					}

					f(x) if { x == null }
					f(x) if { startswith(x, "foo") }
				`,
			},
			roots: []string{"test"},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p contains "test string" if { split(input.a, ":", __local4__1); startswith(__local4__1[0], "foo") }
				`,
				"test.rego": `
					package test

					f(x) if { x == null }
					f(x) if { startswith(x, "foo") }
				`,
			},
		},
		{
			note:        "generate rules with type violations: partial object",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p[k] = value if {
						x := split(input.a, ":")
						f(x[0])
						k := "a"
						value := 1
					}

					f(x) if { x == null }
					f(x) if { startswith(x, "foo") }
				`,
			},
			roots: []string{"test"},
			wantModules: map[string]string{
				"optimized/test/p.rego": `
					package test.p

					a = 1 if { split(input.a, ":", __local5__1); startswith(__local5__1[0], "foo") }
				`,
				"test.rego": `
					package test

					f(x) if { x == null }
					f(x) if { startswith(x, "foo") }
				`,
			},
		},
		{
			note:        "generate rules with type violations: negation",
			entrypoints: []string{"data.test.p"},
			modules: map[string]string{
				"test.rego": `
					package test

					p if { not q }
					q if {
						x := split(input.a, ":")
						f(x[0])
					}
					f(x) if { x == null }
					f(x) if { startswith(x, "foo") }
				`,
			},
			roots: []string{"test"},
			wantModules: map[string]string{
				"optimized/test.rego": `
					package test

					p = __result__ if { not data.partial.__not1_0_2__; __result__ = true }
				`,
				"test.rego": `
					package test
					q = true if { assign(x, split(input.a, ":")); f(x[0]) }
					f(x) if { x == null }
					f(x) if { startswith(x, "foo") }
				`,
				"optimized/partial.rego": `
					package partial
            		__not1_0_2__ = true if { split(input.a, ":", __local3__3); startswith(__local3__3[0], "foo") }
				`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			popts := ast.ParserOptions{AllFutureKeywords: true}
			o := getOptimizer(tc.modules, tc.data, tc.entrypoints, tc.roots, tc.namespace, popts)
			original := o.bundle.Copy()
			err := o.Do(t.Context())
			if err != nil {
				t.Fatal(err)
			}

			exp := &bundle.Bundle{
				Modules: getModuleFiles(tc.wantModules, false, popts),
				Data:    original.Data, // data is not pruned at all today
			}

			if len(tc.roots) > 0 {
				exp.Manifest.Roots = &tc.roots

				// optimizer will add the manifest root in the optimized bundle automatically
				if tc.namespace != "" {
					exp.Manifest.AddRoot(tc.namespace)
				} else {
					exp.Manifest.AddRoot("partial")
				}
			}

			exp.Manifest.Revision = "" // optimizations must reset the revision.

			if !exp.Equal(*o.bundle) {
				t.Errorf("Expected:\n\n%v\n\nGot:\n\n%v", prettyBundle{*exp}, prettyBundle{*o.bundle})
			}

			if !o.bundle.Manifest.Equal(exp.Manifest) {
				t.Errorf("Expected manifest: %v\n\nGot manifest: %v", exp.Manifest, o.bundle.Manifest)
			}
		})
	}
}

func TestOptimizerError(t *testing.T) {
	tests := []struct {
		note        string
		roots       []string
		entrypoints []string
		modules     map[string]string
		expErr      string
	}{
		{
			// Regression test for https://github.com/open-policy-agent/opa/issues/7321
			note:        "short entrypoint ref",
			roots:       []string{"test"},
			entrypoints: []string{"data.test"},
			modules: map[string]string{
				"test.rego": `
					package test
					p = true`,
			},
			expErr: `invalid entrypoint data.test: to create optimized support module, the entrypoint ref must have at least two components in addition to the 'data' root`,
		},
		{
			// Regression test for https://github.com/open-policy-agent/opa/issues/7321
			note:        "data only entrypoint ref",
			roots:       []string{"test"},
			entrypoints: []string{"data"},
			modules: map[string]string{
				"test.rego": `
					package test
					p = true`,
			},
			expErr: `invalid entrypoint data: to create optimized support module, the entrypoint ref must have at least two components in addition to the 'data' root`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			popts := ast.ParserOptions{AllFutureKeywords: true}
			o := getOptimizer(tc.modules, "", tc.entrypoints, tc.roots, "", popts)

			err := o.Do(t.Context())
			if err == nil {
				t.Fatal("expected error but got nil")
			}
			if err.Error() != tc.expErr {
				t.Fatalf("expected error to be:\n\n%v\n\nbut got:\n\n%v", tc.expErr, err)
			}
		})
	}
}

func TestRefSet(t *testing.T) {
	rs := newRefSet(ast.MustParseRef("input"), ast.MustParseRef("data.foo.bar"))

	expFound := []string{
		"input",
		"input.foo",
		"data.foo.bar",
		"data.foo.bar.baz",
		"data.foo.bar[1]",
	}

	for _, exp := range expFound {
		if !rs.ContainsPrefix(ast.MustParseRef(exp)) {
			t.Fatal("expected to find:", exp)
		}
	}

	expNotFound := []string{
		"x.bar",
		"data",
		"data.bar",
		"data.foo",
	}

	for _, exp := range expNotFound {
		if rs.ContainsPrefix(ast.MustParseRef(exp)) {
			t.Fatal("expected not to find:", exp)
		}
	}

	rs.AddPrefix(ast.MustParseRef("data.foo"))

	if !rs.ContainsPrefix(ast.MustParseRef("data.foo")) {
		t.Fatal("expected to find data.foo after adding to set")
	}

	sorted := rs.Sorted()

	if len(sorted) != 2 || !sorted[0].Equal(ast.MustParseTerm("data.foo")) || !sorted[1].Equal(ast.MustParseTerm("input")) {
		t.Fatal("expected 2 prefixes (data.foo and input) but got:", sorted)
	}

	// The prefixes should not be affected (because data.foo already exists).
	rs.AddPrefix(ast.MustParseRef("data.foo.qux"))
	sorted = rs.Sorted()

	if len(sorted) != 2 || !sorted[0].Equal(ast.MustParseTerm("data.foo")) || !sorted[1].Equal(ast.MustParseTerm("input")) {
		t.Fatal("expected 2 prefixes (data.foo and input) but got:", sorted)
	}

}

func getOptimizer(modules map[string]string, data string, entries []string, roots []string, ns string, popts ast.ParserOptions) *optimizer {

	b := &bundle.Bundle{
		Modules: getModuleFiles(modules, true, popts),
	}

	if data != "" {
		b.Data = util.MustUnmarshalJSON([]byte(data)).(map[string]any)
	}

	if len(roots) > 0 {
		b.Manifest.Roots = &roots
	}

	b.Manifest.Init()
	b.Manifest.Revision = "DEADBEEF" // ensures that the manifest revision is getting reset
	entrypoints := make([]*ast.Term, len(entries))

	for i := range entrypoints {
		entrypoints[i] = ast.MustParseTerm(entries[i])
	}

	o := newOptimizer(ast.CapabilitiesForThisVersion(), b).
		WithEntrypoints(entrypoints)

	if ns != "" {
		o = o.WithPartialNamespace(ns)
	}

	o.resultsymprefix = ""

	return o
}

func getModuleFiles(src map[string]string, includeRaw bool, popts ast.ParserOptions) []bundle.ModuleFile {

	keys := util.KeysSorted(src)
	modules := make([]bundle.ModuleFile, 0, len(keys))

	for _, k := range keys {
		module, err := ast.ParseModuleWithOpts(k, src[k], popts)
		if err != nil {
			panic(err)
		}
		modules = append(modules, bundle.ModuleFile{
			Parsed: module,
			Path:   k,
			URL:    k,
		})
		if includeRaw {
			modules[len(modules)-1].Raw = []byte(src[k])
		}
	}

	return modules
}

type prettyBundle struct {
	bundle.Bundle
}

func (p prettyBundle) String() string {
	buf := make([]string, 2, 2+len(p.Modules)*5)
	buf[0] = fmt.Sprintf("%d module(s) (hiding data):", len(p.Modules))
	buf[1] = ""

	for _, mf := range p.Modules {
		buf = append(buf,
			"#",
			fmt.Sprintf("# Module: %q", mf.Path),
			"#",
			mf.Parsed.String(),
			"",
		)
	}

	return strings.Join(buf, "\n")
}
