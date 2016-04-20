// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package eval

import (
	"encoding/json"
	"fmt"
	"os"
)

// StorageErrorCode represents the collection of error types that can be
// returned by Storage.
type StorageErrorCode int

const (
	// StorageInternalErr indicates an unknown, internal error has occurred.
	StorageInternalErr StorageErrorCode = iota

	// StorageNotFoundErr indicates the path used in the storage operation does not
	// locate a document.
	StorageNotFoundErr = iota
)

// StorageError is the error type returned by Storage functions.
type StorageError struct {
	Code    StorageErrorCode
	Message string
}

func (err *StorageError) Error() string {
	return fmt.Sprintf("storage error (code: %d): %v", err.Code, err.Message)
}

var doesNotExistMsg = "document does not exist"
var outOfRangeMsg = "array index out of range"
var nonEmptyMsg = "path must be non-empty"
var stringHeadMsg = "path must begin with string"

func arrayIndexTypeMsg(v interface{}) string {
	return fmt.Sprintf("array index must be string, not %T", v)
}

func objectKeyTypeMsg(v interface{}) string {
	return fmt.Sprintf("object key must be string, not %v (%T)", v, v)
}

func nonCollectionMsg(v interface{}) string {
	return fmt.Sprintf("path refers to non-object/non-array document with element %v (%T)", v, v)
}

func nonArrayMsg(v interface{}) string {
	return fmt.Sprintf("path refers to non-array document with element %v (%T)", v, v)
}

func notFoundError(path []interface{}, f string, a ...interface{}) *StorageError {
	msg := fmt.Sprintf("bad path: %v", path)
	if len(f) > 0 {
		msg += ", " + fmt.Sprintf(f, a...)
	}
	return &StorageError{
		Code:    StorageNotFoundErr,
		Message: msg,
	}
}

// Storage is the backend containing rules and data.
type Storage map[interface{}]interface{}

// NewStorage is a helper for creating a new, empty Storage.
func NewStorage() Storage {
	return Storage(map[interface{}]interface{}{})
}

// NewStorageFromJSONFiles is a helper for creating a new Storage containing documents stored in files.
func NewStorageFromJSONFiles(files []string) (Storage, error) {
	store := NewStorage()
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		reader := json.NewDecoder(f)
		for reader.More() {
			var data map[string]interface{}
			if err := reader.Decode(&data); err != nil {
				return nil, err
			}
			// TODO(tsandall): recursive merge instead of replace?
			for k, v := range data {
				if err := store.Patch(StorageAdd, []interface{}{k}, v); err != nil {
					return nil, err
				}
			}
		}

	}
	return store, nil
}

// NewStorageFromJSONObject returns Storage by converting from map[string]interface{}
func NewStorageFromJSONObject(data map[string]interface{}) Storage {
	store := NewStorage()
	for k, v := range data {
		if err := store.Patch(StorageAdd, []interface{}{k}, v); err != nil {
			panic(err)
		}
	}
	return store
}

