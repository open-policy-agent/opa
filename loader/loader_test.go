// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package loader

import (
	"bytes"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestLoadJSON(t *testing.T) {

	files := map[string]string{
		"/foo.json": `{"a": [1,2,3]}`,
	}

	test.WithTempFS(files, func(rootDir string) {

		loaded, err := NewFileLoader().All([]string{filepath.Join(rootDir, "foo.json")})

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expected := parseJSON(files["/foo.json"])

		if !reflect.DeepEqual(loaded.Documents, expected) {
			t.Fatalf("Expected %v but got: %v", expected, loaded.Documents)
		}
	})
}

func TestLoadRego(t *testing.T) {

	files := map[string]string{
		"/foo.rego": `package ex

p = true { true }`}

	test.WithTempFS(files, func(rootDir string) {
		moduleFile := filepath.Join(rootDir, "foo.rego")
		loaded, err := NewFileLoader().All([]string{moduleFile})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := ast.MustParseModule(files["/foo.rego"])
		if !expected.Equal(loaded.Modules[CleanPath(moduleFile)].Parsed) {
			t.Fatalf("Expected:\n%v\n\nGot:\n%v", expected, loaded.Modules[moduleFile])
		}
	})
}

func TestLoadYAML(t *testing.T) {

	files := map[string]string{
		"/foo.yml": `
        a:
            - 1
            - b
            - "c"
            - null
            - true
            - false
        `,
	}

	test.WithTempFS(files, func(rootDir string) {
		yamlFile := filepath.Join(rootDir, "foo.yml")
		loaded, err := NewFileLoader().All([]string{yamlFile})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := parseJSON(`
        {"a": [1, "b", "c", null, true, false]}`)
		if !reflect.DeepEqual(loaded.Documents, expected) {
			t.Fatalf("Expected %v but got: %v", expected, loaded.Documents)
		}
	})
}

func TestLoadGuessYAML(t *testing.T) {
	files := map[string]string{
		"/foo": `
        a: b
        `,
	}
	test.WithTempFS(files, func(rootDir string) {
		yamlFile := filepath.Join(rootDir, "foo")
		loaded, err := NewFileLoader().All([]string{yamlFile})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := parseJSON(`{"a": "b"}`)
		if !reflect.DeepEqual(loaded.Documents, expected) {
			t.Fatalf("Expected %v but got: %v", expected, loaded.Documents)
		}
	})
}

func TestLoadDirRecursive(t *testing.T) {
	files := map[string]string{
		"/a/data1.json": `{"a": [1,2,3]}`,
		"/a/e.rego":     `package q`,
		"/b/data2.yaml": `{"aaa": {"bbb": 1}}`,
		"/b/data3.yaml": `{"aaa": {"ccc": 2}}`,
		"/b/d/x.json":   "null",
		"/b/d/e.rego":   `package p`,
		"/b/d/ignore":   `deadbeef`,
		"/foo":          `{"zzz": "b"}`,
	}

	test.WithTempFS(files, func(rootDir string) {
		loaded, err := NewFileLoader().All(mustListPaths(rootDir, false)[1:])
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expectedDocuments := parseJSON(`
		{
			"zzz": "b",
			"a": [1,2,3],
			"aaa": {
				"bbb": 1,
				"ccc": 2
			},
			"d": null
		}
		`)
		if !reflect.DeepEqual(loaded.Documents, expectedDocuments) {
			t.Fatalf("Expected:\n%v\n\nGot:\n%v", expectedDocuments, loaded.Documents)
		}
		mod1 := ast.MustParseModule(files["/a/e.rego"])
		mod2 := ast.MustParseModule(files["/b/d/e.rego"])
		expectedMod1 := loaded.Modules[CleanPath(filepath.Join(rootDir, "/a/e.rego"))].Parsed
		expectedMod2 := loaded.Modules[CleanPath(filepath.Join(rootDir, "/b/d/e.rego"))].Parsed
		if !mod1.Equal(expectedMod1) {
			t.Fatalf("Expected:\n%v\n\nGot:\n%v", expectedMod1, mod1)
		}
		if !mod2.Equal(expectedMod2) {
			t.Fatalf("Expected:\n%v\n\nGot:\n%v", expectedMod2, mod2)
		}
	})
}

func TestFilteredPaths(t *testing.T) {
	files := map[string]string{
		"/a/data1.json": `{"a": [1,2,3]}`,
		"/a/e.rego":     `package q`,
		"/b/data2.yaml": `{"aaa": {"bbb": 1}}`,
		"/b/data3.yaml": `{"aaa": {"ccc": 2}}`,
		"/b/d/x.json":   "null",
		"/b/d/e.rego":   `package p`,
		"/b/d/ignore":   `deadbeef`,
		"/foo":          `{"zzz": "b"}`,
	}

	test.WithTempFS(files, func(rootDir string) {

		paths := []string{
			filepath.Join(rootDir, "a"),
			filepath.Join(rootDir, "b"),
			filepath.Join(rootDir, "foo"),
		}

		result, err := FilteredPaths(paths, nil)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if len(result) != len(files) {
			t.Fatalf("Expected %v files across directories but got %v", len(files), len(result))
		}
	})
}

func TestGetBundleDirectoryLoader(t *testing.T) {
	files := map[string]string{
		"bundle.tar.gz": "",
	}

	mod := "package b.c\np=1"

	test.WithTempFS(files, func(rootDir string) {

		bundleFile := filepath.Join(rootDir, "bundle.tar.gz")

		f, err := os.Create(bundleFile)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		b := &bundle.Bundle{
			Manifest: bundle.Manifest{
				Roots:    &[]string{"a", "b/c"},
				Revision: "123",
			},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": []int{4, 5, 6},
				},
			},
			Modules: []bundle.ModuleFile{
				{
					URL:    path.Join(bundleFile, "policy.rego"),
					Path:   "/policy.rego",
					Raw:    []byte(mod),
					Parsed: ast.MustParseModule(mod),
				},
			},
		}

		err = bundle.Write(f, *b)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		err = f.Close()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		bl, isDir, err := GetBundleDirectoryLoader(bundleFile)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if isDir {
			t.Fatal("Expected bundle to be gzipped tarball but got directory")
		}

		// check files
		var result []string
		for {
			f, err := bl.NextFile()
			if err == io.EOF {
				break
			}

			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			result = append(result, f.Path())
		}

		if len(result) != 3 {
			t.Fatalf("Expected 3 files in the bundle but got %v", len(result))
		}
	})
}

