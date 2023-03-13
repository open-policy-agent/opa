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

// Package oci provides access to an OCI content store.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.0-rc2/image-layout.md
package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/internal/descriptor"
	"oras.land/oras-go/v2/internal/graph"
	"oras.land/oras-go/v2/internal/resolver"
)

// ociImageIndexFile is the file name of the index
// from the OCI Image Layout Specification.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.0-rc2/image-layout.md#indexjson-file
const ociImageIndexFile = "index.json"

// Store implements `oras.Target`, and represents a content store
// based on file system with the OCI-Image layout.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.0-rc2/image-layout.md
type Store struct {
	// AutoSaveIndex controls if the OCI store will automatically save the index
	// file on each Tag() call.
	//   - If AutoSaveIndex is set to true, the OCI store will automatically call
	//     this method on each Tag() call.
	//   - If AutoSaveIndex is set to false, it's the caller's responsibility
	//     to manually call SaveIndex() when needed.
	//   - Default value: true.
	AutoSaveIndex bool
	root          string
	indexPath     string
	index         *ocispec.Index
	indexLock     sync.Mutex

	storage     content.Storage
	tagResolver *resolver.Memory
	graph       *graph.Memory
}

// New creates a new OCI store with context.Background().
func New(root string) (*Store, error) {
	return NewWithContext(context.Background(), root)
}

// NewWithContext creates a new OCI store.
func NewWithContext(ctx context.Context, root string) (*Store, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", root, err)
	}
	storage, err := NewStorage(rootAbs)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	store := &Store{
		AutoSaveIndex: true,
		root:          rootAbs,
		indexPath:     filepath.Join(rootAbs, ociImageIndexFile),
		storage:       storage,
		tagResolver:   resolver.NewMemory(),
		graph:         graph.NewMemory(),
	}

	if err := ensureDir(rootAbs); err != nil {
		return nil, err
	}
	if err := store.ensureOCILayoutFile(); err != nil {
		return nil, fmt.Errorf("invalid OCI Image Layout: %w", err)
	}
	if err := store.loadIndexFile(ctx); err != nil {
		return nil, fmt.Errorf("invalid OCI Image Layout: %w", err)
	}

	return store, nil
}

// Fetch fetches the content identified by the descriptor.
func (s *Store) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	return s.storage.Fetch(ctx, target)
}

// Push pushes the content, matching the expected descriptor.
func (s *Store) Push(ctx context.Context, expected ocispec.Descriptor, reader io.Reader) error {
	if err := s.storage.Push(ctx, expected, reader); err != nil {
		return err
	}
	if err := s.graph.Index(ctx, s.storage, expected); err != nil {
		return err
	}
	if descriptor.IsManifest(expected) {
		// tag by digest
		return s.tag(ctx, expected, expected.Digest.String())
	}
	return nil
}

// Exists returns true if the described content exists.
func (s *Store) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	return s.storage.Exists(ctx, target)
}

// Tag tags a descriptor with a reference string.
// reference should be a valid tag (e.g. "latest").
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.0-rc2/image-layout.md#indexjson-file
func (s *Store) Tag(ctx context.Context, desc ocispec.Descriptor, reference string) error {
	if err := validateReference(reference); err != nil {
		return err
	}

	exists, err := s.storage.Exists(ctx, desc)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s: %s: %w", desc.Digest, desc.MediaType, errdef.ErrNotFound)
	}

	if desc.Annotations == nil {
		desc.Annotations = map[string]string{}
	}
	desc.Annotations[ocispec.AnnotationRefName] = reference
	return s.tag(ctx, desc, reference)
}

// tag tags a descriptor with a reference string.
func (s *Store) tag(ctx context.Context, desc ocispec.Descriptor, reference string) error {
	dgst := desc.Digest.String()
	if reference != dgst {
		// mark desc for deduplication in SaveIndex()
		if err := s.tagResolver.Tag(ctx, desc, dgst); err != nil {
			return err
		}
	}
	if err := s.tagResolver.Tag(ctx, desc, reference); err != nil {
		return err
	}
	if s.AutoSaveIndex {
		return s.SaveIndex()
	}
	return nil
}

