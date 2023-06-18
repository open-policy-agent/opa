// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package bundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/file/archive"
)

func TestManifestAddRoot(t *testing.T) {
	m := Manifest{Roots: &[]string{}}
	m.AddRoot("x/y")
	m.AddRoot("y/z")
	exp, act := stringSet{"x/y": struct{}{}, "y/z": struct{}{}}, m.rootSet()
	if !act.Equal(exp) {
		t.Fatalf("expected roots to be %v, got %v", exp, act)
	}
}

func TestManifestEqual(t *testing.T) {
	var m Manifest
	var n Manifest

	assertEqual := func() {
		t.Helper()
		if !m.Equal(n) {
			t.Fatal("expected manifests to be equal")
		}
	}

	assertNotEqual := func() {
		t.Helper()
		if m.Equal(n) {
			t.Fatal("expected manifests to be different")
		}
	}

	assertEqual()

	n.Revision = "xxx"
	assertNotEqual()

	m.Revision = "xxx"
	assertEqual()

	n.WasmResolvers = append(n.WasmResolvers, WasmResolver{})
	assertNotEqual()

	m.WasmResolvers = append(m.WasmResolvers, WasmResolver{})
	assertEqual()

	n.WasmResolvers[0].Module = "yyy"
	assertNotEqual()

	m.WasmResolvers[0].Module = "yyy"
	assertEqual()

	n.Metadata = map[string]interface{}{
		"foo": "bar",
	}
	assertNotEqual()

	m.Metadata = map[string]interface{}{
		"foo": "bar",
	}
	assertEqual()
}

func TestRead(t *testing.T) {
	for _, useMemoryFS := range []bool{false, true} {
		testReadBundle(t, "", useMemoryFS)
	}
}

func TestReadWithBaseDir(t *testing.T) {
	for _, useMemoryFS := range []bool{false, true} {
		testReadBundle(t, "/foo/bar", useMemoryFS)
	}
}

func TestReadWithSizeLimit(t *testing.T) {

	buf := archive.MustWriteTarGz([][2]string{
		{"data.json", `"foo"`},
	})

	loader := NewTarballLoaderWithBaseURL(buf, "")
	br := NewCustomReader(loader).WithSizeLimitBytes(4)

	_, err := br.Read()
	if err == nil || err.Error() != "bundle file 'data.json' exceeded max size (4 bytes)" {
		t.Fatal("expected error but got:", err)
	}

	buf = archive.MustWriteTarGz([][2]string{
		{".signatures.json", `"foo"`},
	})

	loader = NewTarballLoaderWithBaseURL(buf, "")
	br = NewCustomReader(loader).WithSizeLimitBytes(4)

	_, err = br.Read()
	if err == nil || err.Error() != "bundle file '.signatures.json' exceeded max size (4 bytes)" {
		t.Fatal("expected error but got:", err)
	}
}

func TestReadBundleInLazyMode(t *testing.T) {
	files := [][2]string{
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/a/b/y/data.yaml", `foo: 1`},
		{"/example/example.rego", `package example`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}`},
		{"/.manifest", `{"revision": "foo", "roots": ["example"]}`}, // data is outside roots but validation skipped in lazy mode
	}

	buf := archive.MustWriteTarGz(files)
	loader := NewTarballLoaderWithBaseURL(buf, "")
	br := NewCustomReader(loader).WithLazyLoadingMode(true)

	bundle, err := br.Read()
	if err != nil {
		t.Fatal(err)
	}

	if len(bundle.Data) != 0 {
		t.Fatal("expected the bundle object to contain no data")
	}

	if len(bundle.Raw) == 0 {
		t.Fatal("raw bundle bytes not set on bundle object")
	}
}

func TestReadWithBundleEtag(t *testing.T) {

	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
	}

	buf := archive.MustWriteTarGz(files)
	bundle, err := NewReader(buf).WithBundleEtag("foo").Read()
	if err != nil {
		t.Fatal(err)
	}

	if bundle.Etag != "foo" {
		t.Fatalf("Expected bundle etag foo but got %v\n", bundle.Etag)
	}
}

func testReadBundle(t *testing.T, baseDir string, useMemoryFS bool) {
	module := `package example`
	if useMemoryFS && baseDir == "" {
		baseDir = "."
	}

	modulePath := "/example/example.rego"
	if baseDir != "" {
		modulePath = filepath.Join(baseDir, modulePath)
	}

	legacyWasmModulePath := "/policy.wasm"
	if baseDir != "" {
		legacyWasmModulePath = filepath.Join(baseDir, legacyWasmModulePath)
	}

	wasmResolverPath := "/authz/allow/policy.wasm"
	fullWasmResolverPath := wasmResolverPath
	if baseDir != "" {
		fullWasmResolverPath = filepath.Join(baseDir, wasmResolverPath)
	}

	files := [][2]string{
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/a/b/y/data.yaml", `foo: 1`},
		{"/a/b/g/data.yml", "1"},
		{"/example/example.rego", `package example`},
		{"/policy.wasm", `legacy-wasm-module`},
		{wasmResolverPath, `wasm-module`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}`},
		{"/.manifest", fmt.Sprintf(`{"wasm":[{"entrypoint": "authz/allow", "module": "%s"}]}`, fullWasmResolverPath)},
	}

	buf := archive.MustWriteTarGz(files)
	var loader DirectoryLoader
	if useMemoryFS {
		fsys := make(fstest.MapFS, 1)
		fsys["test.tar"] = &fstest.MapFile{Data: buf.Bytes()}
		fh, err := fsys.Open("test.tar")
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		loader = NewTarballLoaderWithBaseURL(fh, baseDir)
	} else {
		loader = NewTarballLoaderWithBaseURL(buf, baseDir)
	}
	br := NewCustomReader(loader).WithBaseDir(baseDir)

	bundle, err := br.Read()
	if err != nil {
		t.Fatal(err)
	}

	expManifest := Manifest{}
	expManifest.Init()
	expManifest.WasmResolvers = []WasmResolver{
		{
			Entrypoint: "authz/allow",
			Module:     fullWasmResolverPath,
		},
	}

	exp := Bundle{
		Manifest: expManifest,
		Data: map[string]interface{}{
			"a": map[string]interface{}{
				"b": map[string]interface{}{
					"c": []interface{}{json.Number("1"), json.Number("2"), json.Number("3")},
					"d": true,
					"g": json.Number("1"),
					"y": map[string]interface{}{
						"foo": json.Number("1"),
					},
					"z": true,
				},
			},
			"x": map[string]interface{}{
				"y": true,
			},
		},
		Modules: []ModuleFile{
			{
				URL:    modulePath,
				Path:   modulePath,
				Parsed: ast.MustParseModule(module),
				Raw:    []byte(module),
			},
		},
		WasmModules: []WasmModuleFile{
			{
				URL:  legacyWasmModulePath,
				Path: legacyWasmModulePath,
				Raw:  []byte(`legacy-wasm-module`),
			},
			{
				URL:         fullWasmResolverPath,
				Path:        fullWasmResolverPath,
				Raw:         []byte("wasm-module"),
				Entrypoints: []ast.Ref{ast.MustParseRef("data.authz.allow")},
			},
		},
	}

	if !exp.Equal(bundle) {
		t.Fatalf("\nExp: %+v\nGot: %+v", exp, bundle)
	}
}

