// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package pathwatcher

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/internal/storage/mock"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestWatchPaths(t *testing.T) {

	fs := map[string]string{
		"/foo/bar/baz.json": "true",
		"/foo/faz/baz.json": "true",
		"/foo/baz.json":     "true",
	}

	expected := []string{
		"/foo", "/foo/bar", "/foo/faz",
	}

	test.WithTempFS(fs, func(rootDir string) {
		paths, err := getWatchPaths([]string{"prefix:" + rootDir + "/foo"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		result := make([]string, 0, len(paths))
		for _, p := range paths {
			result = append(result, filepath.Clean(strings.TrimPrefix(p, rootDir)))
		}
		if !slices.Equal(expected, result) {
			t.Fatalf("Expected %q but got: %q", expected, result)
		}
	})
}

func TestProcessWatcherUpdateForRegoVersion(t *testing.T) {
	files := map[string]string{}

	test.WithTempFS(files, func(rootDir string) {
		regoVersion := ast.RegoV1

		// create a tar-ball bundle
		tar := filepath.Join(rootDir, "bundle.tar.gz")
		buf := archive.MustWriteTarGz(make([][2]string, 0, len(files)))
		bf, err := os.Create(tar)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		_, err = bf.Write(buf.Bytes())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// add only the bundle in the paths
		paths := []string{
			tar,
		}
		filter := func(string, os.FileInfo, int) bool {
			return false
		}
		f := func(ctx context.Context, txn storage.Transaction, loaded *initload.LoadPathsResult) error {
			if loaded.Files.Modules == nil {
				t.Fatalf("Unexpected nil loaded modules")
			}

			return nil
		}

		store := mock.New()

		// add a file that isn't registered in one of the paths
		err = storage.Txn(t.Context(), store, storage.WriteParams, func(txn storage.Transaction) error {
			err := store.UpsertPolicy(t.Context(), txn, "foo.rego", []byte(`package foo`))
			if err != nil {
				t.Fatal(err)
			}

			return nil
		})
		if err != nil {
			t.Fatal(err)
		}

		err = ProcessWatcherUpdateForRegoVersion(t.Context(), regoVersion, paths, "", store, filter, false, false, f)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})
}