func TestLoadBundle(t *testing.T) {

	test.WithTempFS(nil, func(rootDir string) {

		f, err := os.Create(filepath.Join(rootDir, "bundle.tar.gz"))
		if err != nil {
			t.Fatal(err)
		}

		var testBundle = bundle.Bundle{
			Modules: []bundle.ModuleFile{
				{
					Path: "x.rego",
					Raw: []byte(`
				package baz

				p = 1`),
				},
			},
			Data: map[string]interface{}{
				"foo": "bar",
			},
			Manifest: bundle.Manifest{
				Revision: "",
				Roots:    &[]string{""},
			},
		}

		if err := bundle.Write(f, testBundle); err != nil {
			t.Fatal(err)
		}

		paths := mustListPaths(rootDir, false)[1:]
		loaded, err := NewFileLoader().All(paths)
		if err != nil {
			t.Fatal(err)
		}

		actualData := testBundle.Data
		actualData["system"] = map[string]interface{}{"bundle": map[string]interface{}{"manifest": map[string]interface{}{"revision": "", "roots": []interface{}{""}}}}

		if !reflect.DeepEqual(actualData, loaded.Documents) {
			t.Fatalf("Expected %v but got: %v", actualData, loaded.Documents)
		}

		if !bytes.Equal(testBundle.Modules[0].Raw, loaded.Modules["/x.rego"].Raw) {
			t.Fatalf("Expected %v but got: %v", string(testBundle.Modules[0].Raw), loaded.Modules["/x.rego"].Raw)
		}
	})
}