func TestReadWithManifest(t *testing.T) {
	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
	}
	buf := archive.MustWriteTarGz(files)
	bundle, err := NewReader(buf).Read()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Manifest.Revision != "quickbrownfaux" {
		t.Fatalf("Unexpected manifest.revision value: %v", bundle.Manifest.Revision)
	}
}

func TestManifestMetadata(t *testing.T) {
	files := [][2]string{
		{"/.manifest", `{
			"metadata": {
				"foo": {
					"version": "1.0.0"
				}
			}
		}`},
	}
	buf := archive.MustWriteTarGz(files)
	bundle, err := NewReader(buf).Read()
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Manifest.Metadata["foo"] == nil {
		t.Fatal("Unexpected nil metadata key")
	}
	data, ok := bundle.Manifest.Metadata["foo"].(map[string]interface{})
	if !ok {
		t.Fatal("Unexpected structure in metadata")
	}
	if data["version"] != "1.0.0" {
		t.Fatalf("Unexpected metadata value: %v", data["version"])
	}
}

func TestReadWithManifestInData(t *testing.T) {
	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
	}
	buf := archive.MustWriteTarGz(files)
	bundle, err := NewReader(buf).IncludeManifestInData(true).Read()
	if err != nil {
		t.Fatal(err)
	}

	system := bundle.Data["system"].(map[string]interface{})
	b := system["bundle"].(map[string]interface{})
	m := b["manifest"].(map[string]interface{})

	if m["revision"] != "quickbrownfaux" {
		t.Fatalf("Unexpected manifest.revision value: %v. Expected: %v", m["revision"], "quickbrownfaux")
	}
}

func TestReadWithSignaturesSkipVerify(t *testing.T) {
	signedBadTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJmaWxlcyI6W3sibmFtZSI6Ii5tYW5pZmVzdCIsImhhc2giOiI2MDdhMmMzOGExNDQxZGI1OGQyY2I4Nzk4MmM0MmFhOTFhNDM0MmVmNDIyYTZiNTQyZWRkZWJlZWY2ZjA0MTJmIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6ImEvYi9jL2RhdGEuanNvbiIsImhhc2giOiI0MmNmZTY3NjhiNTdiYjVmNzUwM2MxNjVjMjhkZDA3YWM1YjgxMzU1NGViYzg1MGYyY2MzNTg0M2U3MTM3YjFkIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6Imh0dHAvcG9saWN5L3BvbGljeS5yZWdvIiwiaGFzaCI6ImE2MTVlZWFlZTIxZGU1MTc5ZGUwODBkZThjMzA1MmM4ZGE5MDExMzg0MDZiYTcxYzM4YzAzMjg0NWY3ZDU0ZjQiLCJhbGdvcml0aG0iOiJTSEEtMjU2In1dLCJpYXQiOjE1OTIyNDgwMjcsImlzcyI6IkpXVFNlcnZpY2UiLCJrZXlpZCI6ImZvbyIsInNjb3BlIjoid3JpdGUifQ.sQTuw9tBp6DvvQG-MXSxTzJA3hSnKYxjX5fnxiR22JA`

	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
		{"/.signatures.json", fmt.Sprintf(`{"signatures": ["%v"]}`, signedBadTokenHS256)},
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/http/policy/policy.rego", `package example`},
	}

	vc := NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", nil)

	buf := archive.MustWriteTarGz(files)

	loader := NewTarballLoaderWithBaseURL(buf, "/foo/bar")
	reader := NewCustomReader(loader).WithBaseDir("/foo/bar").WithBundleVerificationConfig(vc).WithSkipBundleVerification(true)
	_, err := reader.Read()
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
}

func TestReadWithSignatures(t *testing.T) {

	signedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImZvbyJ9.eyJmaWxlcyI6W3sibmFtZSI6Ii5tYW5pZmVzdCIsImhhc2giOiI1MDdhMmMzOGExNDQxZGI1OGQyY2I4Nzk4MmM0MmFhOTFhNDM0MmVmNDIyYTZiNTQyZWRkZWJlZWY2ZjA0MTJmIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6ImEvYi9jL2RhdGEuanNvbiIsImhhc2giOiI0MmNmZTY3NjhiNTdiYjVmNzUwM2MxNjVjMjhkZDA3YWM1YjgxMzU1NGViYzg1MGYyY2MzNTg0M2U3MTM3YjFkIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6Imh0dHAvcG9saWN5L3BvbGljeS5yZWdvIiwiaGFzaCI6ImE2MTVlZWFlZTIxZGU1MTc5ZGUwODBkZThjMzA1MmM4ZGE5MDExMzg0MDZiYTcxYzM4YzAzMjg0NWY3ZDU0ZjQiLCJhbGdvcml0aG0iOiJTSEEtMjU2In1dLCJpYXQiOjE1OTIyNDgwMjcsImlzcyI6IkpXVFNlcnZpY2UiLCJrZXlpZCI6ImZvbyIsInNjb3BlIjoid3JpdGUifQ.grzWHYvyVS6LfWy0oiFTEJThKooOAwic8sexYaflzOM`
	otherSignedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImZvbyJ9.eyJmaWxlcyI6W3sibmFtZSI6ImEvYi9jL2RhdGEuanNvbiIsImhhc2giOiJmOWNhYzA3MTQ3MDVkMjBkMWEyMDg4MDE4NWNkZWQ2ZTBmNmQwNDA2NjJkMmViYjA5NjFkM2Q5ZjMxN2Q4YWNiIn1dLCJpYXQiOjE1OTIyNDgwMjcsImlzcyI6IkpXVFNlcnZpY2UiLCJzY29wZSI6IndyaXRlIn0.WJhnUjwaVvckSgOd4QcVvKThN6oc99NiPiwHKYnoG7c`
	defaultSigner, _ := GetSigner(defaultSignerID)
	defaultVerifier, _ := GetVerifier(defaultVerifierID)
	if err := RegisterSigner("_bar", defaultSigner); err != nil {
		t.Fatal(err)
	}
	if err := RegisterVerifier("_bar", defaultVerifier); err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		files   [][2]string
		vc      *VerificationConfig
		wantErr bool
		err     error
	}{
		"no_signature_verification_config": {
			[][2]string{{"/.signatures.json", `{"signatures": []}`}},
			nil,
			true, fmt.Errorf("verification key not provided"),
		},
		"no_signatures_file_no_keyid": {
			[][2]string{{"/.manifest", `{"revision": "quickbrownfaux"}`}},
			NewVerificationConfig(map[string]*KeyConfig{}, "", "", nil),
			false, nil,
		},
		"no_signatures_file": {
			[][2]string{{"/.manifest", `{"revision": "quickbrownfaux"}`}},
			NewVerificationConfig(map[string]*KeyConfig{}, "somekey", "", nil),
			true, fmt.Errorf("bundle missing .signatures.json file"),
		},
		"no_signatures": {
			[][2]string{{"/.signatures.json", `{"signatures": []}`}},
			NewVerificationConfig(map[string]*KeyConfig{}, "", "", nil),
			true, fmt.Errorf(".signatures.json: missing JWT (expected exactly one)"),
		},
		"digest_mismatch": {
			[][2]string{
				{"/.signatures.json", fmt.Sprintf(`{"signatures": ["%v"]}`, signedTokenHS256)},
				{"/a/b/c/data.json", "[1,2,3]"},
				{"/.manifest", `{"revision": "quickbrownfaux"}`},
			},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", nil),
			true, fmt.Errorf("a/b/c/data.json: digest mismatch (want: 42cfe6768b57bb5f7503c165c28dd07ac5b813554ebc850f2cc35843e7137b1d, got: a615eeaee21de5179de080de8c3052c8da901138406ba71c38c032845f7d54f4)"),
		},
		"no_hashing_alg": {
			[][2]string{
				{"/.signatures.json", fmt.Sprintf(`{"signatures": ["%v"]}`, otherSignedTokenHS256)},
				{"/a/b/c/data.json", "[1,2,3]"},
			},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", nil),
			true, fmt.Errorf("no hashing algorithm provided for file a/b/c/data.json"),
		},
		"exclude_files": {
			[][2]string{
				{"/.signatures.json", fmt.Sprintf(`{"signatures": ["%v"]}`, signedTokenHS256)},
				{"/.manifest", `{"revision": "quickbrownfaux"}`},
				{"/a/b/c/data.json", "[1,2,3]"},
				{"/http/policy/policy.rego", `package example`},
			},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", []string{".*", "a/b/c/data.json", "http/policy/policy.rego"}),
			false, nil,
		},
		"customer_signer_verifier": {
			[][2]string{
				{"/.signatures.json", fmt.Sprintf(`{"signatures": ["%v"],"plugin":"_bar"}`, signedTokenHS256)},
				{"/.manifest", `{"revision": "quickbrownfaux"}`},
				{"/a/b/c/data.json", "[1,2,3]"},
				{"/http/policy/policy.rego", `package example`},
			},
			NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", []string{".*", "a/b/c/data.json", "http/policy/policy.rego"}),
			false, nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			buf := archive.MustWriteTarGz(tc.files)
			reader := NewReader(buf).WithBundleVerificationConfig(tc.vc)
			_, err := reader.Read()

			if tc.wantErr {
				if err == nil {
					t.Fatal("Expected error but got nil")
				}

				if tc.err != nil && tc.err.Error() != err.Error() {
					t.Fatalf("Expected error message %v but got %v", tc.err.Error(), err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error %v", err)
				}
			}
		})
	}
}

func TestReadWithSignaturesWithBaseDir(t *testing.T) {
	signedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImZvbyJ9.eyJmaWxlcyI6W3sibmFtZSI6ImZvby9iYXIvLm1hbmlmZXN0IiwiaGFzaCI6IjUwN2EyYzM4YTE0NDFkYjU4ZDJjYjg3OTgyYzQyYWE5MWE0MzQyZWY0MjJhNmI1NDJlZGRlYmVlZjZmMDQxMmYiLCJhbGdvcml0aG0iOiJTSEEtMjU2In0seyJuYW1lIjoiZm9vL2Jhci9hL2IvYy9kYXRhLmpzb24iLCJoYXNoIjoiYTYxNWVlYWVlMjFkZTUxNzlkZTA4MGRlOGMzMDUyYzhkYTkwMTEzODQwNmJhNzFjMzhjMDMyODQ1ZjdkNTRmNCIsImFsZ29yaXRobSI6IlNIQS0yNTYifSx7Im5hbWUiOiJmb28vYmFyL2h0dHAvcG9saWN5L3BvbGljeS5yZWdvIiwiaGFzaCI6ImY2NjQ0NjFlMzAzYjM3YzIwYzVlMGJlMjkwMDg4MTY3OGNkZjhlODYwYWE0MzNhNWExNGQ0OTRiYTNjNjY2NDkiLCJhbGdvcml0aG0iOiJTSEEtMjU2In1dLCJpYXQiOjE1OTIyNDgwMjcsImlzcyI6IkpXVFNlcnZpY2UiLCJzY29wZSI6IndyaXRlIn0.qTHkuBDVuT-Zl5pbJdZ6LoJ9eooFOhhpRdCheauDrlA`

	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
		{"/.signatures.json", fmt.Sprintf(`{"signatures": ["%v"]}`, signedTokenHS256)},
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/http/policy/policy.rego", `package example`},
	}

	vc := NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", nil)

	buf := archive.MustWriteTarGz(files)

	loader := NewTarballLoaderWithBaseURL(buf, "/foo/bar")
	reader := NewCustomReader(loader).WithBaseDir("/foo/bar").WithBundleVerificationConfig(vc)
	_, err := reader.Read()
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
}

