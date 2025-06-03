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
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/image-layout.md
package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/internal/container/set"
	"oras.land/oras-go/v2/internal/descriptor"
	"oras.land/oras-go/v2/internal/graph"
	"oras.land/oras-go/v2/internal/manifestutil"
	"oras.land/oras-go/v2/internal/resolver"
	"oras.land/oras-go/v2/registry"
)

// Store implements `oras.Target`, and represents a content store
// based on file system with the OCI-Image layout.
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/image-layout.md
type Store struct {
	// AutoSaveIndex controls if the OCI store will automatically save the index
	// file when needed.
	//   - If AutoSaveIndex is set to true, the OCI store will automatically save
	//     the changes to `index.json` when
	//      1. pushing a manifest
	//      2. calling Tag() or Delete()
	//   - If AutoSaveIndex is set to false, it's the caller's responsibility
	//     to manually call SaveIndex() when needed.
	//   - Default value: true.
	AutoSaveIndex bool

	// AutoGC controls if the OCI store will automatically clean dangling
	// (unreferenced) blobs created by the Delete() operation. This includes the
	// referrers and the unreferenced successor blobs of the deleted content.
	// Tagged manifests will not be deleted.
	//   - Default value: true.
	AutoGC bool

	root        string
	indexPath   string
	index       *ocispec.Index
	storage     *Storage
	tagResolver *resolver.Memory
	graph       *graph.Memory

	// sync ensures that most operations can be done concurrently, while Delete
	// has the exclusive access to Store if a delete operation is underway.
	// Operations such as Fetch, Push use sync.RLock(), while Delete uses
	// sync.Lock().
	sync sync.RWMutex
	// indexLock ensures that only one go-routine is writing to the index.
	indexLock sync.Mutex
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
		AutoGC:        true,
		root:          rootAbs,
		indexPath:     filepath.Join(rootAbs, ocispec.ImageIndexFile),
		storage:       storage,
		tagResolver:   resolver.NewMemory(),
		graph:         graph.NewMemory(),
	}

	if err := ensureDir(filepath.Join(rootAbs, ocispec.ImageBlobsDir)); err != nil {
		return nil, err
	}
	if err := store.ensureOCILayoutFile(); err != nil {
		return nil, fmt.Errorf("invalid OCI Image Layout: %w", err)
	}
	if err := store.loadIndexFile(ctx); err != nil {
		return nil, fmt.Errorf("invalid OCI Image Index: %w", err)
	}

	return store, nil
}

// Fetch fetches the content identified by the descriptor. It returns an io.ReadCloser.
// It's recommended to close the io.ReadCloser before a Delete operation, otherwise
// Delete may fail (for example on NTFS file systems).
func (s *Store) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	s.sync.RLock()
	defer s.sync.RUnlock()

	return s.storage.Fetch(ctx, target)
}

// Push pushes the content, matching the expected descriptor.
func (s *Store) Push(ctx context.Context, expected ocispec.Descriptor, reader io.Reader) error {
	s.sync.RLock()
	defer s.sync.RUnlock()

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
	s.sync.RLock()
	defer s.sync.RUnlock()

	return s.storage.Exists(ctx, target)
}

// Delete deletes the content matching the descriptor from the store. Delete may
// fail on certain systems (i.e. NTFS), if there is a process (i.e. an unclosed
// Reader) using target.
//   - If s.AutoGC is set to true, Delete will recursively
//     remove the dangling blobs caused by the current delete.
//   - If s.AutoDeleteReferrers is set to true, Delete will recursively remove
//     the referrers of the manifests being deleted.
func (s *Store) Delete(ctx context.Context, target ocispec.Descriptor) error {
	s.sync.Lock()
	defer s.sync.Unlock()

	deleteQueue := []ocispec.Descriptor{target}
	for len(deleteQueue) > 0 {
		head := deleteQueue[0]
		deleteQueue = deleteQueue[1:]

		// get referrers if applicable
		if s.AutoGC && descriptor.IsManifest(head) {
			referrers, err := registry.Referrers(ctx, &unsafeStore{s}, head, "")
			if err != nil {
				return err
			}
			deleteQueue = append(deleteQueue, referrers...)
		}

		// delete the head of queue
		danglings, err := s.delete(ctx, head)
		if err != nil {
			return err
		}
		if s.AutoGC {
			for _, d := range danglings {
				// do not delete existing tagged manifests
				if !s.isTagged(d) {
					deleteQueue = append(deleteQueue, d)
				}
			}
		}
	}

	return nil
}

// delete deletes one node and returns the dangling nodes caused by the delete.
func (s *Store) delete(ctx context.Context, target ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	resolvers := s.tagResolver.Map()
	untagged := false
	for reference, desc := range resolvers {
		if content.Equal(desc, target) {
			s.tagResolver.Untag(reference)
			untagged = true
		}
	}
	danglings := s.graph.Remove(target)
	if untagged && s.AutoSaveIndex {
		err := s.saveIndex()
		if err != nil {
			return nil, err
		}
	}
	if err := s.storage.Delete(ctx, target); err != nil {
		return nil, err
	}
	return danglings, nil
}

