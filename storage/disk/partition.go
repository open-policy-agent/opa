// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"github.com/open-policy-agent/opa/storage"
)

type partitionTrie struct {
	partitions map[string]*partitionTrie
}

func buildPartitionTrie(paths []storage.Path) *partitionTrie {
	root := newPartitionTrie()
	for i := range paths {
		root.insert(paths[i])
	}
	return root
}

func newPartitionTrie() *partitionTrie {
	return &partitionTrie{
		partitions: make(map[string]*partitionTrie),
	}
}

func (p *partitionTrie) Find(path storage.Path) (int, *partitionTrie) {
	node := p
	for i, x := range path {
		next, ok := node.partitions[x]
		if !ok {
			return i + 1, nil
		}
		node = next
	}
	return len(path), node
}

func (p *partitionTrie) insert(path storage.Path) {

	if len(path) == 0 {
		return
	}

	head := path[0]
	child, ok := p.partitions[head]
	if !ok {
		child = newPartitionTrie()
		p.partitions[head] = child
	}

	child.insert(path[1:])
}