func TestReadWithPatch(t *testing.T) {
	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux",  "roots": ["a"]}`},
		{"/patch.json", `{"data": [{"op": "add", "path": "/a/b/d", "value": "foo"}, {"op": "remove", "path": "a/b/c"}]}`},
	}

	buf := archive.MustWriteTarGz(files)

	loader := NewTarballLoaderWithBaseURL(buf, "/foo/bar")
	reader := NewCustomReader(loader).WithBaseDir("/foo/bar")
	b, err := reader.Read()
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	actual := b.Type()
	if actual != DeltaBundleType {
		t.Fatalf("Expected delta bundle but got %v", actual)
	}

	if len(b.Patch.Data) != 2 {
		t.Fatalf("Expected two patch operations but got %v", len(b.Patch.Data))
	}

	p1 := PatchOperation{
		Op:    "add",
		Path:  "/a/b/d",
		Value: "foo",
	}

	p2 := PatchOperation{
		Op:   "remove",
		Path: "a/b/c",
	}

	expected := Patch{Data: []PatchOperation{p1, p2}}

	if !reflect.DeepEqual(b.Patch.Data, expected.Data) {
		t.Fatalf("Expected patch %v but got %v", expected.Data, b.Patch.Data)
	}
}

func TestReadWithPatchExtraFiles(t *testing.T) {
	cases := []struct {
		note  string
		files [][2]string
		err   string
	}{
		{
			note: "extra data file",
			files: [][2]string{
				{"/.manifest", `{"revision": "quickbrownfaux",  "roots": ["a"]}`},
				{"/patch.json", `{"data": [{"op": "add", "path": "/a/b/d", "value": "foo"}, {"op": "remove", "path": "a/b/c"}]}`},
				{"/a/b/c/data.json", "[1,2,3]"},
			},
			err: "delta bundle expected to contain only patch file but data files found",
		},
		{
			note: "extra policy file",
			files: [][2]string{
				{"/.manifest", `{"revision": "quickbrownfaux",  "roots": ["a"]}`},
				{"/patch.json", `{"data": [{"op": "add", "path": "/a/b/d", "value": "foo"}, {"op": "remove", "path": "a/b/c"}]}`},
				{"/http/policy/policy.rego", `package example`},
			},
			err: "delta bundle expected to contain only patch file but policy files found",
		},
		{
			note: "extra wasm file",
			files: [][2]string{
				{"/.manifest", `{"revision": "quickbrownfaux",  "roots": ["a"]}`},
				{"/patch.json", `{"data": [{"op": "add", "path": "/a/b/d", "value": "foo"}, {"op": "remove", "path": "a/b/c"}]}`},
				{"/policy.wasm", `modules-compiled-as-wasm-binary`},
			},
			err: "delta bundle expected to contain only patch file but wasm files found",
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			buf := archive.MustWriteTarGz(tc.files)
			loader := NewTarballLoaderWithBaseURL(buf, "/foo/bar")
			reader := NewCustomReader(loader).WithBaseDir("/foo/bar")
			_, err := reader.Read()
			if tc.err == "" && err != nil {
				t.Fatal("Unexpected error occurred:", err)
			} else if tc.err != "" && err == nil {
				t.Fatal("Expected error but got success")
			} else if tc.err != "" && err != nil {
				if tc.err != err.Error() {
					t.Fatalf("Expected error to contain %q but got: %v", tc.err, err)
				}
			}
		})
	}

}

