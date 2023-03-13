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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/internal/descriptor"
	"oras.land/oras-go/v2/internal/fs/tarfs"
	"oras.land/oras-go/v2/internal/graph"
	"oras.land/oras-go/v2/internal/resolver"
)

// ReadOnlyStore implements `oras.ReadonlyTarget`, and represents a read-only
// content store based on file system with the OCI-Image layout.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.0-rc2/image-layout.md
type ReadOnlyStore struct {
	fsys        fs.FS
	storage     content.ReadOnlyStorage
	tagResolver *resolver.Memory
	graph       *graph.Memory
}

// NewFromFS creates a new read-only OCI store from fsys.
func NewFromFS(ctx context.Context, fsys fs.FS) (*ReadOnlyStore, error) {
	store := &ReadOnlyStore{
		fsys:        fsys,
		storage:     NewStorageFromFS(fsys),
		tagResolver: resolver.NewMemory(),
		graph:       graph.NewMemory(),
	}

	if err := store.validateOCILayoutFile(); err != nil {
		return nil, fmt.Errorf("invalid OCI Image Layout: %w", err)
	}
	if err := store.loadIndexFile(ctx); err != nil {
		return nil, fmt.Errorf("invalid OCI Image Layout: %w", err)
	}

	return store, nil
}

// NewFromTar creates a new read-only OCI store from a tar archive located at
// path.
func NewFromTar(ctx context.Context, path string) (*ReadOnlyStore, error) {
	tfs, err := tarfs.New(path)
	if err != nil {
		return nil, err
	}
	return NewFromFS(ctx, tfs)
}

// Fetch fetches the content identified by the descriptor.
func (s *ReadOnlyStore) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	return s.storage.Fetch(ctx, target)
}

// Exists returns true if the described content exists.
func (s *ReadOnlyStore) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	return s.storage.Exists(ctx, target)
}

// Resolve resolves a reference to a descriptor.
func (s *ReadOnlyStore) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	if reference == "" {
		return ocispec.Descriptor{}, errdef.ErrMissingReference
	}

	// attempt resolving manifest
	desc, err := s.tagResolver.Resolve(ctx, reference)
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			// attempt resolving blob
			return resolveBlob(s.fsys, reference)
		}
		return ocispec.Descriptor{}, err
	}
	return descriptor.Plain(desc), nil
}

// Predecessors returns the nodes directly pointing to the current node.
// Predecessors returns nil without error if the node does not exists in the
// store.
func (s *ReadOnlyStore) Predecessors(ctx context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	return s.graph.Predecessors(ctx, node)
}

// Tags lists the tags presented in the `index.json` file of the OCI layout,
// returned in ascending order.
// If `last` is NOT empty, the entries in the response start after the tag
// specified by `last`. Otherwise, the response starts from the top of the tags
// list.
//
// See also `Tags()` in the package `registry`.
func (s *ReadOnlyStore) Tags(ctx context.Context, last string, fn func(tags []string) error) error {
	return listTags(ctx, s.tagResolver, last, fn)
}

// validateOCILayoutFile validates the `oci-layout` file.
func (s *ReadOnlyStore) validateOCILayoutFile() error {
	layoutFile, err := s.fsys.Open(ocispec.ImageLayoutFile)
	if err != nil {
		return fmt.Errorf("failed to open OCI layout file: %w", err)
	}
	defer layoutFile.Close()

	var layout ocispec.ImageLayout
	err = json.NewDecoder(layoutFile).Decode(&layout)
	if err != nil {
		return fmt.Errorf("failed to decode OCI layout file: %w", err)
	}
	return validateOCILayout(&layout)
}

// validateOCILayout validates layout.
func validateOCILayout(layout *ocispec.ImageLayout) error {
	if layout.Version != ocispec.ImageLayoutVersion {
		return errdef.ErrUnsupportedVersion
	}
	return nil
}

// loadIndexFile reads index.json from s.fsys.
func (s *ReadOnlyStore) loadIndexFile(ctx context.Context) error {
	indexFile, err := s.fsys.Open(ociImageIndexFile)
	if err != nil {
		return fmt.Errorf("failed to open index file: %w", err)
	}
	defer indexFile.Close()

	var index ocispec.Index
	if err := json.NewDecoder(indexFile).Decode(&index); err != nil {
		return fmt.Errorf("failed to decode index file: %w", err)
	}
	return loadIndex(ctx, &index, s.storage, s.tagResolver, s.graph)
}

// loadIndex loads index into memory.
func loadIndex(ctx context.Context, index *ocispec.Index, fetcher content.Fetcher, tagger content.Tagger, graph *graph.Memory) error {
	for _, desc := range index.Manifests {
		if err := tagger.Tag(ctx, desc, desc.Digest.String()); err != nil {
			return err
		}
		if ref := desc.Annotations[ocispec.AnnotationRefName]; ref != "" {
			if err := tagger.Tag(ctx, desc, ref); err != nil {
				return err
			}
		}
		plain := descriptor.Plain(desc)
		if err := graph.IndexAll(ctx, fetcher, plain); err != nil {
			return err
		}
	}
	return nil
}

// resolveBlob returns a descriptor describing the blob identified by dgst.
func resolveBlob(fsys fs.FS, dgst string) (ocispec.Descriptor, error) {
	path, err := blobPath(digest.Digest(dgst))
	if err != nil {
		if errors.Is(err, errdef.ErrInvalidDigest) {
			return ocispec.Descriptor{}, errdef.ErrNotFound
		}
		return ocispec.Descriptor{}, err
	}
	fi, err := fs.Stat(fsys, path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ocispec.Descriptor{}, errdef.ErrNotFound
		}
		return ocispec.Descriptor{}, err
	}

	return ocispec.Descriptor{
		MediaType: descriptor.DefaultMediaType,
		Size:      fi.Size(),
		Digest:    digest.Digest(dgst),
	}, nil
}

// listTags returns the tags in ascending order.
// If `last` is NOT empty, the entries in the response start after the tag
// specified by `last`. Otherwise, the response starts from the top of the tags
// list.
//
// See also `Tags()` in the package `registry`.
func listTags(ctx context.Context, tagResolver *resolver.Memory, last string, fn func(tags []string) error) error {
	var tags []string

	tagMap := tagResolver.Map()
	for tag, desc := range tagMap {
		if tag == desc.Digest.String() {
			continue
		}
		if last != "" && tag <= last {
			continue
		}
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	return fn(tags)
}
