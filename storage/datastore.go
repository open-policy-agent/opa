// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
)

// DataStore is the backend containing rule references and data.
type DataStore struct {
	mountPath ast.Ref
	data      map[string]interface{}
	triggers  map[string]TriggerConfig
}

// NewDataStore returns an empty DataStore.
func NewDataStore() *DataStore {
	return &DataStore{
		data:      map[string]interface{}{},
		triggers:  map[string]TriggerConfig{},
		mountPath: ast.Ref{ast.DefaultRootDocument},
	}
}

// NewDataStoreFromJSONObject returns a new DataStore containing
// the supplied documents. This is mostly for test purposes.
func NewDataStoreFromJSONObject(data map[string]interface{}) *DataStore {
	ds := NewDataStore()
	for k, v := range data {
		if err := ds.Patch(AddOp, []interface{}{k}, v); err != nil {
			panic(err)
		}
	}
	return ds
}

// SetMountPath updates the data store's mount path. This is the path the data
// store expects all references to be prefixed with.
func (ds *DataStore) SetMountPath(ref ast.Ref) {
	ds.mountPath = ref
}

// ID returns a unique identifier for the in-memory store.
func (ds *DataStore) ID() string {
	return "org.openpolicyagent/in-memory"
}

// Begin is called when a new transaction is started.
func (ds *DataStore) Begin(txn Transaction, refs []ast.Ref) error {
	// TODO(tsandall):
	return nil
}

// Close is called when a transaction is finished.
func (ds *DataStore) Close(txn Transaction) {
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
func (ds *DataStore) Read(txn Transaction, path ast.Ref) (interface{}, error) {
	return ds.GetRef(path)
}

// Write modifies a document referred to by path.
func (ds *DataStore) Write(txn Transaction, op PatchOp, path ast.Ref, value interface{}) error {
	p, err := path.Underlying()
	if err != nil {
		return err
	}
	// TODO(tsandall): Patch() assumes that paths in writes are relative to
	// "data" so drop the head here.
	return ds.Patch(op, p[1:], value)
}

// Get returns the value in Storage referenced by path.
// If the lookup fails, an error is returned with a message indicating
// why the failure occurred.
func (ds *DataStore) Get(path []interface{}) (interface{}, error) {
	return get(ds.data, path)
}

// GetRef returns the value in Storage referred to by the reference.
// This is a convienence function.
func (ds *DataStore) GetRef(ref ast.Ref) (interface{}, error) {

	ref = ref[len(ds.mountPath):]
	path := make([]interface{}, len(ref))

	for i, x := range ref {
		switch v := x.Value.(type) {
		case ast.Ref:
			n, err := ds.GetRef(v)
			if err != nil {
				return nil, err
			}
			path[i] = n
		case ast.String:
			path[i] = string(v)
		case ast.Number:
			path[i] = float64(v)
		case ast.Boolean:
			path[i] = bool(v)
		case ast.Null:
			path[i] = nil
		default:
			return nil, fmt.Errorf("illegal reference element: %v", x)
		}
	}
	return ds.Get(path)
}

// MakePath ensures the specified path exists by creating elements as necessary.
func (ds *DataStore) MakePath(path []interface{}) error {
	var tmp []interface{}
	for _, p := range path {
		tmp = append(tmp, p)
		node, err := ds.Get(tmp)
		if err != nil {
			switch err := err.(type) {
			case *Error:
				if err.Code == NotFoundErr {
					err := ds.Patch(AddOp, tmp, map[string]interface{}{})
					if err != nil {
						return err
					}
					continue
				}
			}
			return err
		}
		switch node.(type) {
		case map[string]interface{}:
		case []interface{}:
		default:
			return fmt.Errorf("non-collection document: %v", path)
		}
	}
	return nil
}

// MustGet calls Get on ds but panics if an error occurs.
func (ds *DataStore) MustGet(path []interface{}) interface{} {
	return mustGet(ds.data, path)
}

// MustPatch calls Patch on ds but panics if an error occurs.
func (ds *DataStore) MustPatch(op PatchOp, path []interface{}, value interface{}) {
	if err := ds.Patch(op, path, value); err != nil {
		panic(err)
	}
}

// Patch modifies the store by performing the associated add/remove/replace operation on the given path.
func (ds *DataStore) Patch(op PatchOp, path []interface{}, value interface{}) error {

	if len(path) == 0 {
		return notFoundError(path, nonEmptyMsg)
	}

	_, isString := path[0].(string)
	if !isString {
		return notFoundError(path, stringHeadMsg)
	}

	for _, t := range ds.triggers {
		if t.Before != nil {
			// TODO(tsandall): use correct transaction.
			if err := t.Before(invalidTXN, op, path, value); err != nil {
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
			if err := t.After(invalidTXN, op, path, value); err != nil {
				return err
			}
		}
	}

	return nil
}

func (ds *DataStore) String() string {
	return fmt.Sprintf("%v", ds.data)
}

func add(data map[string]interface{}, path []interface{}, value interface{}) error {

	// Special case for adding a new root.
	if len(path) == 1 {
		return addRoot(data, path[0].(string), value)
	}

	// Special case for appending to an array.
	switch v := path[len(path)-1].(type) {
	case string:
		if v == "-" {
			return addAppend(data, path[:len(path)-1], value)
		}
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
		return notFoundError(path, nonCollectionMsg(path[len(path)-2]))
	}

}

func addAppend(data map[string]interface{}, path []interface{}, value interface{}) error {

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

	a, ok := n.([]interface{})
	if !ok {
		return notFoundError(path, nonArrayMsg(path[len(path)-1]))
	}

	a = append(a, value)
	e := path[len(path)-1]

	switch parent := parent.(type) {
	case []interface{}:
		i := int(e.(float64))
		parent[i] = a
	case map[string]interface{}:
		k := e.(string)
		parent[k] = a
	default:
		panic(fmt.Sprintf("illegal value: %v %v", parent, path)) // "node" exists, therefore this is not reachable.
	}

	return nil
}

func addInsertArray(data map[string]interface{}, path []interface{}, node []interface{}, value interface{}) error {

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
		k := e.(string)
		parent[k] = node
	case []interface{}:
		i = int(e.(float64))
		parent[i] = node
	default:
		panic(fmt.Sprintf("illegal value: %v %v", parent, path)) // "node" exists, therefore this is not reachable.
	}

	return nil
}

func addInsertObject(data map[string]interface{}, path []interface{}, node map[string]interface{}, value interface{}) error {

	var k string

	switch last := path[len(path)-1].(type) {
	case string:
		k = last
	default:
		return notFoundError(path, objectKeyTypeMsg(last))
	}

	node[k] = value
	return nil
}

func addRoot(data map[string]interface{}, key string, value interface{}) error {
	data[key] = value
	return nil
}

func get(data map[string]interface{}, path []interface{}) (interface{}, error) {
	if len(path) == 0 {
		return data, nil
	}

	head, ok := path[0].(string)
	if !ok {
		return nil, notFoundError(path, stringHeadMsg)
	}

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
			return nil, notFoundError(path, nonCollectionMsg(v))
		}
	}

	return node, nil
}

