// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/open-policy-agent/opa/util"

	"strconv"
)

// DataStore is a simple in-memory data store that implements the storage.Store interface.
type DataStore struct {
	data     map[string]interface{}
	triggers map[string]TriggerConfig
}

// NewDataStore returns an empty DataStore.
func NewDataStore() *DataStore {
	return &DataStore{
		data:     map[string]interface{}{},
		triggers: map[string]TriggerConfig{},
	}
}

// NewDataStoreFromJSONObject returns a new DataStore containing the supplied
// documents. This is mostly for test purposes.
func NewDataStoreFromJSONObject(data map[string]interface{}) *DataStore {
	ds := NewDataStore()
	for k, v := range data {
		if err := ds.patch(context.Background(), AddOp, Path{k}, v); err != nil {
			panic(err)
		}
	}
	return ds
}

// NewDataStoreFromReader returns a new DataStore from a reader that produces a
// JSON serialized object. This function is for test purposes.
func NewDataStoreFromReader(r io.Reader) *DataStore {
	d := util.NewJSONDecoder(r)
	var data map[string]interface{}
	if err := d.Decode(&data); err != nil {
		panic(err)
	}
	return NewDataStoreFromJSONObject(data)
}

// ID returns a unique identifier for the in-memory store.
func (ds *DataStore) ID() string {
	return "org.openpolicyagent/in-memory"
}

// Begin is called when a new transaction is started.
func (ds *DataStore) Begin(ctx context.Context, txn Transaction, params TransactionParams) error {
	// TODO(tsandall):
	return nil
}

// Close is called when a transaction is finished.
func (ds *DataStore) Close(ctx context.Context, txn Transaction) {
	// TODO(tsandall):
}

// Register adds a trigger.
func (ds *DataStore) Register(id string, config TriggerConfig) error {
	ds.triggers[id] = config
	return nil
}

// Unregister removes a trigger.
func (ds *DataStore) Unregister(id string) {
	delete(ds.triggers, id)
}

// Read fetches a value from the in-memory store.
func (ds *DataStore) Read(ctx context.Context, txn Transaction, path Path) (interface{}, error) {
	return get(ds.data, path)
}

// Write modifies a document referred to by path.
func (ds *DataStore) Write(ctx context.Context, txn Transaction, op PatchOp, path Path, value interface{}) error {
	return ds.patch(ctx, op, path, value)
}

func (ds *DataStore) String() string {
	return fmt.Sprintf("%v", ds.data)
}

func (ds *DataStore) patch(ctx context.Context, op PatchOp, path Path, value interface{}) error {

	if len(path) == 0 {
		if op == AddOp || op == ReplaceOp {
			if obj, ok := value.(map[string]interface{}); ok {
				ds.data = obj
				return nil
			}
			return invalidPatchErr(rootMustBeObjectMsg)
		}
		return invalidPatchErr(rootCannotBeRemovedMsg)
	}

	for _, t := range ds.triggers {
		if t.Before != nil {
			// TODO(tsandall): use correct transaction.
			// TODO(tsandall): fix path
			if err := t.Before(ctx, invalidTXN, op, nil, value); err != nil {
				return err
			}
		}
	}

	// Perform in-place update on data.
	var err error
	switch op {
	case AddOp:
		err = add(ds.data, path, value)
	case RemoveOp:
		err = remove(ds.data, path)
	case ReplaceOp:
		err = replace(ds.data, path, value)
	}

	if err != nil {
		return err
	}

	for _, t := range ds.triggers {
		if t.After != nil {
			// TODO(tsandall): use correct transaction.
			// TODO(tsandall): fix path
			if err := t.After(ctx, invalidTXN, op, nil, value); err != nil {
				return err
			}
		}
	}

	return nil
}

func add(data map[string]interface{}, path Path, value interface{}) error {

	// Special case for adding a new root.
	if len(path) == 1 {
		return addRoot(data, path[0], value)
	}

	// Special case for appending to an array.
	if path[len(path)-1] == "-" {
		return addAppend(data, path[:len(path)-1], value)
	}

	node, err := get(data, path[:len(path)-1])
	if err != nil {
		return err
	}

	switch node := node.(type) {
	case map[string]interface{}:
		return addInsertObject(data, path, node, value)
	case []interface{}:
		return addInsertArray(data, path, node, value)
	default:
		return notFoundError(path, doesNotExistMsg)
	}

}

func addAppend(data map[string]interface{}, path Path, value interface{}) error {

	var parent interface{} = data

	if len(path) > 1 {
		r, err := get(data, path[:len(path)-1])
		if err != nil {
			return err
		}
		parent = r
	}

	n, err := get(data, path)
	if err != nil {
		return err
	}

	node, ok := n.([]interface{})
	if !ok {
		return notFoundError(path, doesNotExistMsg)
	}

	node = append(node, value)
	e := path[len(path)-1]

	switch parent := parent.(type) {
	case []interface{}:
		i, err := strconv.ParseInt(e, 10, 64)
		if err != nil {
			return notFoundError(path, "array index must be integer")
		}
		parent[i] = node
	case map[string]interface{}:
		parent[e] = node
	default:
		panic("illegal value") // node exists, therefore parent must be collection.
	}

	return nil
}