func TestLoadBundleWithReader(t *testing.T) {

	buf := bytes.Buffer{}
	testBundle := bundle.Bundle{
		Modules: []bundle.ModuleFile{
			{
				Path: "x.rego",
				Raw: []byte(`
				package baz

				p = 1`),
			},
		},
		Data: map[string]interface{}{
			"foo": "bar",
		},
		Manifest: bundle.Manifest{
			Revision: "",
			Roots:    &[]string{"foo", "baz"},
		},
	}

	if err := bundle.Write(&buf, testBundle); err != nil {
		t.Fatal(err)
	}

	b, err := NewFileLoader().WithReader(&buf).AsBundle("bundle.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if b == nil {
		t.Fatalf("Expected bundle to be non-nil")
	}

	if exp, act := 1, len(b.Modules); exp != act {
		t.Fatalf("expected %d modules, got %d", exp, act)
	}

	expectedModulePaths := map[string]struct{}{
		"/x.rego": {},
	}
	for _, mf := range b.Modules {
		if _, found := expectedModulePaths[mf.Path]; !found {
			t.Errorf("Unexpected module file with path %s in bundle modules", mf.Path)
		}
	}

	if exp, act := map[string]any{"foo": "bar"}, b.Data; !reflect.DeepEqual(act, exp) {
		t.Fatalf("expected data %+v, got %+v", exp, act)
	}
	if exp, act := []string{"foo", "baz"}, *b.Manifest.Roots; !reflect.DeepEqual(act, exp) {
		t.Fatalf("expected roots %v, got %v", exp, act)
	}
}

func TestLoadBundleSubDir(t *testing.T) {

	test.WithTempFS(nil, func(rootDir string) {

		if err := os.MkdirAll(filepath.Join(rootDir, "a", "b"), 0777); err != nil {
			t.Fatal(err)
		}

		f, err := os.Create(filepath.Join(rootDir, "a", "b", "bundle.tar.gz"))
		if err != nil {
			t.Fatal(err)
		}

		var testBundle = bundle.Bundle{
			Modules: []bundle.ModuleFile{
				{
					Path: "x.rego",
					Raw: []byte(`
				package baz

				p = 1`),
				},
			},
			Data: map[string]interface{}{
				"foo": "bar",
			},
			Manifest: bundle.Manifest{
				Revision: "",
				Roots:    &[]string{""},
			},
		}

		if err := bundle.Write(f, testBundle); err != nil {
			t.Fatal(err)
		}

		paths := mustListPaths(rootDir, false)[1:]
		loaded, err := NewFileLoader().All(paths)
		if err != nil {
			t.Fatal(err)
		}

		actualData := testBundle.Data
		actualData["system"] = map[string]interface{}{"bundle": map[string]interface{}{"manifest": map[string]interface{}{"revision": "", "roots": []interface{}{""}}}}

		if !reflect.DeepEqual(map[string]interface{}{"b": testBundle.Data}, loaded.Documents) {
			t.Fatalf("Expected %v but got: %v", testBundle.Data, loaded.Documents)
		}

		if !bytes.Equal(testBundle.Modules[0].Raw, loaded.Modules["/x.rego"].Raw) {
			t.Fatalf("Expected %v but got: %v", string(testBundle.Modules[0].Raw), loaded.Modules["/x.rego"].Raw)
		}
	})
}

