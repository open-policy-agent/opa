// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package test

import (
	"io/fs"
	"iter"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/open-policy-agent/opa/v1/util"
)

// TempDir creates a temporary directory structure for the scope of a test, and populates it with the given files.
func TempDir(tb testing.TB, files map[string]string) (root string) {
	tb.Helper()

	root = tb.TempDir()
	for path, content := range files {
		path = filepath.Join(root, filepath.FromSlash(path))
		noErr(tb, os.MkdirAll(filepath.Dir(path), 0777))
		if content != "" {
			noErr(tb, os.WriteFile(path, util.StringToByteSlice(content), 0666))
		}
	}

	return root
}

// TempDirOf creates a temporary directory structure for the scope of a test, and populates it
// with the given files, provided as a list of path/content pairs. This is a convenience wrapper
// around TempDir for use when only a few files are needed.
func TempDirOf(tb testing.TB, fileContentPairs ...string) (root string) {
	tb.Helper()

	if len(fileContentPairs)%2 != 0 {
		tb.Fatalf("need even number of file+content strings, got %d", len(fileContentPairs))
	}

	return TempDir(tb, maps.Collect(pairs(fileContentPairs)))
}

// WithTempFS creates a temporary directory structure and invokes f with the root directory path.
//
// Deprecated: Prefer using TempDir or TempDirOf instead as those are proper test helpers and will
// report accurate location information on failures.
func WithTempFS(files map[string]string, f func(string)) {
	rootDir, cleanup, err := MakeTempFS("", "opa_test", files)
	if err != nil {
		panic(err)
	}
	defer cleanup()
	f(rootDir)
}

// MakeTempFS creates a temporary directory structure for test purposes rooted at root.
// If root is empty, the dir is created in the default system temp location.
// If the creation fails, cleanup is nil and the caller does not have to invoke it. If
// creation succeeds, the caller should invoke cleanup when they are done.
func MakeTempFS(root, prefix string, files map[string]string) (rootDir string, cleanup func(), err error) {
	if rootDir, err = os.MkdirTemp(root, prefix); err != nil {
		return "", nil, err
	}

	cleanup = func() {
		_ = os.RemoveAll(rootDir)
	}

	skipCleanup := false

	// Cleanup unless flag is unset. It will be unset if we succeed.
	defer func() {
		if !skipCleanup {
			cleanup()
		}
	}()

	for path, content := range files {
		dirname, filename := filepath.Split(path)
		dirPath := filepath.Join(rootDir, dirname)
		if err := os.MkdirAll(dirPath, 0777); err != nil {
			return "", nil, err
		}

		f, err := os.Create(filepath.Join(dirPath, filename))
		if err != nil {
			return "", nil, err
		}

		if _, err := f.WriteString(content); err != nil {
			return "", nil, err
		}
	}

	skipCleanup = true

	return rootDir, cleanup, nil
}

// WithTestFS creates a temporary file system of `files` in memory
// if `inMemoryFS` is true and invokes `f“ with that filesystem
func WithTestFS(files map[string]string, inMemoryFS bool, f func(string, fs.FS)) {
	if inMemoryFS {
		fsys := make(fstest.MapFS)
		rootDir := "."
		for k, v := range files {
			fsys[filepath.Join(rootDir, k)] = &fstest.MapFile{Data: []byte(v)}
		}
		f(rootDir, fsys)
	} else {
		rootDir, cleanup, err := MakeTempFS("", "opa_test", files)
		if err != nil {
			panic(err)
		}
		defer cleanup()
		f(rootDir, nil)
	}
}

func noErr(tb testing.TB, err error) {
	tb.Helper()
	if err != nil {
		tb.Fatal(err)
	}
}

// pairs returns an iterator of value pairs in s. If s has an odd number of values, the last value is ignored.
func pairs[T any](s []T) iter.Seq2[T, T] {
	if len(s)%2 != 0 {
		s = s[:len(s)-1]
	}
	return func(yield func(T, T) bool) {
		for i := 0; i < len(s); i += 2 {
			if !yield(s[i], s[i+1]) {
				return
			}
		}
	}
}