func mustGet(data map[string]interface{}, path []interface{}) interface{} {
	r, err := get(data, path)
	if err != nil {
		panic(err)
	}
	return r
}

func remove(data map[string]interface{}, path []interface{}) error {

	if _, err := get(data, path); err != nil {
		return err
	}

	// Special case for removing a root.
	if len(path) == 1 {
		return removeRoot(data, path[0].(string))
	}

	node := mustGet(data, path[:len(path)-1])

	switch node := node.(type) {
	case []interface{}:
		return removeArray(data, path, node)
	case map[string]interface{}:
		return removeObject(data, path, node)
	default:
		return notFoundError(path, nonCollectionMsg(path[len(path)-2]))
	}
}

func removeArray(data map[string]interface{}, path []interface{}, node []interface{}) error {

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
		k := e.(string)
		parent[k] = node
	case []interface{}:
		i = int(e.(float64))
		parent[i] = node
	default:
		panic(fmt.Sprintf("illegal value: %v %v", parent, path)) // "node" exists, therefore this is not reachable.
	}

	return nil
}

func removeObject(data map[string]interface{}, path []interface{}, node map[string]interface{}) error {
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

func replace(data map[string]interface{}, path []interface{}, value interface{}) error {

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
		return notFoundError(path, nonCollectionMsg(path[len(path)-2]))
	}

}

func replaceObject(data map[string]interface{}, path []interface{}, node map[string]interface{}, value interface{}) error {
	k, err := checkObjectKey(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	node[k] = value
	return nil
}

func replaceRoot(data map[string]interface{}, path []interface{}, value interface{}) error {
	root := path[0].(string)
	data[root] = value
	return nil
}

func replaceArray(data map[string]interface{}, path []interface{}, node []interface{}, value interface{}) error {
	i, err := checkArrayIndex(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	node[i] = value
	return nil
}

func checkObjectKey(path []interface{}, node map[string]interface{}, v interface{}) (string, error) {
	k, ok := v.(string)
	if !ok {
		return "", notFoundError(path, objectKeyTypeMsg(v))
	}
	_, ok = node[string(k)]
	if !ok {
		return "", notFoundError(path, doesNotExistMsg)
	}
	return string(k), nil
}

func checkArrayIndex(path []interface{}, node []interface{}, v interface{}) (int, error) {
	f, isFloat := v.(float64)
	if !isFloat {
		return 0, notFoundError(path, arrayIndexTypeMsg(v))
	}
	i := int(f)
	if float64(i) != f {
		return 0, notFoundError(path, arrayIndexTypeMsg(v))
	}
	if i >= len(node) {
		return 0, notFoundError(path, outOfRangeMsg)
	} else if i < 0 {
		return 0, notFoundError(path, outOfRangeMsg)
	}
	return i, nil
}
