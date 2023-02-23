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

package graph

import (
	"context"
	"errors"
	"sync"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/internal/descriptor"
	"oras.land/oras-go/v2/internal/status"
	"oras.land/oras-go/v2/internal/syncutil"
)

// Memory is a memory based PredecessorFinder.
type Memory struct {
	predecessors sync.Map // map[descriptor.Descriptor]map[descriptor.Descriptor]ocispec.Descriptor
	indexed      sync.Map // map[descriptor.Descriptor]any
}

// NewMemory creates a new memory PredecessorFinder.
func NewMemory() *Memory {
	return &Memory{}
}

// Index indexes predecessors for each direct successor of the given node.
// There is no data consistency issue as long as deletion is not implemented
// for the underlying storage.
func (m *Memory) Index(ctx context.Context, fetcher content.Fetcher, node ocispec.Descriptor) error {
	successors, err := content.Successors(ctx, fetcher, node)
	if err != nil {
		return err
	}

	m.index(ctx, node, successors)
	return nil
}

// Index indexes predecessors for all the successors of the given node.
// There is no data consistency issue as long as deletion is not implemented
// for the underlying storage.
func (m *Memory) IndexAll(ctx context.Context, fetcher content.Fetcher, node ocispec.Descriptor) error {
	// track content status
	tracker := status.NewTracker()

	var fn syncutil.GoFunc[ocispec.Descriptor]
	fn = func(ctx context.Context, region *syncutil.LimitedRegion, desc ocispec.Descriptor) error {
		// skip the node if other go routine is working on it
		_, committed := tracker.TryCommit(desc)
		if !committed {
			return nil
		}

		// skip the node if it has been indexed
		key := descriptor.FromOCI(desc)
		_, exists := m.indexed.Load(key)
		if exists {
			return nil
		}

		successors, err := content.Successors(ctx, fetcher, desc)
		if err != nil {
			if errors.Is(err, errdef.ErrNotFound) {
				// skip the node if it does not exist
				return nil
			}
			return err
		}
		m.index(ctx, desc, successors)
		m.indexed.Store(key, nil)

		if len(successors) > 0 {
			// traverse and index successors
			return syncutil.Go(ctx, nil, fn, successors...)
		}
		return nil
	}
	return syncutil.Go(ctx, nil, fn, node)
}

// Predecessors returns the nodes directly pointing to the current node.
// Predecessors returns nil without error if the node does not exists in the
// store.
// Like other operations, calling Predecessors() is go-routine safe. However,
// it does not necessarily correspond to any consistent snapshot of the stored
// contents.
func (m *Memory) Predecessors(_ context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error) {
	key := descriptor.FromOCI(node)
	value, exists := m.predecessors.Load(key)
	if !exists {
		return nil, nil
	}
	predecessors := value.(*sync.Map)

	var res []ocispec.Descriptor
	predecessors.Range(func(key, value interface{}) bool {
		res = append(res, value.(ocispec.Descriptor))
		return true
	})
	return res, nil
}

// index indexes predecessors for each direct successor of the given node.
// There is no data consistency issue as long as deletion is not implemented
// for the underlying storage.
func (m *Memory) index(ctx context.Context, node ocispec.Descriptor, successors []ocispec.Descriptor) {
	if len(successors) == 0 {
		return
	}

	predecessorKey := descriptor.FromOCI(node)
	for _, successor := range successors {
		successorKey := descriptor.FromOCI(successor)
		value, _ := m.predecessors.LoadOrStore(successorKey, &sync.Map{})
		predecessors := value.(*sync.Map)
		predecessors.Store(predecessorKey, node)
	}
}