// Tag associates a reference string (e.g. "latest") with the descriptor.
// The reference string is recorded in the "org.opencontainers.image.ref.name"
// annotation of the descriptor. When saved, the updated descriptor is persisted
// in the `index.json` file.
//
//   - If the same reference string is tagged multiple times on different
//     descriptors, the descriptor from the last call will be stored.
//   - If the same descriptor is tagged multiple times with different reference
//     strings, multiple copies of the descriptor with different reference tags
//     will be stored in the `index.json` file.
//
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/image-layout.md#indexjson-file
func (s *Store) Tag(ctx context.Context, desc ocispec.Descriptor, reference string) error {
	s.sync.RLock()
	defer s.sync.RUnlock()

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

	return s.tag(ctx, desc, reference)
}

// tag tags a descriptor with a reference string.
func (s *Store) tag(ctx context.Context, desc ocispec.Descriptor, reference string) error {
	dgst := desc.Digest.String()
	if reference != dgst {
		// also tag desc by its digest
		if err := s.tagResolver.Tag(ctx, desc, dgst); err != nil {
			return err
		}
	}
	if err := s.tagResolver.Tag(ctx, desc, reference); err != nil {
		return err
	}
	if s.AutoSaveIndex {
		return s.saveIndex()
	}
	return nil
}

// Resolve resolves a reference to a descriptor.
//   - If the reference to be resolved is a tag, the returned descriptor will be
//     a full descriptor declared by github.com/opencontainers/image-spec/specs-go/v1.
//   - If the reference is a digest, the returned descriptor will be a
//     plain descriptor (containing only the digest, media type and size).
func (s *Store) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	s.sync.RLock()
	defer s.sync.RUnlock()

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

	if reference == desc.Digest.String() {
		return descriptor.Plain(desc), nil
	}

	return desc, nil
}

// Untag disassociates a reference string from its descriptor.
// When saved, the descriptor entry cotanining the reference in the
// "org.opencontainers.image.ref.name" annotation is removed from the
// `index.json` file.
// The actual content identified by the descriptor is NOT deleted.
//
// Reference: https://github.com/opencontainers/image-spec/blob/v1.1.1/image-layout.md#indexjson-file
func (s *Store) Untag(ctx context.Context, reference string) error {
	if reference == "" {
		return errdef.ErrMissingReference
	}

	s.sync.RLock()
	defer s.sync.RUnlock()

	desc, err := s.tagResolver.Resolve(ctx, reference)
	if err != nil {
		return fmt.Errorf("resolving reference %q: %w", reference, err)
	}
	if reference == desc.Digest.String() {
		return fmt.Errorf("reference %q is a digest and not a tag: %w", reference, errdef.ErrInvalidReference)
	}

	s.tagResolver.Untag(reference)
	if s.AutoSaveIndex {
		return s.saveIndex()
	}
	return nil
}