func addInsertArray(data map[string]interface{}, path Path, node []interface{}, value interface{}) error {

	i, err := checkArrayIndex(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	var parent interface{} = data

	if len(path) > 2 {
		parent = mustGet(data, path[:len(path)-2]) // "node" exists, therefore parent must exist.
	}

	node = append(node, 0)
	copy(node[i+1:], node[i:])
	node[i] = value
	e := path[len(path)-2]

	switch parent := parent.(type) {
	case map[string]interface{}:
		parent[e] = node
	case []interface{}:
		i, err := strconv.ParseInt(e, 10, 64)
		if err != nil {
			return notFoundError(path, "array index must be integer")
		}
		parent[i] = node
	default:
		panic("illegal value") // node exists, therefore parent must be collection.
	}

	return nil
}

func addInsertObject(data map[string]interface{}, path Path, node map[string]interface{}, value interface{}) error {
	k := path[len(path)-1]
	node[k] = value
	return nil
}

func addRoot(data map[string]interface{}, key string, value interface{}) error {
	data[key] = value
	return nil
}

func get(data map[string]interface{}, path Path) (interface{}, error) {
	if len(path) == 0 {
		return data, nil
	}

	head := path[0]
	node, ok := data[head]
	if !ok {
		return nil, notFoundError(path, doesNotExistMsg)

	}

	for _, v := range path[1:] {
		switch n := node.(type) {

		case map[string]interface{}:
			k, err := checkObjectKey(path, n, v)
			if err != nil {
				return nil, err
			}
			node = n[k]

		case []interface{}:
			idx, err := checkArrayIndex(path, n, v)
			if err != nil {
				return nil, err
			}
			node = n[idx]

		default:
			return nil, notFoundError(path, doesNotExistMsg)
		}
	}

	return node, nil
}

func mustGet(data map[string]interface{}, path Path) interface{} {
	r, err := get(data, path)
	if err != nil {
		panic(err)
	}
	return r
}

func remove(data map[string]interface{}, path Path) error {

	if _, err := get(data, path); err != nil {
		return err
	}

	// Special case for removing a root.
	if len(path) == 1 {
		return removeRoot(data, path[0])
	}

	node := mustGet(data, path[:len(path)-1])

	switch node := node.(type) {
	case []interface{}:
		return removeArray(data, path, node)
	case map[string]interface{}:
		return removeObject(data, path, node)
	default:
		return notFoundError(path, doesNotExistMsg)
	}
}

func removeArray(data map[string]interface{}, path Path, node []interface{}) error {

	i, err := checkArrayIndex(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	var parent interface{} = data

	if len(path) > 2 {
		parent = mustGet(data, path[:len(path)-2]) // "node" exists, therefore parent must exist.
	}

	node = append(node[:i], node[i+1:]...)
	e := path[len(path)-2]

	switch parent := parent.(type) {
	case map[string]interface{}:
		parent[e] = node
	case []interface{}:
		i, err := strconv.ParseInt(e, 10, 64)
		if err != nil {
			return notFoundError(path, "array index must be integer")
		}
		parent[i] = node
	default:
		panic(fmt.Sprintf("illegal value: %v %v", parent, path)) // "node" exists, therefore this is not reachable.
	}

	return nil
}

func removeObject(data map[string]interface{}, path Path, node map[string]interface{}) error {
	k, err := checkObjectKey(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	delete(node, k)
	return nil
}

func removeRoot(data map[string]interface{}, root string) error {
	delete(data, root)
	return nil
}

func replace(data map[string]interface{}, path Path, value interface{}) error {

	if _, err := get(data, path); err != nil {
		return err
	}

	if len(path) == 1 {
		return replaceRoot(data, path, value)
	}

	node := mustGet(data, path[:len(path)-1])

	switch node := node.(type) {
	case map[string]interface{}:
		return replaceObject(data, path, node, value)
	case []interface{}:
		return replaceArray(data, path, node, value)
	default:
		return notFoundError(path, doesNotExistMsg)
	}

}

func replaceObject(data map[string]interface{}, path Path, node map[string]interface{}, value interface{}) error {
	k, err := checkObjectKey(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	node[k] = value
	return nil
}

func replaceRoot(data map[string]interface{}, path Path, value interface{}) error {
	root := path[0]
	data[root] = value
	return nil
}

func replaceArray(data map[string]interface{}, path Path, node []interface{}, value interface{}) error {
	i, err := checkArrayIndex(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	node[i] = value
	return nil
}

func checkObjectKey(path Path, node map[string]interface{}, v string) (string, error) {
	if _, ok := node[v]; !ok {
		return "", notFoundError(path, doesNotExistMsg)
	}
	return v, nil
}

func checkArrayIndex(path Path, node []interface{}, v string) (int, error) {
	i64, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, notFoundError(path, "array index must be integer")
	}
	i := int(i64)
	if i >= len(node) {
		return 0, notFoundError(path, outOfRangeMsg)
	} else if i < 0 {
		return 0, notFoundError(path, outOfRangeMsg)
	}
	return i, nil
}
