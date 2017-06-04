// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

// InMemory implements the storage.Store interface.
type InMemory struct {
	mu       sync.Mutex
	txn      transaction
	data     map[string]interface{}
	policies map[string][]byte
	triggers map[string]storage.TriggerConfig
	indices  *indices
}

type transaction uint64

const (
	invalidTXN = transaction(0)
)

func (t transaction) ID() uint64 {
	return uint64(t)
}

// New returns an empty InMemory store.
func New() *InMemory {
	return &InMemory{
		data:     map[string]interface{}{},
		triggers: map[string]storage.TriggerConfig{},
		policies: map[string][]byte{},
		indices:  newIndices(),
	}
}

// NewFromObject returns a new InMemory store from the supplied data object.
func NewFromObject(data map[string]interface{}) *InMemory {
	db := New()
	ctx := context.Background()
	txn, err := db.NewTransaction(ctx)
	if err != nil {
		panic(err)
	}
	if err := db.Write(ctx, txn, storage.AddOp, storage.Path{}, data); err != nil {
		panic(err)
	}
	if err := db.Commit(ctx, txn); err != nil {
		panic(err)
	}
	return db
}

// NewFromReader returns a new InMemory store from a reader that produces a
// JSON serialized object. This function is for test purposes.
func NewFromReader(r io.Reader) *InMemory {
	d := util.NewJSONDecoder(r)
	var data map[string]interface{}
	if err := d.Decode(&data); err != nil {
		panic(err)
	}
	return NewFromObject(data)
}

// NewTransaction returns a new Transaction.
func (db *InMemory) NewTransaction(ctx context.Context, params ...storage.TransactionParams) (storage.Transaction, error) {
	db.mu.Lock()
	db.txn++
	return db.txn, nil
}

// Commit completes the transaction.
func (db *InMemory) Commit(ctx context.Context, txn storage.Transaction) error {
	db.mu.Unlock()
	return nil
}

// Abort cancels a transaction.
func (db *InMemory) Abort(ctx context.Context, txn storage.Transaction) {
	db.mu.Unlock()
}

// ListPolicies returns policies in the store.
func (db *InMemory) ListPolicies(context.Context, storage.Transaction) ([]string, error) {
	ids := make([]string, 0, len(db.policies))
	for id := range db.policies {
		ids = append(ids, id)
	}
	return ids, nil
}

// GetPolicy returns a policy.
func (db *InMemory) GetPolicy(_ context.Context, _ storage.Transaction, id string) ([]byte, error) {
	bs, ok := db.policies[id]
	if !ok {
		return nil, notFoundErrorf("policy id '%s'", id)
	}
	return bs, nil
}

// UpsertPolicy inserts or updates a policy in the store.
func (db *InMemory) UpsertPolicy(_ context.Context, _ storage.Transaction, id string, bs []byte) error {
	cpy := make([]byte, len(bs))
	copy(cpy, bs)
	db.policies[id] = cpy
	return nil
}

// DeletePolicy removes a policy from the store.
func (db *InMemory) DeletePolicy(_ context.Context, _ storage.Transaction, id string) error {
	_, ok := db.policies[id]
	if !ok {
		return notFoundErrorf("policy id '%s'", id)
	}
	delete(db.policies, id)
	return nil
}

// Register adds a trigger.
func (db *InMemory) Register(id string, config storage.TriggerConfig) error {
	db.triggers[id] = config
	return nil
}

// Unregister removes a trigger.
func (db *InMemory) Unregister(id string) {
	delete(db.triggers, id)
}

// Read fetches a value from the store.
func (db *InMemory) Read(ctx context.Context, txn storage.Transaction, path storage.Path) (interface{}, error) {
	val, err := get(db.data, path)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// Write modifies a document referred to by path.
func (db *InMemory) Write(ctx context.Context, txn storage.Transaction, op storage.PatchOp, path storage.Path, value interface{}) error {
	return db.patch(ctx, txn, op, path, value)
}

// Build creates an index for the ref.
func (db *InMemory) Build(ctx context.Context, txn storage.Transaction, ref ast.Ref) error {
	return db.indices.Build(ctx, db, txn, ref)
}

// Index searches the indices for the ref using the value.
func (db *InMemory) Index(ctx context.Context, txn storage.Transaction, ref ast.Ref, value interface{}, iter storage.IndexIterator) error {
	return db.indices.Index(ctx, ref, value, iter)
}

func (db *InMemory) patch(ctx context.Context, txn storage.Transaction, op storage.PatchOp, path storage.Path, value interface{}) error {

	if len(path) == 0 {
		if op == storage.AddOp || op == storage.ReplaceOp {
			if obj, ok := value.(map[string]interface{}); ok {
				db.data = obj
				return nil
			}
			return invalidPatchErr(rootMustBeObjectMsg)
		}
		return invalidPatchErr(rootCannotBeRemovedMsg)
	}

	for _, t := range db.triggers {
		if t.Before != nil {
			// TODO(tsandall): fix path
			if err := t.Before(ctx, txn, op, nil, value); err != nil {
				return err
			}
		}
	}

	// Perform in-place update on data.
	var err error
	switch op {
	case storage.AddOp:
		err = add(db.data, path, value)
	case storage.RemoveOp:
		err = remove(db.data, path)
	case storage.ReplaceOp:
		err = replace(db.data, path, value)
	}

	if err != nil {
		return err
	}

	for _, t := range db.triggers {
		if t.After != nil {
			// TODO(tsandall): fix path
			if err := t.After(ctx, txn, op, nil, value); err != nil {
				return err
			}
		}
	}

	return nil
}

func add(data map[string]interface{}, path storage.Path, value interface{}) error {

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
		return notFoundError(path)
	}

}