func TestReadWithPatchPersistProperty(t *testing.T) {
	cases := []struct {
		note    string
		files   [][2]string
		persist bool
		err     string
	}{
		{
			note: "persist true property",
			files: [][2]string{
				{"/patch.json", `{"data": [{"op": "add", "path": "/a/b/d", "value": "foo"}, {"op": "remove", "path": "a/b/c"}]}`},
			},
			persist: true,
			err:     "'persist' property is true in config. persisting delta bundle to disk is not supported",
		},
		{
			note: "persist false property",
			files: [][2]string{
				{"/patch.json", `{"data": [{"op": "add", "path": "/a/b/d", "value": "foo"}, {"op": "remove", "path": "a/b/c"}]}`},
			},
			persist: false,
			err:     "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			buf := archive.MustWriteTarGz(tc.files)
			loader := NewTarballLoaderWithBaseURL(buf, "/foo/bar")
			reader := NewCustomReader(loader).
				WithBundlePersistence(tc.persist).WithBaseDir("/foo/bar")
			_, err := reader.Read()
			if tc.err == "" && err != nil {
				t.Fatal("Unexpected error occurred:", err)
			} else if tc.err != "" && err == nil {
				t.Fatal("Expected error but got success")
			} else if tc.err != "" && err != nil {
				if tc.err != err.Error() {
					t.Fatalf("Expected error to contain %q but got: %v", tc.err, err)
				}
			}
		})
	}
}

func TestReadWithSignaturesExtraFiles(t *testing.T) {
	signedTokenHS256 := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImZvbyJ9.eyJmaWxlcyI6W3sibmFtZSI6Ii5tYW5pZmVzdCIsImhhc2giOiI1MDdhMmMzOGExNDQxZGI1OGQyY2I4Nzk4MmM0MmFhOTFhNDM0MmVmNDIyYTZiNTQyZWRkZWJlZWY2ZjA0MTJmIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6ImEvYi9jL2RhdGEuanNvbiIsImhhc2giOiI0MmNmZTY3NjhiNTdiYjVmNzUwM2MxNjVjMjhkZDA3YWM1YjgxMzU1NGViYzg1MGYyY2MzNTg0M2U3MTM3YjFkIiwiYWxnb3JpdGhtIjoiU0hBLTI1NiJ9LHsibmFtZSI6Imh0dHAvcG9saWN5L3BvbGljeS5yZWdvIiwiaGFzaCI6ImE2MTVlZWFlZTIxZGU1MTc5ZGUwODBkZThjMzA1MmM4ZGE5MDExMzg0MDZiYTcxYzM4YzAzMjg0NWY3ZDU0ZjQiLCJhbGdvcml0aG0iOiJTSEEtMjU2In1dLCJpYXQiOjE1OTIyNDgwMjcsImlzcyI6IkpXVFNlcnZpY2UiLCJzY29wZSI6IndyaXRlIn0.Vmm9UDiInUnXXlk-OOjiCy3rR7EVvXS-OFst1rbh3Zo`

	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
		{"/.signatures.json", fmt.Sprintf(`{"signatures": ["%v"]}`, signedTokenHS256)},
	}

	vc := NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "", "write", nil)

	buf := archive.MustWriteTarGz(files)
	reader := NewReader(buf).WithBundleVerificationConfig(vc)
	_, err := reader.Read()
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	expected := []string{
		"file(s) [a/b/c/data.json http/policy/policy.rego] specified in bundle signatures but not found in the target bundle",
		"file(s) [http/policy/policy.rego a/b/c/data.json] specified in bundle signatures but not found in the target bundle",
	}

	var found bool
	if err.Error() == expected[0] || err.Error() == expected[1] {
		found = true
	}

	if !found {
		t.Fatalf("Expected error message to be one of %v but got %v", expected, err.Error())
	}
}

func TestVerifyBundleFileHash(t *testing.T) {
	// add files to the bundle and reader
	// compare the hash the for target files
	files := [][2]string{
		{"/.manifest", `{"revision": "quickbrownfaux"}`},
		{"/a/b/c/data.json", "[1,2,3]"},
		{"/a/b/d/data.json", "true"},
		{"/a/b/y/data.yaml", `foo: 1`},
		{"/example/example.rego", `package example`},
		{"/policy.wasm", `modules-compiled-as-wasm-binary`},
		{"/data.json", `{"x": {"y": true}, "a": {"b": {"z": true}}}`},
	}

	buf := archive.MustWriteTarGz(files)
	reader := NewReader(buf)
	reader.files = map[string]FileInfo{}

	expDigests := make([]string, len(files))
	expDigests[0] = "a005c38a509dc2d5a7407b9494efb2ad"
	expDigests[1] = "60f7b5dc86ded48785436192a08dbfd04894d7f1b417c4f8d3714679a7f78cb3c833f16a8559a1cf1f32968747dc1d95ef34826263dacf125ded8f5c374be4c0"
	expDigests[2] = "b326b5062b2f0e69046810717534cb09"
	expDigests[3] = "20f27a640a233e6524fe7d138898583cd43475724806feb26be7f214e1d10b29edf6a0d3cb08f82107a45686b61b8fdabab6406cf4e70efe134f42238dbd70ab"
	expDigests[4] = "ceecc199d432a4eeae305914ea4816cb"
	expDigests[5] = "4f73765168fd8b5c294b739436da312cc5e979faf09f67bf576d36ea79a4f79c70cbb3c33d06ff65f531a9f42abd0a8f4daacc554cb521837e876dc28f56ce89"
	expDigests[6] = "36669864a622563256817033b1fc53db"

	// populate the files on the reader
	// this simulates the files seen by the reader after
	// decoding the signatures in the "signatures.json" file
	for i, f := range files {
		file := FileInfo{
			Name: f[0],
			Hash: expDigests[i],
		}

		if i%2 == 0 {
			file.Algorithm = MD5.String()
		} else {
			file.Algorithm = SHA512.String()
		}

		reader.files[f[0]] = file
	}

	for _, f := range files {
		buf := bytes.NewBufferString(f[1])
		err := reader.verifyBundleFile(f[0], *buf)
		if err != nil {
			t.Fatal(err)
		}
	}

	// check there are no files left on the reader
	if len(reader.files) != 0 {
		t.Fatalf("Expected no files on the reader but got %v", len(reader.files))
	}
}

func TestIsFileExcluded(t *testing.T) {
	cases := []struct {
		note    string
		file    string
		pattern []string
		exp     bool
	}{
		{
			note:    "exact",
			file:    "data.json",
			pattern: []string{"data.json"},
			exp:     true,
		},
		{
			note:    "hidden",
			file:    ".manifest",
			pattern: []string{".*"},
			exp:     true,
		},
		{
			note:    "no_match",
			file:    "data.json",
			pattern: []string{".*"},
			exp:     false,
		},
		{
			note:    "dir_match",
			file:    "/a/b/data.json",
			pattern: []string{"/a/b/*"},
			exp:     true,
		},
		{
			note:    "dir_no_match",
			file:    "/a/b/c/data.json",
			pattern: []string{"/a/b/*"},
			exp:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {

			buf := archive.MustWriteTarGz([][2]string{})
			vc := NewVerificationConfig(map[string]*KeyConfig{}, "", "", tc.pattern)
			reader := NewReader(buf).WithBundleVerificationConfig(vc)
			actual := reader.isFileExcluded(tc.file)

			if actual != tc.exp {
				t.Fatalf("Expected file exclude result for %v %v but got %v", tc.file, tc.exp, actual)
			}
		})
	}
}

func TestReadRootValidation(t *testing.T) {
	cases := []struct {
		note  string
		files [][2]string
		err   string
	}{
		{
			note: "default full extent",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd"}`},
				{"/data.json", `{"a": 1}`},
				{"/x.rego", `package foo`},
			},
			err: "",
		},
		{
			note: "explicit full extent",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": [""]}`},
				{"/data.json", `{"a": 1}`},
				{"/x.rego", `package foo`},
			},
			err: "",
		},
		{
			note: "implicit prefixed",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a/b", "foo"]}`},
				{"/data.json", `{"a": {"b": 1}}`},
				{"/x.rego", `package foo.bar`},
			},
			err: "",
		},
		{
			note: "err empty",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": []}`},
				{"/x.rego", `package foo`},
			},
			err: "manifest roots [] do not permit 'package foo' in module '/x.rego'",
		},
		{
			note: "err overlapped",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a/b", "a"]}`},
			},
			err: "manifest has overlapped roots: 'a/b' and 'a'",
		},
		{
			note: "edge overlapped partial segment",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a", "another_root"]}`},
			},
			err: "",
		},
		{
			note: "err package outside scope",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a", "b", "c/d"]}`},
				{"/a.rego", `package b.c`},
				{"/x.rego", `package c.e`},
			},
			err: "manifest roots [a b c/d] do not permit 'package c.e' in module '/x.rego'",
		},
		{
			note: "err data outside scope",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a", "b", "c/d"]}`},
				{"/data.json", `{"a": 1}`},
				{"/c/e/data.json", `"bad bad bad"`},
			},
			err: "manifest roots [a b c/d] do not permit data at path '/c/e'",
		},
		{
			note: "err data patch outside scope",
			files: [][2]string{
				{"/.manifest", `{"revision": "abcd", "roots": ["a", "b", "c/d"]}`},
				{"/patch.json", `{"data": [{"op": "add", "path": "/a/b/d", "value": "foo"}, {"op": "remove", "path": "/c/e"}]}`},
			},
			err: "manifest roots [a b c/d] do not permit data patch at path 'c/e'",
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			buf := archive.MustWriteTarGz(tc.files)
			_, err := NewReader(buf).IncludeManifestInData(true).Read()
			if tc.err == "" && err != nil {
				t.Fatal("Unexpected error occurred:", err)
			} else if tc.err != "" && err == nil {
				t.Fatal("Expected error but got success")
			} else if tc.err != "" && err != nil {
				if !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("Expected error to contain %q but got: %v", tc.err, err)
				}
			}
		})
	}
}

