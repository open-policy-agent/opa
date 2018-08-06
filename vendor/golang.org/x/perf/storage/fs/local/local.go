// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package local implements the fs.FS interface using local files.
// Metadata is not stored separately; the header of each file should
// contain metadata as written by storage/app.
package local

import (
	"os"
	"path/filepath"

	"golang.org/x/net/context"
	"golang.org/x/perf/storage/fs"
)

// impl is an fs.FS backed by local disk.
type impl struct {
	root string
}

// NewFS constructs an FS that writes to the provided directory.
func NewFS(root string) fs.FS {
	return &impl{root}
}

// NewWriter creates a file and assigns metadata as extended filesystem attributes.
func (fs *impl) NewWriter(ctx context.Context, name string, metadata map[string]string) (fs.Writer, error) {
	if err := os.MkdirAll(filepath.Join(fs.root, filepath.Dir(name)), 0777); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(fs.root, name))
	if err != nil {
		return nil, err
	}
	return &wrapper{f}, nil
}

type wrapper struct {
	*os.File
}

// CloseWithError closes the file and attempts to unlink it.
func (w *wrapper) CloseWithError(error) error {
	w.Close()
	return os.Remove(w.Name())
}
