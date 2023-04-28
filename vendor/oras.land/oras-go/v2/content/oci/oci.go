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
// Reference: https://github.com/opencontainers/image-spec/blob/main/image-layout.md
package oci

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/internal/graph"
	"oras.land/oras-go/v2/internal/resolver"
)

// ociImageIndexFile is the file name of the index
// from the OCI Image Layout Specification.
// Reference: https://github.com/opencontainers/image-spec/blob/master/image-layout.md#indexjson-file
const ociImageIndexFile = "index.json"

// Store implements `oras.Target`, and represents a content store
// based on file system with the OCI-Image layout.
// Reference: https://github.com/opencontainers/image-spec/blob/master/image-layout.md
type Store struct {
	// AutoSaveIndex controls if the OCI store will automatically save the index
	// file on each Tag() call.
	// If AutoSaveIndex is set to true, the OCI store will automatically call
	// this method on each Tag() call.
	// If AutoSaveIndex is set to false, it's the caller's responsibility
	// to manually call SaveIndex() when needed.
	// Default value: true.
	AutoSaveIndex bool
	root          string
	indexPath     string

	storage  content.Storage
	resolver *resolver.Memory
	graph    *graph.Memory
	index    *ocispec.Index
}

// New creates a new OCI store with context.Background().
func New(root string) (*Store, error) {
	return NewWithContext(context.Background(), root)
}

// NewWithContext creates a new OCI store.
func NewWithContext(ctx context.Context, root string) (*Store, error) {
	store := &Store{
		AutoSaveIndex: true,
		root:          root,
		indexPath:     filepath.Join(root, ociImageIndexFile),
		storage:       NewStorage(root),
		resolver:      resolver.NewMemory(),
		graph:         graph.NewMemory(),
	}

	if err := ensureDir(root); err != nil {
		return nil, err
	}

	if err := store.ensureOCILayoutFile(); err != nil {
		return nil, err
	}

	if err := store.loadIndex(ctx); err != nil {
		return nil, err
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

	return s.graph.Index(ctx, s.storage, expected)
}

// Exists returns true if the described content exists.
func (s *Store) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	return s.storage.Exists(ctx, target)
}

// Tag tags a descriptor with a reference string.
// A reference should be either a valid tag (e.g. "latest"),
// or a digest matching the descriptor (e.g. "@sha256:abc123").
// Reference: https://github.com/opencontainers/image-spec/blob/main/image-layout.md#indexjson-file
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

	if err := s.resolver.Tag(ctx, desc, reference); err != nil {
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

	return s.resolver.Resolve(ctx, reference)
}

// Predecessors returns the nodes directly pointing to the current node.
// Predecessors returns nil without error if the node does not exists in the
// store.
func (s *Store) Predecessors(ctx context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	return s.graph.Predecessors(ctx, node)
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

		return ioutil.WriteFile(layoutFilePath, layoutJSON, 0666)
	}
	defer layoutFile.Close()

	var layout *ocispec.ImageLayout
	err = json.NewDecoder(layoutFile).Decode(&layout)
	if err != nil {
		return fmt.Errorf("failed to decode OCI layout file: %w", err)
	}
	if layout.Version != ocispec.ImageLayoutVersion {
		return errdef.ErrUnsupportedVersion
	}

	return nil
}

// loadIndex reads the index.json from the file system.
func (s *Store) loadIndex(ctx context.Context) error {
	indexFile, err := os.Open(s.indexPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to open index file: %w", err)
		}
		s.index = &ocispec.Index{
			Versioned: specs.Versioned{
				SchemaVersion: 2, // historical value
			},
		}
		return nil
	}
	defer indexFile.Close()

	if err := json.NewDecoder(indexFile).Decode(&s.index); err != nil {
		return fmt.Errorf("failed to decode index file: %w", err)
	}

	for _, desc := range s.index.Manifests {
		if ref := desc.Annotations[ocispec.AnnotationRefName]; ref != "" {
			if err = s.resolver.Tag(ctx, desc, ref); err != nil {
				return err
			}
		}

		// traverse the whole DAG and index predecessors for all the nodes.
		if err := s.graph.IndexAll(ctx, s.storage, desc); err != nil {
			return err
		}
	}

	return nil
}

// SaveIndex writes the `index.json` file to the file system.
// If AutoSaveIndex is set to true (default value),
// the OCI store will automatically call this method on each Tag() call.
// If AutoSaveIndex is set to false, it's the caller's responsibility
// to manually call this method when needed.
func (s *Store) SaveIndex() error {
	// first need to update the index.
	var manifests []ocispec.Descriptor
	refMap := s.resolver.Map()
	for _, desc := range refMap {
		manifests = append(manifests, desc)
	}

	s.index.Manifests = manifests
	indexJSON, err := json.Marshal(s.index)
	if err != nil {
		return fmt.Errorf("failed to marshal index file: %w", err)
	}

	return os.WriteFile(s.indexPath, indexJSON, 0666)
}

// validateReference validates ref against desc.
func validateReference(ref string) error {
	if ref == "" {
		return errdef.ErrMissingReference
	}

	// TODO: may enforce more strict validation if needed.
	return nil
}
