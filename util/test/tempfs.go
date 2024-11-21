// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package test

import (
	"io/fs"

	v1 "github.com/open-policy-agent/opa/v1/util/test"
)

// WithTempFS creates a temporary directory structure and invokes f with the
// root directory path.
func WithTempFS(files map[string]string, f func(string)) {
	v1.WithTempFS(files, f)
}

// MakeTempFS creates a temporary directory structure for test purposes rooted at root.
// If root is empty, the dir is created in the default system temp location.
// If the creation fails, cleanup is nil and the caller does not have to invoke it. If
// creation succeeds, the caller should invoke cleanup when they are done.
func MakeTempFS(root, prefix string, files map[string]string) (rootDir string, cleanup func(), err error) {
	return v1.MakeTempFS(root, prefix, files)
}

// WithTestFS creates a temporary file system of `files` in memory
// if `inMemoryFS` is true and invokes `f“ with that filesystem
func WithTestFS(files map[string]string, inMemoryFS bool, f func(string, fs.FS)) {
	v1.WithTestFS(files, inMemoryFS, f)
}