func TestRootPathsContain(t *testing.T) {
	tests := []struct {
		note  string
		roots []string
		path  string
		want  bool
	}{
		{
			note:  "empty contains empty",
			roots: []string{""},
			path:  "",
			want:  true,
		},
		{
			note:  "empty contains non-empty",
			roots: []string{""},
			path:  "foo/bar",
			want:  true,
		},
		{
			note:  "single prefix",
			roots: []string{"foo"},
			path:  "foo/bar",
			want:  true,
		},
		{
			note:  "single prefix no match",
			roots: []string{"bar"},
			path:  "foo/bar",
			want:  false,
		},
		{
			note:  "multiple prefix",
			roots: []string{"baz", "foo"},
			path:  "foo/bar",
			want:  true,
		},
		{
			note:  "multiple prefix no match",
			roots: []string{"baz", "qux"},
			path:  "foo/bar",
			want:  false,
		},
		{
			note:  "single exact",
			roots: []string{"foo/bar"},
			path:  "foo/bar",
			want:  true,
		},
		{
			note:  "single exact no match",
			roots: []string{"foo/ba"},
			path:  "foo/bar",
			want:  false,
		},
		{
			note:  "multiple exact",
			roots: []string{"baz/bar", "foo/bar"},
			path:  "foo/bar",
			want:  true,
		},
		{
			note:  "root too long",
			roots: []string{"foo/bar/"},
			path:  "foo/bar",
			want:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			if RootPathsContain(tc.roots, tc.path) != tc.want {
				t.Fatalf("expected %v contains %v to be %v", tc.roots, tc.path, tc.want)
			}
		})
	}
}

func TestReadErrorBadGzip(t *testing.T) {
	buf := bytes.NewBufferString("bad gzip bytes")
	_, err := NewReader(buf).Read()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadErrorBadTar(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	_, _ = gw.Write([]byte("bad tar bytes"))
	_ = gw.Close()
	_, err := NewReader(&buf).Read()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadErrorBadContents(t *testing.T) {
	tests := []struct {
		files [][2]string
	}{
		{[][2]string{{"/test.rego", "lkafjasdkljf"}}},
		{[][2]string{{"/data.json", "lskjafkljsdf"}}},
		{[][2]string{{"/data.json", "[1,2,3]"}}},
		{[][2]string{
			{"/a/b/data.json", "[1,2,3]"},
			{"a/b/c/data.json", "true"},
		}},
		{[][2]string{{"/test.rego", ""}}},
		{[][2]string{
			{"/a/b/data.json", `{"c": "foo"}`},
			{"/data.json", `{"a": {"b": {"c": [123]}}}`},
		}},
	}
	for _, test := range tests {
		buf := archive.MustWriteTarGz(test.files)
		_, err := NewReader(buf).Read()
		if err == nil {
			t.Fatal("expected error")
		}
	}

}

func TestRoundtripDeprecatedWrite(t *testing.T) {

	bundle := Bundle{
		Data: map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": []interface{}{json.Number("1"), json.Number("2"), json.Number("3")},
				"baz": true,
				"qux": "hello",
			},
		},
		Modules: []ModuleFile{
			{
				URL:    "/foo/corge/corge.rego",
				Path:   "/foo/corge/corge.rego",
				Parsed: ast.MustParseModule(`package foo.corge`),
				Raw:    []byte(`package foo.corge`),
			},
		},
		WasmModules: []WasmModuleFile{
			{
				Path: "/policy.wasm",
				URL:  "/policy.wasm",
				Raw:  []byte("modules-compiled-as-wasm-binary"),
			},
		},
		Manifest: Manifest{
			Revision: "quickbrownfaux",
		},
	}

	var buf bytes.Buffer

	if err := Write(&buf, bundle); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	bundle2, err := NewReader(&buf).Read()
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if !bundle2.Equal(bundle) {
		t.Fatalf("\nExp: %+v\nGot: %+v", bundle, bundle2)
	}

}

