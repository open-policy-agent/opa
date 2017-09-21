// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package loader

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/open-policy-agent/opa/ast"
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
		"/a/data1.json": `[1,2,3]`,
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
			"b": {
				"aaa": {
					"bbb": 1,
					"ccc": 2
				},
				"d": null
			}
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

func TestLoadRooted(t *testing.T) {
	files := map[string]string{
		"/foo.json":     "[1,2,3]",
		"/bar.yaml":     "abc",
		"/baz/qux.json": "null",
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
		{"four": [1,2,3], "one": {"two": "abc"}, "three": {"baz": null}}
		`)
		if !reflect.DeepEqual(loaded.Documents, expected) {
			t.Fatalf("Expected %v but got: %v", expected, loaded.Documents)
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
		"/z.rego": `
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

func parseYAML(s string) interface{} {
	var x interface{}
	if err := yaml.Unmarshal([]byte(s), &x); err != nil {
		panic(err)
	}
	return x
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
