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
	"os"
	"path/filepath"
	"sync"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/internal/ioutil"
)

// bufPool is a pool of byte buffers that can be reused for copying content
// between files.
var bufPool = sync.Pool{
	New: func() interface{} {
		// the buffer size should be larger than or equal to 128 KiB
		// for performance considerations.
		// we choose 1 MiB here so there will be less disk I/O.
		buffer := make([]byte, 1<<20) // buffer size = 1 MiB
		return &buffer
	},
}

// Storage is a CAS based on file system with the OCI-Image layout.
// Reference: https://github.com/opencontainers/image-spec/blob/master/image-layout.md
type Storage struct {
	// blobRoot is the root directory of the stored blobs.
	blobRoot string
	// ingestRoot is the root directory of the temporary ingest files.
	ingestRoot string
}

// NewStorage creates a new CAS based on file system with the OCI-Image layout.
func NewStorage(root string) *Storage {
	return &Storage{
		blobRoot:   filepath.Join(root, "blobs"),
		ingestRoot: filepath.Join(root, "ingest"),
	}
}

// Fetch fetches the content identified by the descriptor.
func (s *Storage) Fetch(_ context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	path, err := s.blobPath(target.Digest)
	if err != nil {
		return nil, fmt.Errorf("%s: %s: %w", target.Digest, target.MediaType, errdef.ErrInvalidDigest)
	}

	fp, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: %s: %w", target.Digest, target.MediaType, errdef.ErrNotFound)
		}
		return nil, err
	}

	return fp, nil
}

// Push pushes the content, matching the expected descriptor.
func (s *Storage) Push(_ context.Context, expected ocispec.Descriptor, content io.Reader) error {
	target, err := s.blobPath(expected.Digest)
	if err != nil {
		return fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrInvalidDigest)
	}

	// check if the target content already exists in the blob directory.
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrAlreadyExists)
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := ensureDir(filepath.Dir(target)); err != nil {
		return err
	}

	// write the content to a temporary ingest file.
	ingest, err := s.ingest(expected, content)
	if err != nil {
		return err
	}

	// move the content from the temporary ingest file to the target path.
	// since blobs are read-only once stored, if the target blob already exists,
	// Rename() will fail for permission denied when trying to overwrite it.
	if err := os.Rename(ingest, target); err != nil {
		// remove the ingest file in case of error
		os.Remove(ingest)
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("%s: %s: %w", expected.Digest, expected.MediaType, errdef.ErrAlreadyExists)
		}

		return err
	}

	return nil
}

// Exists returns true if the described content Exists.
func (s *Storage) Exists(_ context.Context, target ocispec.Descriptor) (bool, error) {
	path, err := s.blobPath(target.Digest)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

// ingest write the content into a temporary ingest file.
func (s *Storage) ingest(expected ocispec.Descriptor, content io.Reader) (path string, ingestErr error) {
	if err := ensureDir(s.ingestRoot); err != nil {
		return "", fmt.Errorf("failed to ensure ingest dir: %w", err)
	}

	// create a temp file with the file name format "blobDigest_randomString"
	// in the ingest direcotry.
	// Go ensures that multiple programs or goroutines calling CreateTemp
	// simultaneously will not choose the same file.
	fp, err := os.CreateTemp(s.ingestRoot, expected.Digest.Encoded()+"_*")
	if err != nil {
		return "", fmt.Errorf("failed to create ingest file: %w", err)
	}

	path = fp.Name()
	defer func() {
		// remove the temp file in case of error.
		// this executes after the file is closed.
		if ingestErr != nil {
			os.Remove(path)
		}
	}()
	defer fp.Close()

	buf := bufPool.Get().(*[]byte)
	defer bufPool.Put(buf)
	if err := ioutil.CopyBuffer(fp, content, *buf, expected); err != nil {
		return "", fmt.Errorf("failed to ingest: %w", err)
	}

	// change to readonly
	if err := os.Chmod(path, 0444); err != nil {
		return "", fmt.Errorf("failed to make readonly: %w", err)
	}

	return
}

// blobPath calculates blob path from the given digest.
func (s *Storage) blobPath(dgst digest.Digest) (string, error) {
	if err := dgst.Validate(); err != nil {
		return "", fmt.Errorf("cannot calculate blob path from invalid digest %s: %v", dgst.String(), err)
	}

	return filepath.Join(s.blobRoot, dgst.Algorithm().String(), dgst.Encoded()), nil
}

// ensureDir ensures the directories of the path exists.
func ensureDir(path string) error {
	return os.MkdirAll(path, 0777)
}