func TestRoundtrip(t *testing.T) {

	bundle := Bundle{
		Data: map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": []interface{}{json.Number("1"), json.Number("2"), json.Number("3")},
				"baz": true,
				"qux": "hello",
			},
		},
		Modules: []ModuleFile{
			{
				URL:    "/foo/corge/corge.rego",
				Path:   "/foo/corge/corge.rego",
				Parsed: ast.MustParseModule(`package foo.corge`),
				Raw:    []byte("package foo.corge\n"),
			},
		},
		WasmModules: []WasmModuleFile{
			{
				Path: "/policy.wasm",
				URL:  "/policy.wasm",
				Raw:  []byte("modules-compiled-as-wasm-binary"),
			},
		},
		Manifest: Manifest{
			Roots:    &[]string{""},
			Revision: "quickbrownfaux",
			Metadata: map[string]interface{}{"version": "v1", "hello": "world"},
		},
	}

	if err := bundle.GenerateSignature(NewSigningConfig("secret", "HS256", ""), "foo", false); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	var buf bytes.Buffer

	if err := NewWriter(&buf).Write(bundle); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	vc := NewVerificationConfig(map[string]*KeyConfig{"foo": {Key: "secret", Algorithm: "HS256"}}, "foo", "", nil)

	bundle2, err := NewReader(&buf).WithBundleVerificationConfig(vc).Read()
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if !bundle2.Equal(bundle) {
		t.Fatal("Exp:", bundle, "\n\nGot:", bundle2)
	}

	if !reflect.DeepEqual(bundle2.Signatures, bundle.Signatures) {
		t.Fatal("Expected signatures to be same")
	}
}

func TestRoundtripWithPlanModules(t *testing.T) {

	b := Bundle{
		Data: map[string]interface{}{},
		PlanModules: []PlanModuleFile{
			{
				URL:  "/plan.json",
				Path: "/plan.json",
				Raw:  []byte(`{"foo": 7}`), // NOTE(tsandall): contents are ignored
			},
		},
	}

	var buf bytes.Buffer
	if err := Write(&buf, b); err != nil {
		t.Fatal(err)
	}

	b2, err := NewReader(&buf).Read()
	if err != nil {
		t.Fatal(err)
	}

	if len(b2.PlanModules) != 1 ||
		b2.PlanModules[0].Path != b.PlanModules[0].Path ||
		b2.PlanModules[0].URL != b.PlanModules[0].URL ||
		!bytes.Equal(b2.PlanModules[0].Raw, b.PlanModules[0].Raw) {
		t.Fatalf("expected %+v but got %+v", b, b2)
	}
}

func TestRoundtripDeltaBundle(t *testing.T) {

	// replace a value
	p1 := PatchOperation{
		Op:    "replace",
		Path:  "a/baz",
		Value: "bux",
	}

	// add a new object member
	p2 := PatchOperation{
		Op:    "add",
		Path:  "/a/foo",
		Value: []string{"hello", "world"},
	}

	bundle := Bundle{
		Patch: Patch{Data: []PatchOperation{p1, p2}},
		Manifest: Manifest{
			Revision: "delta",
			Roots:    &[]string{"a"},
		},
	}

	var buf bytes.Buffer

	if err := NewWriter(&buf).Write(bundle); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	bundle2, err := NewReader(&buf).Read()
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if !bundle2.Equal(bundle) {
		t.Fatal("Exp:", bundle, "\n\nGot:", bundle2)
	}
}

func TestWriterUsePath(t *testing.T) {

	bundle := Bundle{
		Data: map[string]interface{}{},
		Modules: []ModuleFile{
			{
				URL:    "/url.rego",
				Path:   "/path.rego",
				Parsed: ast.MustParseModule(`package x`),
				Raw:    []byte("package x\n"),
			},
		},
		Manifest: Manifest{Revision: "quickbrownfaux"},
	}

	var buf bytes.Buffer

	if err := NewWriter(&buf).UseModulePath(true).Write(bundle); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	bundle2, err := NewReader(&buf).Read()
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if bundle2.Modules[0].URL != "/path.rego" || bundle2.Modules[0].Path != "/path.rego" {
		t.Fatal("expected module path to be used but got:", bundle2.Modules[0])
	}
}

func TestWriterSkipEmptyManifest(t *testing.T) {

	bundle := Bundle{
		Data:     map[string]interface{}{},
		Manifest: Manifest{},
	}

	var buf bytes.Buffer

	if err := NewWriter(&buf).Write(bundle); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	gr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(gr)
	for {
		f, err := tr.Next()
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
			break
		}

		if f.Name != "/data.json" {
			t.Fatal("expected only /data.json but got:", f.Name)
		}
	}
}

func TestGenerateSignature(t *testing.T) {
	signatures := SignaturesConfig{Signatures: []string{"some_token"}}

	bundle := Bundle{
		Data: map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": []interface{}{json.Number("1"), json.Number("2"), json.Number("3")},
				"baz": true,
				"qux": "hello",
			},
		},
		Modules: []ModuleFile{
			{
				URL:    "/foo/corge/corge.rego",
				Path:   "/foo/corge/corge.rego",
				Parsed: ast.MustParseModule(`package foo.corge`),
				Raw:    []byte("package foo.corge\n"),
			},
		},
		Wasm: []byte("modules-compiled-as-wasm-binary"),
		Manifest: Manifest{
			Revision: "quickbrownfaux",
		},
		Signatures: signatures,
	}

	sc := NewSigningConfig("secret", "HS256", "")

	err := bundle.GenerateSignature(sc, "", false)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if reflect.DeepEqual(signatures, bundle.Signatures) {
		t.Fatal("Expected signatures to be different")
	}

	current := bundle.Signatures
	err = bundle.GenerateSignature(sc, "", false)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if !reflect.DeepEqual(current, bundle.Signatures) {
		t.Fatal("Expected signatures to be same")
	}
}

func TestGenerateSignatureWithPlugin(t *testing.T) {
	signatures := SignaturesConfig{Signatures: []string{"some_token"}, Plugin: "_foo"}

	bundle := Bundle{
		Data: map[string]interface{}{
			"foo": map[string]interface{}{
				"bar": []interface{}{json.Number("1"), json.Number("2"), json.Number("3")},
				"baz": true,
				"qux": "hello",
			},
		},
		Modules: []ModuleFile{
			{
				URL:    "/foo/corge/corge.rego",
				Path:   "/foo/corge/corge.rego",
				Parsed: ast.MustParseModule(`package foo.corge`),
				Raw:    []byte("package foo.corge\n"),
			},
		},
		Wasm: []byte("modules-compiled-as-wasm-binary"),
		Manifest: Manifest{
			Revision: "quickbrownfaux",
		},
		Signatures: signatures,
	}

	defaultSigner, _ := GetSigner(defaultSignerID)
	defaultVerifier, _ := GetVerifier(defaultVerifierID)
	if err := RegisterSigner("_foo", defaultSigner); err != nil {
		t.Fatal(err)
	}
	if err := RegisterVerifier("_foo", defaultVerifier); err != nil {
		t.Fatal(err)
	}
	sc := NewSigningConfig("secret", "HS256", "").WithPlugin("_foo")

	err := bundle.GenerateSignature(sc, "", false)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if reflect.DeepEqual(signatures, bundle.Signatures) {
		t.Fatal("Expected signatures to be different")
	}

	current := bundle.Signatures
	err = bundle.GenerateSignature(sc, "", false)
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if !reflect.DeepEqual(current, bundle.Signatures) {
		t.Fatal("Expected signatures to be same")
	}
}