func TestAsBundleWithDir(t *testing.T) {
	files := map[string]string{
		"/foo/data.json":    "[1,2,3]",
		"/bar/bar.yaml":     "abc",  // Should be ignored
		"/baz/qux/qux.json": "null", // Should be ignored
		"/foo/policy.rego":  "package foo\np = 1",
		"base.rego":         "package bar\nx = 1",
		"/.manifest":        `{"roots": ["foo", "bar", "baz"]}`,
	}

	test.WithTempFS(files, func(rootDir string) {
		b, err := NewFileLoader().AsBundle(rootDir)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if b == nil {
			t.Fatalf("Expected bundle to be non-nil")
		}

		if len(b.Modules) != 2 {
			t.Fatalf("expected 2 modules, got %d", len(b.Modules))
		}

		expectedModulePaths := map[string]struct{}{
			filepath.Join(rootDir, "foo/policy.rego"): {},
			filepath.Join(rootDir, "base.rego"):       {},
		}
		for _, mf := range b.Modules {
			if _, found := expectedModulePaths[mf.Path]; !found {
				t.Errorf("Unexpected module file with path %s in bundle modules", mf.Path)
			}
		}

		expectedData := util.MustUnmarshalJSON([]byte(`{"foo": [1,2,3]}`))
		if !reflect.DeepEqual(b.Data, expectedData) {
			t.Fatalf("expected data %+v, got %+v", expectedData, b.Data)
		}

		expectedRoots := []string{"foo", "bar", "baz"}
		if !reflect.DeepEqual(*b.Manifest.Roots, expectedRoots) {
			t.Fatalf("expected roots %s, got: %s", expectedRoots, *b.Manifest.Roots)
		}
	})
}

func TestAsBundleWithFileURLDir(t *testing.T) {
	files := map[string]string{
		"/foo/data.json":   "[1,2,3]",
		"/foo/policy.rego": "package foo.bar\np = 1",
		"/.manifest":       `{"roots": ["foo"]}`,
	}

	test.WithTempFS(files, func(rootDir string) {
		b, err := NewFileLoader().AsBundle("file://" + rootDir)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if b == nil {
			t.Fatalf("Expected bundle to be non-nil")
		}

		if len(b.Modules) != 1 {
			t.Fatalf("expected 1 modules, got %d", len(b.Modules))
		}
		expectedModulePaths := map[string]struct{}{
			filepath.Join(rootDir, "/foo/policy.rego"): {},
		}
		for _, mf := range b.Modules {
			if _, found := expectedModulePaths[mf.Path]; !found {
				t.Errorf("Unexpected module file with path %s in bundle modules", mf.Path)
			}
		}

		expectedData := util.MustUnmarshalJSON([]byte(`{"foo": [1,2,3]}`))
		if !reflect.DeepEqual(b.Data, expectedData) {
			t.Fatalf("expected data %+v, got %+v", expectedData, b.Data)
		}

		expectedRoots := []string{"foo"}
		if !reflect.DeepEqual(*b.Manifest.Roots, expectedRoots) {
			t.Fatalf("expected roots %s, got: %s", expectedRoots, *b.Manifest.Roots)
		}
	})
}

func TestAsBundleWithFile(t *testing.T) {
	files := map[string]string{
		"bundle.tar.gz": "",
	}

	mod := "package b.c\np=1"

	test.WithTempFS(files, func(rootDir string) {

		bundleFile := filepath.Join(rootDir, "bundle.tar.gz")

		f, err := os.Create(bundleFile)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		b := &bundle.Bundle{
			Manifest: bundle.Manifest{
				Roots:    &[]string{"a", "b/c"},
				Revision: "123",
			},
			Data: map[string]interface{}{
				"a": map[string]interface{}{
					"b": []int{4, 5, 6},
				},
			},
			Modules: []bundle.ModuleFile{
				{
					URL:    path.Join(bundleFile, "policy.rego"),
					Path:   "/policy.rego",
					Raw:    []byte(mod),
					Parsed: ast.MustParseModule(mod),
				},
			},
		}

		err = bundle.Write(f, *b)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		err = f.Close()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		actual, err := NewFileLoader().AsBundle(bundleFile)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		var tmp interface{} = b
		err = util.RoundTrip(&tmp)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		if !actual.Equal(*b) {
			t.Fatalf("Loaded bundle doesn't match expected.\n\nExpected: %+v\n\nActual: %+v\n\n", b, actual)
		}
	})
}

