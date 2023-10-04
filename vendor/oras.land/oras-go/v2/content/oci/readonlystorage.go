/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oci

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/internal/fs/tarfs"
)

// ReadOnlyStorage is a read-only CAS based on file system with the OCI-Image
// layout.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.0-rc4/image-layout.md
type ReadOnlyStorage struct {
	fsys fs.FS
}

// NewStorageFromFS creates a new read-only CAS from fsys.
func NewStorageFromFS(fsys fs.FS) *ReadOnlyStorage {
	return &ReadOnlyStorage{
		fsys: fsys,
	}
}

// NewStorageFromTar creates a new read-only CAS from a tar archive located at
// path.
func NewStorageFromTar(path string) (*ReadOnlyStorage, error) {
	tfs, err := tarfs.New(path)
	if err != nil {
		return nil, err
	}
	return NewStorageFromFS(tfs), nil
}

// Fetch fetches the content identified by the descriptor.
func (s *ReadOnlyStorage) Fetch(_ context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	path, err := blobPath(target.Digest)
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", target.Digest, target.MediaType, errdef.ErrInvalidDigest)
	}

	fp, err := s.fsys.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s: %s: %w", target.Digest, target.MediaType, errdef.ErrNotFound)
		}
		return nil, err
	}

	return fp, nil
}

// Exists returns true if the described content Exists.
func (s *ReadOnlyStorage) Exists(_ context.Context, target ocispec.Descriptor) (bool, error) {
	path, err := blobPath(target.Digest)
	if err != nil {
		return false, fmt.Errorf("%s: %s: %w", target.Digest, target.MediaType, errdef.ErrInvalidDigest)
	}

	_, err = fs.Stat(s.fsys, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// blobPath calculates blob path from the given digest.
func blobPath(dgst digest.Digest) (string, error) {
	if err := dgst.Validate(); err != nil {
		return "", fmt.Errorf("cannot calculate blob path from invalid digest %s: %w: %v",
			dgst.String(), errdef.ErrInvalidDigest, err)
	}
	return path.Join(ociBlobsDir, dgst.Algorithm().String(), dgst.Encoded()), nil
}
