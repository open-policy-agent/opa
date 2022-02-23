// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"fmt"
	"sort"

	"github.com/open-policy-agent/opa/storage"
)

const pathWildcard = "*"

type pathMapper struct {
	dataPrefix                string
	dataPrefixNoTrailingSlash string
	policiesPrefix            string
}

func newPathMapper(schemaVersion, partitionVersion int64) *pathMapper {
	var pm pathMapper
	pm.dataPrefix = fmt.Sprintf("/%v/%v/data/", schemaVersion, partitionVersion)
	pm.dataPrefixNoTrailingSlash = pm.dataPrefix[:len(pm.dataPrefix)-1]
	pm.policiesPrefix = fmt.Sprintf("/%v/%v/policies/", schemaVersion, partitionVersion)
	return &pm
}

func (pm *pathMapper) PolicyKey2ID(key []byte) string {
	return string(key[len(pm.policiesPrefix):])
}

func (pm *pathMapper) PolicyIDPrefix() []byte {
	return []byte(pm.policiesPrefix)
}

func (pm *pathMapper) PolicyID2Key(id string) []byte {
	return []byte(pm.policiesPrefix + id)
}

func (pm *pathMapper) DataKey2Path(key []byte) (storage.Path, error) {
	p, ok := storage.ParsePathEscaped(string(key))
	if !ok {
		return nil, &storage.Error{Code: storage.InternalErr, Message: fmt.Sprintf("corrupt key: %s", key)}
	}
	// skip /<schema_version>/<partition_version>/<data prefix>
	return p[3:], nil
}

func (pm *pathMapper) DataPrefix2Key(path storage.Path) ([]byte, error) {
	if len(path) == 0 {
		return []byte(pm.dataPrefix), nil
	}
	return []byte(pm.dataPrefixNoTrailingSlash + path.String() + "/"), nil
}

func (pm *pathMapper) DataPath2Key(path storage.Path) ([]byte, error) {
	if len(path) == 0 {
		return nil, &storage.Error{Code: storage.InternalErr, Message: "empty path"}
	}
	return []byte(pm.dataPrefixNoTrailingSlash + path.String()), nil
}

type pathSet []storage.Path

func (ps pathSet) IsDisjoint() bool {
	for i := range ps {
		for j := range ps {
			if i != j {
				if hasPrefixWithWildcard(ps[i], ps[j]) {
					return false
				}
			}
		}
	}
	return true
}

// hasPrefixWithWildcard returns true if p starts with other; respecting
// wildcards
func hasPrefixWithWildcard(p, other storage.Path) bool {
	if len(other) > len(p) {
		return false
	}
	for i := range other {
		if p[i] == pathWildcard || other[i] == pathWildcard {
			continue
		}
		if p[i] != other[i] {
			return false
		}
	}
	return true
}

func (ps pathSet) Diff(other pathSet) pathSet {
	diff := pathSet{}
	for _, x := range ps {
		if !other.Contains(x) {
			diff = append(diff, x)
		}
	}
	return diff
}

func (ps pathSet) Contains(x storage.Path) bool {
	for _, other := range ps {
		if x.Equal(other) {
			return true
		}
	}
	return false
}

func (ps pathSet) Sorted() []storage.Path {
	cpy := make(pathSet, len(ps))
	copy(cpy, ps)
	sort.Slice(cpy, func(i, j int) bool {
		return cpy[i].Compare(cpy[j]) < 0
	})
	return cpy
}
