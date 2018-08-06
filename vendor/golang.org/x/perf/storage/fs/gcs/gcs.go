// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gcs implements the fs.FS interface using Google Cloud Storage.
package gcs

import (
	"cloud.google.com/go/storage"
	"golang.org/x/net/context"
	"golang.org/x/perf/storage/fs"
)

// impl is an fs.FS backed by Google Cloud Storage.
type impl struct {
	bucket *storage.BucketHandle
}

// NewFS constructs an FS that writes to the provided bucket.
// On AppEngine, ctx must be a request-derived Context.
func NewFS(ctx context.Context, bucketName string) (fs.FS, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &impl{client.Bucket(bucketName)}, nil
}

func (fs *impl) NewWriter(ctx context.Context, name string, metadata map[string]string) (fs.Writer, error) {
	w := fs.bucket.Object(name).NewWriter(ctx)
	// TODO(quentin): Do these need "x-goog-meta-" prefixes?
	w.Metadata = metadata
	return w, nil
}
