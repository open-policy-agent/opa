// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inspect

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/util/test"
)

func TestGenerateBundleInfoWithFileDir(t *testing.T) {

	files := map[string]string{
		"/fuz/data.json":   "[1,2,3]",
		"/fuz/fuz.rego":    "package fuz\np = 1",
		"/data.json":       `{"a": {"b": {"c": [123]}}}`,
		"/foo/policy.rego": "package foo\np = 1",
		"/baz/authz.rego":  "package foo\nx = 1",
		"base.rego":        "package bar\nx = 1",
		"/.manifest":       `{"roots": ["foo", "bar", "fuz", "baz", "a"], "revision": "rev"}`,
	}

	test.WithTempFS(files, func(rootDir string) {
		info, err := File(rootDir, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		expectedManifest := bundle.Manifest{
			Revision:      "rev",
			Roots:         &[]string{"foo", "bar", "fuz", "baz", "a"},
			WasmResolvers: nil,
			Metadata:      nil,
		}

		if !info.Manifest.Equal(expectedManifest) {
			t.Fatalf("expected manifest %v, but got: %v", expectedManifest, info.Manifest)
		}

		expectedNamespaces := map[string][]string{
			"data":     {filepath.Join(rootDir, "data.json")},
			"data.bar": {filepath.Join(rootDir, "base.rego")},
			"data.foo": {filepath.Join(rootDir, "baz/authz.rego"), filepath.Join(rootDir, "foo/policy.rego")},
			"data.fuz": {filepath.Join(rootDir, "fuz/fuz.rego"), filepath.Join(rootDir, "fuz/data.json")},
		}

		if !reflect.DeepEqual(info.Namespaces, expectedNamespaces) {
			t.Fatalf("expected namespaces %v, but got %v", expectedNamespaces, info.Namespaces)
		}
	})
}

func TestBundleInfoHasAnnotationLocationDataSet(t *testing.T) {

	files := map[string]string{
		"/fuz/fuz.rego": `# METADATA
# title: My package
package fuz

p = 1`,
	}

	test.WithTempFS(files, func(rootDir string) {
		info, err := File(rootDir, true)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if got, exp := len(info.Annotations), 1; got != exp {
			t.Fatalf("expected %d annotation, but got: %d", exp, got)
		}

		bs, err := json.Marshal(info.Annotations[0])
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		exp := fmt.Sprintf(`"location":{"file":"%s/fuz/fuz.rego","row":3,"col":1}`, rootDir)

		if got := string(bs); !strings.Contains(got, exp) {
			t.Fatalf("expected to find %q in %q", exp, got)
		}
	})
}

func TestGenerateBundleInfoWithFile(t *testing.T) {
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

		info, err := File(bundleFile, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if !info.Manifest.Equal(b.Manifest) {
			t.Fatalf("expected manifest %v, but got: %v", b.Manifest, info.Manifest)
		}

		expectedNamespaces := map[string][]string{
			"data":     {"/data.json"},
			"data.b.c": {"/policy.rego"},
		}

		if !reflect.DeepEqual(info.Namespaces, expectedNamespaces) {
			t.Fatalf("expected namespaces %v, but got %v", expectedNamespaces, info.Namespaces)
		}
	})
}

