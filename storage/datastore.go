// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"fmt"

	"github.com/open-policy-agent/opa/ast"
)

// ErrCode represents the collection of error types that can be
// returned by Storage.
type ErrCode int

const (
	// InternalErr indicates an unknown, internal error has occurred.
	InternalErr ErrCode = iota

	// NotFoundErr indicates the path used in the storage operation does not
	// locate a document.
	NotFoundErr = iota
)

// Error is the error type returned by Storage functions.
type Error struct {
	Code    ErrCode
	Message string
}

func (err *Error) Error() string {
	return fmt.Sprintf("storage error (code: %d): %v", err.Code, err.Message)
}

// IsNotFound returns true if this error is a NotFoundErr
func IsNotFound(err error) bool {
	switch err := err.(type) {
	case *Error:
		return err.Code == NotFoundErr
	}
	return false
}

var doesNotExistMsg = "document does not exist"
var outOfRangeMsg = "array index out of range"
var nonEmptyMsg = "path must be non-empty"
var stringHeadMsg = "path must begin with string"

func arrayIndexTypeMsg(v interface{}) string {
	return fmt.Sprintf("array index must be integer, not %T", v)
}

func objectKeyTypeMsg(v interface{}) string {
	return fmt.Sprintf("object key must be string, not %v (%T)", v, v)
}

func nonCollectionMsg(v interface{}) string {
	return fmt.Sprintf("path refers to non-collection document with element %v", v)
}

func nonArrayMsg(v interface{}) string {
	return fmt.Sprintf("path refers to non-array document with element %v", v)
}

func notFoundError(path []interface{}, f string, a ...interface{}) *Error {
	msg := fmt.Sprintf("bad path: %v", path)
	if len(f) > 0 {
		msg += ", " + fmt.Sprintf(f, a...)
	}
	return notFoundErrorf(msg)
}

func notFoundErrorf(f string, a ...interface{}) *Error {
	msg := fmt.Sprintf(f, a...)
	return &Error{
		Code:    NotFoundErr,
		Message: msg,
	}
}

// DataStore is the backend containing rule references and data.
type DataStore struct {
	Indices *Indices
	data    map[string]interface{}
}

// NewDataStore returns an empty DataStore.
func NewDataStore() *DataStore {
	return &DataStore{
		Indices: NewIndices(),
		data:    map[string]interface{}{},
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

// Get returns the value in Storage referenced by path.
// If the lookup fails, an error is returned with a message indicating
// why the failure occurred.
func (ds *DataStore) Get(path []interface{}) (interface{}, error) {
	return get(ds.data, path)
}

// GetRef returns the value in Storage referred to by the reference.
// This is a convienence function.
func (ds *DataStore) GetRef(ref ast.Ref) (interface{}, error) {
	if !ref[0].Equal(ast.DefaultRootDocument) {
		return nil, fmt.Errorf("illegal root %v: %v", ref[0], ref)
	}
	path := []interface{}{}
	for _, x := range ref[1:] {
		switch v := x.Value.(type) {
		case ast.Ref:
			n, err := ds.GetRef(v)
			if err != nil {
				return nil, err
			}
			path = append(path, n)
		case ast.String:
			path = append(path, string(v))
		case ast.Number:
			path = append(path, float64(v))
		case ast.Boolean:
			path = append(path, bool(v))
		case ast.Null:
			path = append(path, nil)
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

// MustGet returns the value in Storage reference by path.
// If the lookup fails, the function will panic.
func (ds *DataStore) MustGet(path []interface{}) interface{} {
	return mustGet(ds.data, path)
}

// PatchOp is the enumeration of supposed modifications.
type PatchOp int

// Patch supports add, remove, and replace operations.
const (
	AddOp     PatchOp = iota
	RemoveOp          = iota
	ReplaceOp         = iota
)

// Patch modifies the store by performing the associated add/remove/replace operation on the given path.
func (ds *DataStore) Patch(op PatchOp, path []interface{}, value interface{}) error {

	if len(path) == 0 {
		return notFoundError(path, nonEmptyMsg)
	}

	_, isString := path[0].(string)
	if !isString {
		return notFoundError(path, stringHeadMsg)
	}

	// Drop the indices that are affected by this patch. This is inefficient for mixed read-write
	// access patterns, but it's easy and simple for now. Once we have more information about
	// the use cases, we can optimize this.
	r := []ast.Ref{}

	err := ds.Indices.Iter(func(ref ast.Ref, index *Index) error {
		if !commonPrefix(ref[1:], path) {
			return nil
		}
		r = append(r, ref)
		return nil
	})

	if err != nil {
		return err
	}

	for _, ref := range r {
		ds.Indices.Drop(ref)
	}

	// Perform in-place update on data.
	switch op {
	case AddOp:
		return add(ds.data, path, value)
	case RemoveOp:
		return remove(ds.data, path)
	case ReplaceOp:
		return replace(ds.data, path, value)
	}

	// Unreachable.
	return nil
}

func (ds *DataStore) String() string {
	return fmt.Sprintf("%v", ds.data)
}

// commonPrefix returns true the reference is a prefix of the path or vice versa.
// The shorter value is the prefix of the other if all of the ground elements
// are equal to the elements at the same indices in the other.
func commonPrefix(ref ast.Ref, path []interface{}) bool {

	min := len(ref)
	if len(path) < min {
		min = len(path)
	}

	cmp := func(a ast.Value, b interface{}) bool {
		switch a := a.(type) {
		case ast.Null:
			if b == nil {
				return true
			}
		case ast.Boolean:
			if b, ok := b.(bool); ok {
				return b == bool(a)
			}
		case ast.Number:
			if b, ok := b.(float64); ok {
				return b == float64(a)
			}
		case ast.String:
			if b, ok := b.(string); ok {
				return b == string(a)
			}
		}
		return false
	}

	v := ast.String(ref[0].Value.(ast.String))

	if !cmp(v, path[0]) {
		return false
	}

	for i := 1; i < min; i++ {
		if !ref[i].IsGround() {
			continue
		}
		if cmp(ref[i].Value, path[i]) {
			continue
		}
		return false
	}

	return true
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
		return nil, notFoundError(path, nonEmptyMsg)
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
