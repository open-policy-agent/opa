// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing/fstest"
)

// WithTempFS creates a temporary directory structure and invokes f with the
// root directory path.
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

	rootDir, err = os.MkdirTemp(root, prefix)

	if err != nil {
		return "", nil, err
	}

	cleanup = func() {
		os.RemoveAll(rootDir)
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