// Predecessors returns the nodes directly pointing to the current node.
// Predecessors returns nil without error if the node does not exists in the
// store.
func (s *Store) Predecessors(ctx context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	s.sync.RLock()
	defer s.sync.RUnlock()

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
	s.sync.RLock()
	defer s.sync.RUnlock()

	return listTags(s.tagResolver, last, fn)
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
			MediaType: ocispec.MediaTypeImageIndex,
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
//     the OCI store will automatically save the changes to `index.json`
//     on Tag() and Delete() calls, and when pushing a manifest.
//   - If AutoSaveIndex is set to false, it's the caller's responsibility
//     to manually call this method when needed.
func (s *Store) SaveIndex() error {
	s.sync.RLock()
	defer s.sync.RUnlock()

	return s.saveIndex()
}

func (s *Store) saveIndex() error {
	s.indexLock.Lock()
	defer s.indexLock.Unlock()

	var manifests []ocispec.Descriptor
	tagged := set.New[digest.Digest]()
	refMap := s.tagResolver.Map()

	// 1. Add descriptors that are associated with tags
	// Note: One descriptor can be associated with multiple tags.
	for ref, desc := range refMap {
		if ref != desc.Digest.String() {
			annotations := make(map[string]string, len(desc.Annotations)+1)
			maps.Copy(annotations, desc.Annotations)
			annotations[ocispec.AnnotationRefName] = ref
			desc.Annotations = annotations
			manifests = append(manifests, desc)
			// mark the digest as tagged for deduplication in step 2
			tagged.Add(desc.Digest)
		}
	}
	// 2. Add descriptors that are not associated with any tag
	for ref, desc := range refMap {
		if ref == desc.Digest.String() && !tagged.Contains(desc.Digest) {
			// skip tagged ones since they have been added in step 1
			manifests = append(manifests, deleteAnnotationRefName(desc))
		}
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

// GC removes garbage from Store. Unsaved index will be lost. To prevent unexpected
// loss, call SaveIndex() before GC or set AutoSaveIndex to true.
// The garbage to be cleaned are:
//   - unreferenced (dangling) blobs in Store which have no predecessors
//   - garbage blobs in the storage whose metadata is not stored in Store
func (s *Store) GC(ctx context.Context) error {
	s.sync.Lock()
	defer s.sync.Unlock()

	// get reachable nodes by reloading the index
	err := s.gcIndex(ctx)
	if err != nil {
		return fmt.Errorf("unable to reload index: %w", err)
	}
	reachableNodes := s.graph.DigestSet()

	// clean up garbage blobs in the storage
	rootpath := filepath.Join(s.root, ocispec.ImageBlobsDir)
	algDirs, err := os.ReadDir(rootpath)
	if err != nil {
		return err
	}
	for _, algDir := range algDirs {
		if !algDir.IsDir() {
			continue
		}
		alg := algDir.Name()
		// skip unsupported directories
		if !isKnownAlgorithm(alg) {
			continue
		}
		algPath := path.Join(rootpath, alg)
		digestEntries, err := os.ReadDir(algPath)
		if err != nil {
			return err
		}
		for _, digestEntry := range digestEntries {
			if err := isContextDone(ctx); err != nil {
				return err
			}
			dgst := digestEntry.Name()
			blobDigest := digest.NewDigestFromEncoded(digest.Algorithm(alg), dgst)
			if err := blobDigest.Validate(); err != nil {
				// skip irrelevant content
				continue
			}
			if !reachableNodes.Contains(blobDigest) {
				// remove the blob from storage if it does not exist in Store
				err = os.Remove(path.Join(algPath, dgst))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// gcIndex reloads the index and updates metadata. Information of untagged blobs
// are cleaned and only tagged blobs remain.
func (s *Store) gcIndex(ctx context.Context) error {
	tagResolver := resolver.NewMemory()
	graph := graph.NewMemory()
	tagged := set.New[digest.Digest]()

	// index tagged manifests
	refMap := s.tagResolver.Map()
	for ref, desc := range refMap {
		if ref == desc.Digest.String() {
			continue
		}
		if err := tagResolver.Tag(ctx, deleteAnnotationRefName(desc), desc.Digest.String()); err != nil {
			return err
		}
		if err := tagResolver.Tag(ctx, desc, ref); err != nil {
			return err
		}
		plain := descriptor.Plain(desc)
		if err := graph.IndexAll(ctx, s.storage, plain); err != nil {
			return err
		}
		tagged.Add(desc.Digest)
	}

	// index referrer manifests
	for ref, desc := range refMap {
		if ref != desc.Digest.String() || tagged.Contains(desc.Digest) {
			continue
		}
		// check if the referrers manifest can traverse to the existing graph
		subject := &desc
		for {
			subject, err := manifestutil.Subject(ctx, s.storage, *subject)
			if err != nil {
				return err
			}
			if subject == nil {
				break
			}
			if graph.Exists(*subject) {
				if err := tagResolver.Tag(ctx, deleteAnnotationRefName(desc), desc.Digest.String()); err != nil {
					return err
				}
				plain := descriptor.Plain(desc)
				if err := graph.IndexAll(ctx, s.storage, plain); err != nil {
					return err
				}
				break
			}
		}
	}
	s.tagResolver = tagResolver
	s.graph = graph
	return nil
}

// isTagged checks if the blob given by the descriptor is tagged.
func (s *Store) isTagged(desc ocispec.Descriptor) bool {
	tagSet := s.tagResolver.TagSet(desc)
	if tagSet.Contains(string(desc.Digest)) {
		return len(tagSet) > 1
	}
	return len(tagSet) > 0
}

// unsafeStore is used to bypass lock restrictions in Delete.
type unsafeStore struct {
	*Store
}

func (s *unsafeStore) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	return s.storage.Fetch(ctx, target)
}

func (s *unsafeStore) Predecessors(ctx context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	return s.graph.Predecessors(ctx, node)
}

// isContextDone returns an error if the context is done.
// Reference: https://pkg.go.dev/context#Context
func isContextDone(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// validateReference validates ref.
func validateReference(ref string) error {
	if ref == "" {
		return errdef.ErrMissingReference
	}

	// TODO: may enforce more strict validation if needed.
	return nil
}

// isKnownAlgorithm checks is a string is a supported hash algorithm
func isKnownAlgorithm(alg string) bool {
	switch digest.Algorithm(alg) {
	case digest.SHA256, digest.SHA512, digest.SHA384:
		return true
	default:
		return false
	}
}