func TestFormatModulesRaw(t *testing.T) {

	bundle1 := Bundle{
		Modules: []ModuleFile{
			{
				URL:    "/foo/corge/corge.rego",
				Path:   "/foo/corge/corge.rego",
				Parsed: ast.MustParseModule(`package foo.corge`),
				Raw:    []byte("package foo.corge\n"),
			},
		},
	}

	bundle2 := Bundle{
		Modules: []ModuleFile{
			{
				URL:    "/foo/corge/corge.rego",
				Path:   "/foo/corge/corge.rego",
				Parsed: ast.MustParseModule(`package foo.corge`),
				Raw:    []byte("package foo.corge"),
			},
		},
	}

	tests := map[string]struct {
		bundle Bundle
		exp    bool
	}{
		"equal":     {bundle: bundle1, exp: true},
		"not_equal": {bundle: bundle2, exp: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			orig := tc.bundle.Modules[0].Raw
			err := tc.bundle.FormatModules(false)
			if err != nil {
				t.Fatal("Unexpected error:", err)
			}

			actual := bytes.Equal(orig, tc.bundle.Modules[0].Raw)
			if actual != tc.exp {
				t.Fatalf("Expected result %v but got %v", tc.exp, actual)
			}
		})
	}
}

func TestFormatModulesParsed(t *testing.T) {

	bundle := Bundle{
		Modules: []ModuleFile{
			{
				URL:    "/foo/corge/corge.rego",
				Path:   "/foo/corge/corge.rego",
				Parsed: ast.MustParseModule(`package foo.corge`),
				Raw:    nil,
			},
		},
	}

	tests := map[string]struct {
		bundle Bundle
	}{
		"parsed": {bundle: bundle},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			err := tc.bundle.FormatModules(false)
			if err != nil {
				t.Fatal("Unexpected error:", err)
			}

			exp := []byte("package foo.corge\n")
			if !bytes.Equal(tc.bundle.Modules[0].Raw, exp) {
				t.Fatalf("Expected raw policy %v but got %v", exp, tc.bundle.Modules[0].Raw)
			}
		})
	}
}

func TestHashBundleFiles(t *testing.T) {
	h, _ := NewSignatureHasher(SHA256)

	tests := map[string]struct {
		data     map[string]interface{}
		manifest Manifest
		wasm     []byte
		plan     []byte
		exp      int
	}{
		"no_content":                 {map[string]interface{}{}, Manifest{}, nil, nil, 1},
		"data":                       {map[string]interface{}{"foo": "bar"}, Manifest{}, nil, nil, 1},
		"data_and_manifest":          {map[string]interface{}{"foo": "bar"}, Manifest{Revision: "quickbrownfaux"}, []byte{}, nil, 2},
		"data_and_manifest_and_wasm": {map[string]interface{}{"foo": "bar"}, Manifest{Revision: "quickbrownfaux"}, []byte("modules-compiled-as-wasm-binary"), nil, 3},
		"data_and_plan":              {map[string]interface{}{"foo": "bar"}, Manifest{Revision: "quickbrownfaux"}, nil, []byte("not a plan but good enough"), 3},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			var plans []PlanModuleFile
			if len(tc.plan) > 0 {
				plans = append(plans, PlanModuleFile{
					URL:  "/plan.json",
					Path: "/plan.json",
					Raw:  tc.plan,
				})
			}

			f, err := hashBundleFiles(h, &Bundle{Data: tc.data, Manifest: tc.manifest, Wasm: tc.wasm, PlanModules: plans})
			if err != nil {
				t.Fatal("Unexpected error:", err)
			}

			if len(f) != tc.exp {
				t.Fatalf("Expected %v file(s) to be added to the signature but got %v", tc.exp, len(f))
			}
		})
	}
}

func TestWriterUseURL(t *testing.T) {

	bundle := Bundle{
		Data: map[string]interface{}{},
		Modules: []ModuleFile{
			{
				URL:    "/url.rego",
				Path:   "/path.rego",
				Parsed: ast.MustParseModule(`package x`),
				Raw:    []byte("package x\n"),
			},
		},
		Manifest: Manifest{Revision: "quickbrownfaux"},
	}

	var buf bytes.Buffer

	if err := NewWriter(&buf).UseModulePath(false).Write(bundle); err != nil {
		t.Fatal("Unexpected error:", err)
	}

	bundle2, err := NewReader(&buf).Read()
	if err != nil {
		t.Fatal("Unexpected error:", err)
	}

	if bundle2.Modules[0].URL != "/url.rego" || bundle2.Modules[0].Path != "/url.rego" {
		t.Fatal("expected module path to be used but got:", bundle2.Modules[0])
	}
}

func TestRootPathsOverlap(t *testing.T) {
	cases := []struct {
		note     string
		rootA    string
		rootB    string
		expected bool
	}{
		{"both empty", "", "", true},
		{"a empty", "", "foo/bar", true},
		{"b empty", "foo/bar", "", true},
		{"no overlap", "a/b/c", "x/y", false},
		{"partial segment overlap a", "a/b", "a/banana", false},
		{"partial segment overlap b", "a/banana", "a/b", false},
		{"overlap a", "a/b", "a/b/c", true},
		{"overlap b", "a/b/c", "a/b", true},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			actual := RootPathsOverlap(tc.rootA, tc.rootB)
			if actual != tc.expected {
				t.Errorf("Expected %t, got %t", tc.expected, actual)
			}
		})
	}
}

func TestParsedModules(t *testing.T) {
	cases := []struct {
		note            string
		bundle          Bundle
		name            string
		expectedModules []string
	}{
		{
			note: "base",
			bundle: Bundle{
				Modules: []ModuleFile{
					{
						Path:   "/foo/policy.rego",
						Parsed: ast.MustParseModule(`package foo`),
						Raw:    []byte(`package foo`),
					},
				},
			},
			name: "test-bundle",
			expectedModules: []string{
				"test-bundle/foo/policy.rego",
			},
		},
		{
			note: "filepath name",
			bundle: Bundle{
				Modules: []ModuleFile{
					{
						Path:   "/foo/policy.rego",
						Parsed: ast.MustParseModule(`package foo`),
						Raw:    []byte(`package foo`),
					},
				},
			},
			name: "/some/system/path",
			expectedModules: []string{
				"/some/system/path/foo/policy.rego",
			},
		},
		{
			note: "file url name",
			bundle: Bundle{
				Modules: []ModuleFile{
					{
						Path:   "/foo/policy.rego",
						Parsed: ast.MustParseModule(`package foo`),
						Raw:    []byte(`package foo`),
					},
				},
			},
			name: "file:///some/system/path",
			expectedModules: []string{
				"/some/system/path/foo/policy.rego",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsedMods := tc.bundle.ParsedModules(tc.name)

			for _, exp := range tc.expectedModules {
				mod, ok := parsedMods[exp]
				if !ok {
					t.Fatalf("Missing expected module %s, got: %+v", exp, parsedMods)
				}
				if mod == nil {
					t.Fatalf("Expected module to be non-nil")
				}
			}
		})
	}
}

func TestMergeCorruptManifest(t *testing.T) {
	_, err := Merge([]*Bundle{
		{},
		{},
	})
	if err == nil || err.Error() != "bundle manifest not initialized" {
		t.Fatal("unexpected error:", err)
	}
}