func TestLoadRooted(t *testing.T) {
	files := map[string]string{
		"/foo.json":         "[1,2,3]",
		"/bar/bar.yaml":     "abc",
		"/baz/qux/qux.json": "null",
	}

	test.WithTempFS(files, func(rootDir string) {
		paths := mustListPaths(rootDir, false)[1:]
		sort.Strings(paths)
		paths[0] = "one.two:" + paths[0]
		paths[1] = "three:" + paths[1]
		paths[2] = "four:" + paths[2]
		t.Log(paths)
		loaded, err := NewFileLoader().All(paths)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := parseJSON(`
		{"four": [1,2,3], "one": {"two": "abc"}, "three": {"qux": null}}
		`)
		if !reflect.DeepEqual(loaded.Documents, expected) {
			t.Fatalf("Expected %v but got: %v", expected, loaded.Documents)
		}
	})
}

//go:embed internal/embedtest
var embedTestFS embed.FS

func TestLoadFS(t *testing.T) {
	paths := []string{
		"four:foo.json",
		"one.two:bar",
		"three:baz",
	}

	fsys, err := fs.Sub(embedTestFS, "internal/embedtest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	loaded, err := NewFileLoader().WithFS(fsys).All(paths)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedRegoBytes, err := fs.ReadFile(fsys, "bar/bar.rego")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	expectedRego := ast.MustParseModule(string(expectedRegoBytes))
	moduleFile := "bar/bar.rego"
	if !expectedRego.Equal(loaded.Modules[moduleFile].Parsed) {
		t.Fatalf(
			"Expected:\n%v\n\nGot:\n%v",
			expectedRego,
			loaded.Modules[moduleFile],
		)
	}

	expected := parseJSON(`
	{"four": [1,2,3], "one": {"two": "abc"}, "three": {"qux": null}}
	`)
	if !reflect.DeepEqual(loaded.Documents, expected) {
		t.Fatalf("Expected %v but got: %v", expected, loaded.Documents)
	}
}

func TestLoadWithJSONOptions(t *testing.T) {
	paths := []string{
		"four:foo.json",
		"one.two:bar",
		"three:baz",
	}

	fsys, err := fs.Sub(embedTestFS, "internal/embedtest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// load the file with JSON options set to include location data
	loaded, err := NewFileLoader().WithFS(fsys).WithJSONOptions(&ast.JSONOptions{
		MarshalOptions: ast.JSONMarshalOptions{
			IncludeLocation: ast.NodeToggle{
				Package: true,
			},
		},
	}).All(paths)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	mod, ok := loaded.Modules["bar/bar.rego"]
	if !ok {
		t.Fatalf("Expected bar/bar.rego to be loaded")
	}

	bs, err := json.Marshal(mod.Parsed.Package)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	exp := `{"location":{"file":"bar/bar.rego","row":1,"col":1},"path":[{"type":"var","value":"data"},{"type":"string","value":"bar"}]}`
	if string(bs) != exp {
		t.Fatalf("Expected %v but got: %v", exp, string(bs))
	}
}

func TestGlobExcludeName(t *testing.T) {

	files := map[string]string{
		"/.data.json":          `{"x":1}`,
		"/.y/data.json":        `{"y": 2}`,
		"/.y/z/data.json":      `3`,
		"/z/.hidden/data.json": `"donotinclude"`,
		"/z/a/.hidden.json":    `"donotinclude"`,
	}

	test.WithTempFS(files, func(rootDir string) {
		paths := mustListPaths(rootDir, false)[1:]
		sort.Strings(paths)
		result, err := NewFileLoader().Filtered(paths, GlobExcludeName(".*", 1))
		if err != nil {
			t.Fatal(err)
		}
		exp := parseJSON(`{
			"x": 1,
			"y": 2,
			"z": 3
		}`)
		if !reflect.DeepEqual(exp, result.Documents) {
			t.Fatalf("Expected %v but got %v", exp, result.Documents)
		}
	})
}

func TestLoadErrors(t *testing.T) {
	files := map[string]string{
		"/x1.json":    `{"x": [1,2,3]}`,
		"/x2.json":    `{"x": {"y": 1}}`,
		"/empty.rego": `   `,
		"/dir/a.json": ``,
		"/dir/b.yaml": `
		foo:
		  - bar:
		`,
		"/bad_doc.json": "[1,2,3]",
	}
	test.WithTempFS(files, func(rootDir string) {
		paths := mustListPaths(rootDir, false)[1:]
		sort.Strings(paths)
		_, err := NewFileLoader().All(paths)
		if err == nil {
			t.Fatalf("Expected failure")
		}

		expected := []string{
			"bad_doc.json: bad document type",
			"a.json: EOF",
			"b.yaml: error converting YAML to JSON",
			"empty.rego:0: rego_parse_error: empty module",
			"x2.json: merge error",
			"rego_parse_error: empty module",
		}

		for _, s := range expected {
			if !strings.Contains(err.Error(), s) {
				t.Fatalf("Expected error to contain %v but got:\n%v", s, err)
			}
		}
	})
}

func TestLoadFileURL(t *testing.T) {
	files := map[string]string{
		"/a/a/1.json": `1`,        // this will load as a directory (e.g., file://a/a)
		"b.json":      `{"b": 2}`, // this will load as a normal file
		"c.json":      `3`,        // this will loas as rooted file
	}
	test.WithTempFS(files, func(rootDir string) {

		paths := mustListPaths(rootDir, false)[1:]
		sort.Strings(paths)

		for i := range paths {
			paths[i] = "file://" + paths[i]
		}

		paths[2] = "c:" + paths[2]

		result, err := NewFileLoader().All(paths)
		if err != nil {
			t.Fatal(err)
		}

		exp := parseJSON(`{"a": 1, "b": 2, "c": 3}`)
		if !reflect.DeepEqual(exp, result.Documents) {
			t.Fatalf("Expected %v but got %v", exp, result.Documents)
		}
	})
}

func TestUnsupportedURLScheme(t *testing.T) {
	_, err := NewFileLoader().All([]string{"http://openpolicyagent.org"})
	if err == nil || !strings.Contains(err.Error(), "unsupported URL scheme: http://openpolicyagent.org") {
		t.Fatal(err)
	}
}

func TestSplitPrefix(t *testing.T) {

	tests := []struct {
		input     string
		wantParts []string
		wantPath  string
	}{
		{
			input:    "foo/bar",
			wantPath: "foo/bar",
		},
		{
			input:     "foo:/bar",
			wantParts: []string{"foo"},
			wantPath:  "/bar",
		},
		{
			input:     "foo.bar:/baz",
			wantParts: []string{"foo", "bar"},
			wantPath:  "/baz",
		},
		{
			input:    "file:///a/b/c",
			wantPath: "file:///a/b/c",
		},
		{
			input:     "x.y:file:///a/b/c",
			wantParts: []string{"x", "y"},
			wantPath:  "file:///a/b/c",
		},
		{
			input:    "file:///c:/a/b/c",
			wantPath: "file:///c:/a/b/c",
		},
		{
			input:     "x.y:file:///c:/a/b/c",
			wantParts: []string{"x", "y"},
			wantPath:  "file:///c:/a/b/c",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			parts, gotPath := SplitPrefix(tc.input)
			if !reflect.DeepEqual(parts, tc.wantParts) {
				t.Errorf("wanted parts %v but got %v", tc.wantParts, parts)
			}
			if gotPath != tc.wantPath {
				t.Errorf("wanted path %q but got %q", gotPath, tc.wantPath)
			}
		})
	}
}

func TestLoadRegos(t *testing.T) {
	files := map[string]string{
		"/x.rego": `
			package x
			p = true
			`,
		"/y.reg": `
			package x
			p = true { # syntax error missing }
		`,
		"/subdir/z.rego": `
			package x
			q = true
		`,
	}

	test.WithTempFS(files, func(rootDir string) {
		paths := mustListPaths(rootDir, false)[1:]
		sort.Strings(paths)
		result, err := AllRegos(paths)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Modules) != 2 {
			t.Fatalf("Expected exactly two modules but found: %v", result)
		}
	})
}

