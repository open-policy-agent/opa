// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package loader

import (
	"bytes"
	"os"
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

		loaded, err := All([]string{filepath.Join(rootDir, "foo.json")})

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
		loaded, err := All([]string{moduleFile})
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
		loaded, err := All([]string{yamlFile})
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
		loaded, err := All([]string{yamlFile})
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
		loaded, err := All(mustListPaths(rootDir, false)[1:])
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
		loaded, err := All(paths)
		if err != nil {
			t.Fatal(err)
		}

		actualData := testBundle.Data
		actualData["system"] = map[string]interface{}{"bundle": map[string]interface{}{"manifest": map[string]interface{}{"revision": "", "roots": []interface{}{""}}}}

		if !reflect.DeepEqual(actualData, loaded.Documents) {
			t.Fatalf("Expected %v but got: %v", actualData, loaded.Documents)
		}

		if !bytes.Equal(testBundle.Modules[0].Raw, loaded.Modules["/x.rego"].Raw) {
			t.Fatalf("Expected %v but got: %v", string(testBundle.Modules[0].Raw), loaded.Modules["x.rego"].Raw)
		}
	})

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
		loaded, err := All(paths)
		if err != nil {
			t.Fatal(err)
		}

		actualData := testBundle.Data
		actualData["system"] = map[string]interface{}{"bundle": map[string]interface{}{"manifest": map[string]interface{}{"revision": "", "roots": []interface{}{""}}}}

		if !reflect.DeepEqual(map[string]interface{}{"b": testBundle.Data}, loaded.Documents) {
			t.Fatalf("Expected %v but got: %v", testBundle.Data, loaded.Documents)
		}

		if !bytes.Equal(testBundle.Modules[0].Raw, loaded.Modules["/x.rego"].Raw) {
			t.Fatalf("Expected %v but got: %v", string(testBundle.Modules[0].Raw), loaded.Modules["x.rego"].Raw)
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
		b, err := AsBundle(rootDir)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if b == nil {
			t.Fatalf("Expected bundle to be non-nil")
		}

		if len(b.Modules) != 2 {
			t.Fatalf("expected 2 modules, got %d", len(b.Modules))
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
		"/foo/data.json": "[1,2,3]",
		"/.manifest":     `{"roots": ["foo"]}`,
	}

	test.WithTempFS(files, func(rootDir string) {
		b, err := AsBundle("file://" + rootDir)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if b == nil {
			t.Fatalf("Expected bundle to be non-nil")
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
				Path:   "/policy.rego",
				Raw:    []byte(mod),
				Parsed: ast.MustParseModule(mod),
			},
		},
	}

	test.WithTempFS(files, func(rootDir string) {
		path := filepath.Join(rootDir, "bundle.tar.gz")
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		err = bundle.Write(f, *b)
		f.Close()
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		actual, err := AsBundle(path)
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
		loaded, err := All(paths)
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
		result, err := Filtered(paths, GlobExcludeName(".*", 1))
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
		_, err := All(paths)
		if err == nil {
			t.Fatalf("Expected failure")
		}

		expected := []string{
			"bad_doc.json: bad document type",
			"a.json: EOF",
			"b.yaml: error converting YAML to JSON",
			"empty.rego: empty policy",
			"x2.json: merge error",
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

		result, err := All(paths)
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
	_, err := All([]string{"http://openpolicyagent.org"})
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
			parts, path := SplitPrefix(tc.input)
			if !reflect.DeepEqual(parts, tc.wantParts) {
				t.Errorf("wanted parts %v but got %v", tc.wantParts, parts)
			}
			if path != tc.wantPath {
				t.Errorf("wanted path %q but got %q", path, tc.wantPath)
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