// Get returns the value in Storage referenced by path.
// If the lookup fails, an error is returned with a message indicating
// why the failure occurred.
func (store Storage) Get(path []interface{}) (interface{}, error) {

	if len(path) == 0 {
		return nil, notFoundError(path, nonEmptyMsg)
	}

	head, ok := path[0].(string)
	if !ok {
		return nil, notFoundError(path, stringHeadMsg)
	}

	node, ok := store[head]
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

// MustGet returns the value in Storage reference by path.
// If the lookup fails, the function will panic.
func (store Storage) MustGet(path []interface{}) interface{} {
	node, err := store.Get(path)
	if err != nil {
		panic(err)
	}
	return node
}

// StorageOp is the enumeration of supposed modifications.
type StorageOp int

const (

	// StorageAdd represents an additive operation.
	StorageAdd StorageOp = iota

	// StorageRemove represents a removal operation.
	StorageRemove = iota

	// StorageReplace represents a replacement operation.
	StorageReplace = iota
)

// Patch modifies the store by performing the associated add/remove/replace operation on the given path.
func (store Storage) Patch(op StorageOp, path []interface{}, value interface{}) error {

	if len(path) == 0 {
		return notFoundError(path, nonEmptyMsg)
	}

	_, isString := path[0].(string)
	if !isString {
		return notFoundError(path, stringHeadMsg)
	}

	switch op {
	case StorageAdd:
		return store.add(path, value)
	case StorageRemove:
		return store.remove(path)
	case StorageReplace:
		return store.replace(path, value)
	default:
		return &StorageError{Code: StorageInternalErr, Message: fmt.Sprintf("invalid operation: %v", op)}
	}
}

func (store Storage) add(path []interface{}, value interface{}) error {

	// Special case for adding a new root.
	if len(path) == 1 {
		return store.addRoot(path[0], value)
	}

	// Special case for appending to an array.
	switch v := path[len(path)-1].(type) {
	case string:
		if v == "-" {
			return store.addAppend(path[:len(path)-1], value)
		}
	}

	node, err := store.Get(path[:len(path)-1])
	if err != nil {
		return err
	}

	switch node := node.(type) {
	case map[string]interface{}:
		return store.addInsertObject(path, node, value)
	case []interface{}:
		return store.addInsertArray(path, node, value)
	default:
		return notFoundError(path, nonCollectionMsg(path[len(path)-2]))
	}

}

func (store Storage) addAppend(path []interface{}, value interface{}) error {

	var nodeParent interface{} = store
	if len(path) > 1 {
		r, err := store.Get(path[:len(path)-1])
		if err != nil {
			return err
		}
		nodeParent = r
	}

	node, err := store.Get(path)
	if err != nil {
		return err
	}

	switch n := node.(type) {
	case []interface{}:
		node = append(n, value)
	default:
		return notFoundError(path, nonArrayMsg(path[len(path)-1]))
	}

	switch nodeParent := nodeParent.(type) {
	case Storage:
		nodeParent[path[len(path)-1]] = node
	case []interface{}:
		// This is safe because it was validated by the lookup above.
		idx := int(path[len(path)-1].(float64))
		nodeParent[idx] = node
	case map[string]interface{}:
		// This is safe because it was validated by the lookup above.
		key := path[len(path)-1].(string)
		nodeParent[key] = node
	default:
		// "node" exists, therefore this is not reachable.
		panic(fmt.Sprintf("illegal value: %v %v", nodeParent, path))
	}

	return nil
}

func (store Storage) addInsertArray(path []interface{}, node []interface{}, value interface{}) error {

	idx, err := checkArrayIndex(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	var nodeParent interface{} = store

	if len(path) > 2 {
		// "node" exists, therefore parent must exist.
		n := store.MustGet(path[:len(path)-2])
		nodeParent = n
	}

	switch nodeParent := nodeParent.(type) {
	case Storage:
		node = append(node, 0)
		copy(node[idx+1:], node[idx:])
		node[idx] = value
		key := path[len(path)-2]
		nodeParent[key] = node
		return nil
	case map[string]interface{}:
		node = append(node, 0)
		copy(node[idx+1:], node[idx:])
		node[idx] = value
		key := path[len(path)-2].(string)
		nodeParent[key] = node
		return nil
	case []interface{}:
		node = append(node, 0)
		copy(node[idx+1:], node[idx:])
		node[idx] = value
		idx = int(path[len(path)-2].(float64))
		nodeParent[idx] = node
		return nil
	default:
		// "node" exists, therefore this is not reachable.
		panic(fmt.Sprintf("illegal value: %v %v", nodeParent, path))
	}
}

func (store Storage) addInsertObject(path []interface{}, node map[string]interface{}, value interface{}) error {
	switch last := path[len(path)-1].(type) {
	case string:
		node[last] = value
		return nil
	default:
		return notFoundError(path, objectKeyTypeMsg(last))
	}
}

func (store Storage) addRoot(key interface{}, value interface{}) error {
	store[key] = value
	return nil
}

func (store Storage) remove(path []interface{}) error {

	// Special case for removing a root.
	if len(path) == 1 {
		delete(store, path[0])
		return nil
	}

	node, err := store.Get(path[:len(path)-1])
	if err != nil {
		return err
	}

	switch node := node.(type) {
	case []interface{}:
		return store.removeArray(path, node)
	case map[string]interface{}:
		return store.removeObject(path, node)
	default:
		return notFoundError(path, nonCollectionMsg(path[len(path)-2]))
	}
}

func (store Storage) removeArray(path []interface{}, node []interface{}) error {

	idx, err := checkArrayIndex(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	var nodeParent interface{} = store

	if len(path) > 2 {
		// "node" exists, therefore parent must exist.
		n := store.MustGet(path[:len(path)-2])
		nodeParent = n
	}

	node = append(node[:idx], node[idx+1:]...)

	switch nodeParent := nodeParent.(type) {
	case Storage:
		key := path[len(path)-2]
		nodeParent[key] = node
		return nil
	case map[string]interface{}:
		key := path[len(path)-2].(string)
		nodeParent[key] = node
		return nil
	case []interface{}:
		idx = int(path[len(path)-2].(float64))
		nodeParent[idx] = node
		return nil
	default:
		// "node" exists, therefore this is not reachable.
		panic(fmt.Sprintf("illegal value: %v %v", nodeParent, path))
	}

}

func (store Storage) removeObject(path []interface{}, node map[string]interface{}) error {
	key, err := checkObjectKey(path, node, path[len(path)-1])
	if err != nil {
		return err
	}
	delete(node, key)
	return nil
}

func (store Storage) replace(path []interface{}, value interface{}) error {

	if len(path) == 1 {
		root := path[0]
		if _, ok := store[root]; !ok {
			return notFoundError(path, doesNotExistMsg)
		}
		store[root] = value
		return nil
	}

	node, err := store.Get(path[:len(path)-1])

	if err != nil {
		return err
	}

	switch node := node.(type) {
	case map[string]interface{}:
		return store.replaceObject(path, node, value)
	case []interface{}:
		return store.replaceArray(path, node, value)
	default:
		return notFoundError(path, nonCollectionMsg(path[len(path)-2]))
	}

}

func (store Storage) replaceObject(path []interface{}, node map[string]interface{}, value interface{}) error {
	key, err := checkObjectKey(path, node, path[len(path)-1])
	if err != nil {
		return err
	}
	node[key] = value
	return nil
}

func (store Storage) replaceArray(path []interface{}, node []interface{}, value interface{}) error {
	idx, err := checkArrayIndex(path, node, path[len(path)-1])
	if err != nil {
		return err
	}
	node[idx] = value
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
	idx := int(f)
	if float64(idx) != f {
		return 0, notFoundError(path, arrayIndexTypeMsg(v))
	}
	if idx >= len(node) {
		return 0, notFoundError(path, outOfRangeMsg)
	} else if idx < 0 {
		return 0, notFoundError(path, outOfRangeMsg)
	}
	return idx, nil
}