// Resolve resolves a reference to a descriptor.
func (s *Store) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	if reference == "" {
		return ocispec.Descriptor{}, errdef.ErrMissingReference
	}

	// attempt resolving manifest
	desc, err := s.tagResolver.Resolve(ctx, reference)
	if err != nil {
		if errors.Is(err, errdef.ErrNotFound) {
			// attempt resolving blob
			return resolveBlob(os.DirFS(s.root), reference)
		}
		return ocispec.Descriptor{}, err
	}
	return descriptor.Plain(desc), nil
}

// Predecessors returns the nodes directly pointing to the current node.
// Predecessors returns nil without error if the node does not exists in the
// store.
func (s *Store) Predecessors(ctx context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	return s.graph.Predecessors(ctx, node)
}

// Tags lists the tags presented in the `index.json` file of the OCI layout,
// returned in ascending order.
// If `last` is NOT empty, the entries in the response start after the tag
// specified by `last`. Otherwise, the response starts from the top of the tags
// list.
//
// See also `Tags()` in the package `registry`.
func (s *Store) Tags(ctx context.Context, last string, fn func(tags []string) error) error {
	return listTags(ctx, s.tagResolver, last, fn)
}

// ensureOCILayoutFile ensures the `oci-layout` file.
func (s *Store) ensureOCILayoutFile() error {
	layoutFilePath := filepath.Join(s.root, ocispec.ImageLayoutFile)
	layoutFile, err := os.Open(layoutFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to open OCI layout file: %w", err)
		}

		layout := ocispec.ImageLayout{
			Version: ocispec.ImageLayoutVersion,
		}
		layoutJSON, err := json.Marshal(layout)
		if err != nil {
			return fmt.Errorf("failed to marshal OCI layout file: %w", err)
		}
		return os.WriteFile(layoutFilePath, layoutJSON, 0666)
	}
	defer layoutFile.Close()

	var layout ocispec.ImageLayout
	err = json.NewDecoder(layoutFile).Decode(&layout)
	if err != nil {
		return fmt.Errorf("failed to decode OCI layout file: %w", err)
	}
	return validateOCILayout(&layout)
}

// loadIndexFile reads index.json from the file system.
// Create index.json if it does not exist.
func (s *Store) loadIndexFile(ctx context.Context) error {
	indexFile, err := os.Open(s.indexPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to open index file: %w", err)
		}

		// write index.json if it does not exist
		s.index = &ocispec.Index{
			Versioned: specs.Versioned{
				SchemaVersion: 2, // historical value
			},
			Manifests: []ocispec.Descriptor{},
		}
		return s.writeIndexFile()
	}
	defer indexFile.Close()

	var index ocispec.Index
	if err := json.NewDecoder(indexFile).Decode(&index); err != nil {
		return fmt.Errorf("failed to decode index file: %w", err)
	}
	s.index = &index
	return loadIndex(ctx, s.index, s.storage, s.tagResolver, s.graph)
}

// SaveIndex writes the `index.json` file to the file system.
//   - If AutoSaveIndex is set to true (default value),
//     the OCI store will automatically call this method on each Tag() call.
//   - If AutoSaveIndex is set to false, it's the caller's responsibility
//     to manually call this method when needed.
func (s *Store) SaveIndex() error {
	s.indexLock.Lock()
	defer s.indexLock.Unlock()

	var manifests []ocispec.Descriptor
	refMap := s.tagResolver.Map()
	for ref, desc := range refMap {
		if ref == desc.Digest.String() && desc.Annotations[ocispec.AnnotationRefName] != "" {
			// skip saving desc if ref is a digest and desc is tagged
			continue
		}
		manifests = append(manifests, desc)
	}
	s.index.Manifests = manifests
	return s.writeIndexFile()
}

// writeIndexFile writes the `index.json` file.
func (s *Store) writeIndexFile() error {
	indexJSON, err := json.Marshal(s.index)
	if err != nil {
		return fmt.Errorf("failed to marshal index file: %w", err)
	}
	return os.WriteFile(s.indexPath, indexJSON, 0666)
}

// validateReference validates ref.
func validateReference(ref string) error {
	if ref == "" {
		return errdef.ErrMissingReference
	}

	// TODO: may enforce more strict validation if needed.
	return nil
}