func parseJSON(x string) interface{} {
	return util.MustUnmarshalJSON([]byte(x))
}

func mustListPaths(path string, recurse bool) (paths []string) {
	paths, err := Paths(path, recurse)
	if err != nil {
		panic(err)
	}
	return paths
}

func TestDirs(t *testing.T) {
	paths := []string{
		"/foo/bar.json", "/foo/bar/baz.json", "/foo.json",
	}

	e := []string{"/", "/foo", "/foo/bar"}
	sorted := Dirs(paths)
	if !reflect.DeepEqual(sorted, e) {
		t.Errorf("got: %q wanted: %q", sorted, e)
	}
}

func TestSchemas(t *testing.T) {

	tests := []struct {
		note   string
		path   string
		files  map[string]string
		exp    map[string]string
		expErr string
	}{
		{
			note: "empty path",
			path: "", // no error, no files
		},
		{
			note:   "bad file path",
			path:   "foo/bar/baz.json",
			expErr: "stat foo/bar/baz.json: no such file or directory",
		},
		{
			note: "bad file content",
			path: "foo/bar/baz.json",
			files: map[string]string{
				"foo/bar/baz.json": `{
					"foo
				}`,
			},
			expErr: "found unexpected end of stream",
		},
		{
			note: "one global file",
			path: "foo/bar/baz.json",
			files: map[string]string{
				"foo/bar/baz.json": `{"type": "string"}`,
			},
			exp: map[string]string{
				"schema": `{"type": "string"}`,
			},
		},
		{
			note: "directory loading",
			path: "foo/",
			files: map[string]string{
				"foo/qux.json":     `{"type": "number"}`,
				"foo/bar/baz.json": `{"type": "string"}`,
			},
			exp: map[string]string{
				"schema.qux":     `{"type": "number"}`,
				"schema.bar.baz": `{"type": "string"}`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			test.WithTempFS(tc.files, func(rootDir string) {
				err := os.Chdir(rootDir)
				if err != nil {
					t.Fatal(err)
				}
				ss, err := Schemas(tc.path)
				if tc.expErr != "" {
					if err == nil {
						t.Fatal("expected error")
					}
					if !strings.Contains(err.Error(), tc.expErr) {
						t.Fatalf("expected error to contain %q but got %q", tc.expErr, err)
					}
				} else {
					if err != nil {
						t.Fatal("unexpected error:", err)
					}
					for k, v := range tc.exp {
						var key ast.Ref
						if k == "schema" {
							key = ast.SchemaRootRef.Copy()
						} else {
							key = ast.MustParseRef(k)
						}
						var schema interface{}
						err = util.Unmarshal([]byte(v), &schema)
						if err != nil {
							t.Fatalf("Unexpected error: %v", err)
						}
						result := ss.Get(key)
						if result == nil {
							t.Fatalf("expected schema with key %v", key)
						}
						if !reflect.DeepEqual(schema, result) {
							t.Fatalf("expected schema %v but got %v", schema, result)
						}
					}
				}
			})
		})
	}
}