func TestMerge(t *testing.T) {

	cases := []struct {
		note       string
		bundles    []*Bundle
		wantBundle *Bundle
		wantErr    error
	}{
		{
			note:    "empty list",
			wantErr: errors.New("expected at least one bundle"),
		},
		{
			note: "no op",
			bundles: []*Bundle{
				{
					Manifest: Manifest{
						Revision: "abcdef",
					},
					Modules: []ModuleFile{
						{
							Path:   "x.rego",
							Parsed: ast.MustParseModule(`package foo`),
							Raw:    []byte("package foo"),
						},
					},
				},
			},
			wantBundle: &Bundle{
				Manifest: Manifest{
					Revision: "abcdef",
					Roots:    &[]string{""},
				},
				Modules: []ModuleFile{
					{
						Path:   "x.rego",
						Parsed: ast.MustParseModule(`package foo`),
						Raw:    []byte("package foo"),
					},
				},
			},
		},
		{
			note: "wasm merge legacy error",
			bundles: []*Bundle{
				{
					Manifest: Manifest{
						Roots: &[]string{
							"foo",
						},
					},
					Wasm: []byte("not really wasm, but good enough"),
				},
				{
					Manifest: Manifest{
						Roots: &[]string{
							"bar",
						},
					},
					Wasm: []byte("not really wasm, but good enough"),
				},
			},
			wantBundle: &Bundle{
				Manifest: Manifest{
					Roots: &[]string{
						"foo",
						"bar",
					},
				},
				Data: map[string]interface{}{},
			},
		},
		{
			note: "wasm merge ok",
			bundles: []*Bundle{
				{
					Manifest: Manifest{
						Roots: &[]string{
							"logs",
						},
					},
					WasmModules: []WasmModuleFile{
						{
							URL:         "logs/mask/policy.wasm",
							Path:        "logs/mask/policy.wasm",
							Entrypoints: []ast.Ref{ast.MustParseRef("system.log.mask")},
							Raw:         []byte("not really wasm, but good enough"),
						},
					},
				},
				{
					Manifest: Manifest{
						Roots: &[]string{
							"authz",
						},
					},
					WasmModules: []WasmModuleFile{
						{
							URL:         "authz/allow/policy.wasm",
							Path:        "authz/allow/policy.wasm",
							Entrypoints: []ast.Ref{ast.MustParseRef("authz.allow")},
							Raw:         []byte("not really wasm, but good enough"),
						},
					},
				},
			},
			wantBundle: &Bundle{
				Manifest: Manifest{
					Roots: &[]string{
						"logs",
						"authz",
					},
				},
				WasmModules: []WasmModuleFile{
					{
						URL:         "logs/mask/policy.wasm",
						Path:        "logs/mask/policy.wasm",
						Entrypoints: []ast.Ref{ast.MustParseRef("system.log.mask")},
						Raw:         []byte("not really wasm, but good enough"),
					},
					{
						URL:         "authz/allow/policy.wasm",
						Path:        "authz/allow/policy.wasm",
						Entrypoints: []ast.Ref{ast.MustParseRef("authz.allow")},
						Raw:         []byte("not really wasm, but good enough"),
					},
				},
				Data: map[string]interface{}{},
			},
		},
		{
			note: "merge policy",
			bundles: []*Bundle{
				{
					Manifest: Manifest{
						Roots: &[]string{
							"foo",
						},
					},
					Modules: []ModuleFile{
						{
							URL:    "foo/bar.rego",
							Parsed: ast.MustParseModule(`package foo`),
							Raw:    []byte("package foo"),
						},
					},
				},
				{
					Manifest: Manifest{
						Roots: &[]string{
							"baz",
						},
					},
					Modules: []ModuleFile{
						{
							URL:    "baz/qux.rego",
							Parsed: ast.MustParseModule(`package baz`),
							Raw:    []byte("package baz"),
						},
					},
				},
			},
			wantBundle: &Bundle{
				Manifest: Manifest{
					Roots: &[]string{
						"foo",
						"baz",
					},
				},
				Modules: []ModuleFile{
					{
						URL:    "foo/bar.rego",
						Parsed: ast.MustParseModule(`package foo`),
						Raw:    []byte("package foo"),
					},
					{
						URL:    "baz/qux.rego",
						Parsed: ast.MustParseModule(`package baz`),
						Raw:    []byte("package baz"),
					},
				},
				Data: map[string]interface{}{},
			},
		},
		{
			note: "merge data",
			bundles: []*Bundle{
				{
					Manifest: Manifest{
						Roots: &[]string{
							"foo/bar",
						},
					},
					Data: map[string]interface{}{
						"foo": map[string]interface{}{
							"bar": "val1",
						},
					},
				},
				{
					Manifest: Manifest{
						Roots: &[]string{
							"baz",
						},
					},
					Data: map[string]interface{}{
						"baz": "val2",
					},
				},
			},
			wantBundle: &Bundle{
				Manifest: Manifest{
					Roots: &[]string{
						"foo/bar",
						"baz",
					},
				},
				Data: map[string]interface{}{
					"foo": map[string]interface{}{
						"bar": "val1",
					},
					"baz": "val2",
				},
			},
		},
		{
			note: "merge empty data",
			bundles: []*Bundle{
				{
					Manifest: Manifest{
						Roots: &[]string{
							"foo/bar",
						},
					},
					Data: map[string]interface{}{},
				},
				{
					Manifest: Manifest{
						Roots: &[]string{
							"baz",
						},
					},
					Data: map[string]interface{}{},
				},
			},
			wantBundle: &Bundle{
				Manifest: Manifest{
					Roots: &[]string{
						"foo/bar",
						"baz",
					},
				},
				Data: map[string]interface{}{},
			},
		},
		{
			note: "merge plans",
			bundles: []*Bundle{
				{
					Manifest: Manifest{
						Roots: &[]string{"a"},
					},
					PlanModules: []PlanModuleFile{
						{
							URL:  "a/plan.json",
							Path: "a/plan.json",
							Raw:  []byte("not a real plan but good enough"),
						},
					},
				},
				{
					Manifest: Manifest{
						Roots: &[]string{"b"},
					},
					PlanModules: []PlanModuleFile{
						{
							URL:  "b/plan.json",
							Path: "b/plan.json",
							Raw:  []byte("not a real plan but good enough"),
						},
					},
				},
			},
			wantBundle: &Bundle{
				Data: map[string]interface{}{},
				Manifest: Manifest{
					Roots: &[]string{"a", "b"},
				},
				PlanModules: []PlanModuleFile{
					{
						URL:  "a/plan.json",
						Path: "a/plan.json",
						Raw:  []byte("not a real plan but good enough"),
					},
					{
						URL:  "b/plan.json",
						Path: "b/plan.json",
						Raw:  []byte("not a real plan but good enough"),
					},
				},
			},
		},
		{
			note: "conflicting roots",
			bundles: []*Bundle{
				{
					Manifest: Manifest{
						Roots: &[]string{
							"foo/bar",
						},
					},
				},
				{
					Manifest: Manifest{
						Roots: &[]string{
							"foo",
						},
					},
				},
			},
			wantErr: errors.New("manifest has overlapped roots: 'foo/bar' and 'foo'"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			for i := range tc.bundles {
				if err := tc.bundles[i].Manifest.validateAndInjectDefaults(*tc.bundles[i]); err != nil {
					panic(err)
				}
			}
			b, err := Merge(tc.bundles)
			if tc.wantErr != nil {
				if err == nil {
					t.Fatal("expected error")
				} else if err.Error() != tc.wantErr.Error() {
					t.Fatalf("expected error %q but got: %q", tc.wantErr, err)
				}
			} else if err != nil {
				t.Fatal("unexpected error:", err)
			} else if !b.Equal(*tc.wantBundle) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", tc.wantBundle, b)
			} else if !reflect.DeepEqual(b.Manifest, tc.wantBundle.Manifest) {
				t.Fatalf("Expected manifest:\n\n%v\n\nGot manifest:\n\n%v", tc.wantBundle.Manifest, b.Manifest)
			}
		})
	}
}