func addAppend(data map[string]interface{}, path storage.Path, value interface{}) error {

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
		return notFoundError(path)
	}

	node = append(node, value)
	e := path[len(path)-1]

	switch parent := parent.(type) {
	case []interface{}:
		i, err := strconv.ParseInt(e, 10, 64)
		if err != nil {
			return notFoundErrorHint(path, arrayIndexTypeMsg)
		}
		parent[i] = node
	case map[string]interface{}:
		parent[e] = node
	default:
		panic("illegal value") // node exists, therefore parent must be collection.
	}

	return nil
}

func addInsertArray(data map[string]interface{}, path storage.Path, node []interface{}, value interface{}) error {

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
			return notFoundErrorHint(path, arrayIndexTypeMsg)
		}
		parent[i] = node
	default:
		panic("illegal value") // node exists, therefore parent must be collection.
	}

	return nil
}

func addInsertObject(data map[string]interface{}, path storage.Path, node map[string]interface{}, value interface{}) error {
	k := path[len(path)-1]
	node[k] = value
	return nil
}

func addRoot(data map[string]interface{}, key string, value interface{}) error {
	data[key] = value
	return nil
}

func get(data map[string]interface{}, path storage.Path) (interface{}, error) {
	if len(path) == 0 {
		return data, nil
	}

	head := path[0]
	node, ok := data[head]
	if !ok {
		return nil, notFoundError(path)

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
			return nil, notFoundError(path)
		}
	}

	return node, nil
}

func mustGet(data map[string]interface{}, path storage.Path) interface{} {
	r, err := get(data, path)
	if err != nil {
		panic(err)
	}
	return r
}

func remove(data map[string]interface{}, path storage.Path) error {

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
		return notFoundError(path)
	}
}

func removeArray(data map[string]interface{}, path storage.Path, node []interface{}) error {

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
			return notFoundErrorHint(path, arrayIndexTypeMsg)
		}
		parent[i] = node
	default:
		panic(fmt.Sprintf("illegal value: %v %v", parent, path)) // "node" exists, therefore this is not reachable.
	}

	return nil
}

func removeObject(data map[string]interface{}, path storage.Path, node map[string]interface{}) error {
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

func replace(data map[string]interface{}, path storage.Path, value interface{}) error {

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
		return notFoundError(path)
	}

}

func replaceObject(data map[string]interface{}, path storage.Path, node map[string]interface{}, value interface{}) error {
	k, err := checkObjectKey(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	node[k] = value
	return nil
}

func replaceRoot(data map[string]interface{}, path storage.Path, value interface{}) error {
	root := path[0]
	data[root] = value
	return nil
}

func replaceArray(data map[string]interface{}, path storage.Path, node []interface{}, value interface{}) error {
	i, err := checkArrayIndex(path, node, path[len(path)-1])
	if err != nil {
		return err
	}

	node[i] = value
	return nil
}

func checkObjectKey(path storage.Path, node map[string]interface{}, v string) (string, error) {
	if _, ok := node[v]; !ok {
		return "", notFoundError(path)
	}
	return v, nil
}

func checkArrayIndex(path storage.Path, node []interface{}, v string) (int, error) {
	i64, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, notFoundErrorHint(path, arrayIndexTypeMsg)
	}
	i := int(i64)
	if i >= len(node) {
		return 0, notFoundErrorHint(path, outOfRangeMsg)
	} else if i < 0 {
		return 0, notFoundErrorHint(path, outOfRangeMsg)
	}
	return i, nil
}

var doesNotExistMsg = "document does not exist"
var rootMustBeObjectMsg = "root must be object"
var rootCannotBeRemovedMsg = "root cannot be removed"
var conflictMsg = "value conflict"
var outOfRangeMsg = "array index out of range"
var arrayIndexTypeMsg = "array index must be integer"
var corruptPolicyMsg = "corrupt policy found"

func internalError(f string, a ...interface{}) *storage.Error {
	return &storage.Error{
		Code:    storage.InternalErr,
		Message: fmt.Sprintf(f, a...),
	}
}

func invalidPatchErr(f string, a ...interface{}) *storage.Error {
	return &storage.Error{
		Code:    storage.InvalidPatchErr,
		Message: fmt.Sprintf(f, a...),
	}
}

func notFoundError(path storage.Path) *storage.Error {
	return notFoundErrorf("%v: %v", path.String(), doesNotExistMsg)
}

func notFoundErrorHint(path storage.Path, hint string) *storage.Error {
	return notFoundErrorf("%v: %v", path.String(), hint)
}

func notFoundRefError(ref ast.Ref) *storage.Error {
	return notFoundErrorf("%v: %v", ref.String(), doesNotExistMsg)
}

func notFoundErrorf(f string, a ...interface{}) *storage.Error {
	msg := fmt.Sprintf(f, a...)
	return &storage.Error{
		Code:    storage.NotFoundErr,
		Message: msg,
	}
}
