// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package fs provides a backend-agnostic filesystem layer for storing
// performance results.
package fs

import (
	"errors"
	"io"
	"sort"
	"sync"

	"golang.org/x/net/context"
)

// An FS stores uploaded benchmark data files.
type FS interface {
	// NewWriter returns a Writer for a given file name.
	// When the Writer is closed, the file will be stored with the
	// given metadata and the data written to the writer.
	NewWriter(ctx context.Context, name string, metadata map[string]string) (Writer, error)
}

// A Writer is an io.Writer that can also be closed with an error.
type Writer interface {
	io.WriteCloser
	// CloseWithError cancels the writing of the file, removing
	// any partially written data.
	CloseWithError(error) error
}

// MemFS is an in-memory filesystem implementing the FS interface.
type MemFS struct {
	mu      sync.Mutex
	content map[string]*memFile
}

// NewMemFS constructs a new, empty MemFS.
func NewMemFS() *MemFS {
	return &MemFS{
		content: make(map[string]*memFile),
	}
}

// NewWriter returns a Writer for a given file name. As a side effect,
// it associates the given metadata with the file.
func (fs *MemFS) NewWriter(_ context.Context, name string, metadata map[string]string) (Writer, error) {
	meta := make(map[string]string)
	for k, v := range metadata {
		meta[k] = v
	}
	return &memFile{fs: fs, name: name, metadata: meta}, nil
}

// Files returns the names of the files written to fs.
func (fs *MemFS) Files() []string {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	var files []string
	for f := range fs.content {
		files = append(files, f)
	}
	sort.Strings(files)
	return files
}

// memFile represents a file in a MemFS. While the file is being
// written, fs points to the filesystem. Close writes the file's
// content to fs and sets fs to nil.
type memFile struct {
	fs       *MemFS
	name     string
	metadata map[string]string
	content  []byte
}

func (f *memFile) Write(p []byte) (int, error) {
	f.content = append(f.content, p...)
	return len(p), nil
}

func (f *memFile) Close() error {
	if f.fs == nil {
		return errors.New("already closed")
	}
	f.fs.mu.Lock()
	defer f.fs.mu.Unlock()
	f.fs.content[f.name] = f
	f.fs = nil
	return nil
}

func (f *memFile) CloseWithError(error) error {
	f.fs = nil
	return nil
}