func TestGenerateBundleInfoWithBundleTarGz(t *testing.T) {
	signedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImZvbyJ9.eyJmaWxlcyI6W3sibmFtZSI6Ii5tYW5pZmVzdCIsImhhc2giOiI1MDdhMmMzOGExNDQxZGI1OGQyY2I4Nzk4MmM0MmFhOTFhNDM0MmVmNDIyYTZiNTQyZWRkZWJlZWY2ZjA0MTJmIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6ImEvYi9jL2RhdGEuanNvbiIsImhhc2giOiI0MmNmZTY3NjhiNTdiYjVmNzUwM2MxNjVjMjhkZDA3YWM1YjgxMzU1NGViYzg1MGYyY2MzNTg0M2U3MTM3YjFkIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6Imh0dHAvcG9saWN5L3BvbGljeS5yZWdvIiwiaGFzaCI6ImE2MTVlZWFlZTIxZGU1MTc5ZGUwODBkZThjMzA1MmM4ZGE5MDExMzg0MDZiYTcxYzM4YzAzMjg0NWY3ZDU0ZjQiLCJhbGdvcml0aG0iOiJTSEEtMjU2In1dLCJpYXQiOjE1OTIyNDgwMjcsImlzcyI6IkpXVFNlcnZpY2UiLCJrZXlpZCI6ImZvbyIsInNjb3BlIjoid3JpdGUifQ.grzWHYvyVS6LfWy0oiFTEJThKooOAwic8sexYaflzOM`

	files := [][2]string{
		{"/.signatures.json", fmt.Sprintf(`{"signatures": ["%v"]}`, signedTokenHS256)},
		{"/.manifest", `{"revision": "quickbrownfaux", "metadata": {"foo": "bar"}, "wasm": [{"entrypoint": "http/example/authz/allow", "module": "/policy.wasm"}, {"entrypoint": "http/example/foo/allow", "module": "/example/policy.wasm"}]}`},
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/a/b/y/data.yaml", `foo: 1`},
		{"/a/b/y/foo.rego", `package a.b.y`},
		{"/example/example.rego", `package example`},
		{"/example/policy.wasm", `modules-compiled-as-wasm-binary`},
		{"/policy.wasm", `modules-compiled-as-wasm-binary`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}`},
	}

	buf := archive.MustWriteTarGz(files)

	test.WithTempFS(nil, func(rootDir string) {
		bundleFile := filepath.Join(rootDir, "bundle.tar.gz")

		out, err := os.Create(bundleFile)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		_, err = out.Write(buf.Bytes())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		info, err := File(bundleFile, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		metadata := map[string]interface{}{"foo": "bar"}
		wasmResolvers := []bundle.WasmResolver{{
			Entrypoint: "http/example/authz/allow",
			Module:     "/policy.wasm",
		}, {
			Entrypoint: "http/example/foo/allow",
			Module:     "/example/policy.wasm"},
		}

		expectedManifest := bundle.Manifest{
			Revision:      "quickbrownfaux",
			WasmResolvers: wasmResolvers,
			Metadata:      metadata,
		}

		if !info.Manifest.Equal(expectedManifest) {
			t.Fatalf("expected manifest %v, but got: %v", expectedManifest, info.Manifest)
		}

		expectedNamespaces := map[string][]string{
			"data.example":                  {"/example/example.rego"},
			"data":                          {"/data.json"},
			"data.a.b.c":                    {"/a/b/c/data.json"},
			"data.a.b.d":                    {"/a/b/d/data.json"},
			"data.a.b.y":                    {"/a/b/y/foo.rego", "/a/b/y/data.yaml"},
			"data.http.example.authz.allow": {"/policy.wasm"},
			"data.http.example.foo.allow":   {"/example/policy.wasm"},
		}

		if !reflect.DeepEqual(info.Namespaces, expectedNamespaces) {
			t.Fatalf("expected namespaces %v, but got %v", expectedNamespaces, info.Namespaces)
		}

		expectedWasmModules := []map[string]interface{}{}
		expectedWasmModule1 := map[string]interface{}{
			"path":        "/example/policy.wasm",
			"url":         filepath.Join(bundleFile, "/example/policy.wasm"),
			"entrypoints": []string{"data.http.example.foo.allow"},
		}

		expectedWasmModule2 := map[string]interface{}{
			"path":        "/policy.wasm",
			"url":         filepath.Join(bundleFile, "policy.wasm"),
			"entrypoints": []string{"data.http.example.authz.allow"},
		}

		expectedWasmModules = append(expectedWasmModules, expectedWasmModule1, expectedWasmModule2)

		if !reflect.DeepEqual(info.WasmModules, expectedWasmModules) {
			t.Fatalf("expected wasm modules %v, but got %v", expectedWasmModules, info.WasmModules)
		}

		expectedSign := bundle.SignaturesConfig{
			Signatures: []string{signedTokenHS256},
		}

		if !reflect.DeepEqual(info.Signatures, expectedSign) {
			t.Fatalf("expected signature config %v, but got %v", expectedSign, info.Signatures)
		}
	})
}
