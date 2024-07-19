// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem

import (
	"container/list"
	"encoding/json"
	"strconv"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/internal/errors"
	"github.com/open-policy-agent/opa/storage/internal/ptr"
)

// transaction implements the low-level read/write operations on the in-memory
// store and contains the state required for pending transactions.
//
// For write transactions, the struct contains a logical set of updates
// performed by write operations in the transaction. Each write operation
// compacts the set such that two updates never overlap:
//
// - If new update path is a prefix of existing update path, existing update is
// removed, new update is added.
//
// - If existing update path is a prefix of new update path, existing update is
// modified.
//
// - Otherwise, new update is added.
//
// Read transactions do not require any special handling and simply passthrough
// to the underlying store. Read transactions do not support upgrade.
type transaction struct {
	xid      uint64
	write    bool
	stale    bool
	db       *store
	updates  *list.List
	policies map[string]policyUpdate
	context  *storage.Context
}

type policyUpdate struct {
	value  []byte
	remove bool
}

func newTransaction(xid uint64, write bool, context *storage.Context, db *store) *transaction {
	return &transaction{
		xid:      xid,
		write:    write,
		db:       db,
		policies: map[string]policyUpdate{},
		updates:  list.New(),
		context:  context,
	}
}

func (txn *transaction) ID() uint64 {
	return txn.xid
}

func (txn *transaction) Write(op storage.PatchOp, path storage.Path, value interface{}) error {

	if !txn.write {
		return &storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: "data write during read transaction",
		}
	}

	v, err := ast.InterfaceToValue(value) // Can we do a lazy object here?
	if err != nil {
		return err
	}

	if len(path) == 0 {
		return txn.updateRoot(op, value, v)
	}

	for curr := txn.updates.Front(); curr != nil; {
		update := curr.Value.(*updateAST)

		// Check if new update masks existing update exactly. In this case, the
		// existing update can be removed and no other updates have to be
		// visited (because no two updates overlap.)
		if update.path.Equal(path) {
			if update.remove {
				if op != storage.AddOp {
					return errors.NewNotFoundError(path)
				}
			}
			txn.updates.Remove(curr)
			break
		}

		// Check if new update masks existing update. In this case, the
		// existing update has to be removed but other updates may overlap, so
		// we must continue.
		if update.path.HasPrefix(path) {
			remove := curr
			curr = curr.Next()
			txn.updates.Remove(remove)
			continue
		}

		// Check if new update modifies existing update. In this case, the
		// existing update is mutated.
		if path.HasPrefix(update.path) {
			if update.remove {
				return errors.NewNotFoundError(path)
			}
			suffix := path[len(update.path):]
			newUpdate, err := newUpdateAST(update.value, op, suffix, 0, v)
			if err != nil {
				return err
			}
			update.value = newUpdate.Apply(update.value)
			return nil
		}

		curr = curr.Next()
	}

	update, err := newUpdateAST(txn.db.dataAST, op, path, 0, v)
	if err != nil {
		return err
	}

	txn.updates.PushFront(update)
	return nil
}

func (txn *transaction) updateRoot(op storage.PatchOp, value interface{}, valueAST ast.Value) error {
	if op == storage.RemoveOp {
		return invalidPatchError(rootCannotBeRemovedMsg)
	}

	if _, ok := value.(map[string]interface{}); !ok {
		return invalidPatchError(rootMustBeObjectMsg)
	}

	txn.updates.Init()
	txn.updates.PushFront(&updateAST{
		path:   storage.Path{},
		remove: false,
		value:  valueAST,
	})
	return nil
}

func (txn *transaction) Commit() (result storage.TriggerEvent) {
	result.Context = txn.context
	for curr := txn.updates.Front(); curr != nil; curr = curr.Next() {
		action := curr.Value.(*updateAST)
		//updated := action.Apply(txn.db.data)
		//txn.db.data = updated.(map[string]interface{})
		updated := action.Apply(txn.db.dataAST)
		txn.db.dataAST = updated.(ast.Object)

		result.Data = append(result.Data, storage.DataEvent{
			Path:    action.path,
			Data:    action.value,
			Removed: action.remove,
		})
	}
	for id, update := range txn.policies {
		if update.remove {
			delete(txn.db.policies, id)
		} else {
			txn.db.policies[id] = update.value
		}

		result.Policy = append(result.Policy, storage.PolicyEvent{
			ID:      id,
			Data:    update.value,
			Removed: update.remove,
		})
	}
	return result
}

func (txn *transaction) Read(path storage.Path) (interface{}, error) {

	if !txn.write {
		//return ptr.Ptr(txn.db.data, path)
		return ptr.PtrAST(txn.db.dataAST, path)
	}

	merge := []*updateAST{}

	for curr := txn.updates.Front(); curr != nil; curr = curr.Next() {

		update := curr.Value.(*updateAST)

		if path.HasPrefix(update.path) {
			if update.remove {
				return nil, errors.NewNotFoundError(path)
			}
			return ptr.PtrAST(update.value, path[len(update.path):])
		}

		if update.path.HasPrefix(path) {
			merge = append(merge, update)
		}
	}

	data, err := ptr.PtrAST(txn.db.dataAST, path)

	if err != nil {
		return nil, err
	}

	if len(merge) == 0 {
		return data, nil
	}

	//cpy := deepcopy.DeepCopy(data)

	var cpy ast.Value

	switch data := data.(type) {
	case ast.Object:
		cpy = data.Copy()
	case *ast.Array:
		cpy = data.Copy()
	}

	for _, update := range merge {
		cpy = update.Relative(path).Apply(cpy)
	}

	return cpy, nil
}

func (txn *transaction) ListPolicies() []string {
	var ids []string
	for id := range txn.db.policies {
		if _, ok := txn.policies[id]; !ok {
			ids = append(ids, id)
		}
	}
	for id, update := range txn.policies {
		if !update.remove {
			ids = append(ids, id)
		}
	}
	return ids
}

func (txn *transaction) GetPolicy(id string) ([]byte, error) {
	if update, ok := txn.policies[id]; ok {
		if !update.remove {
			return update.value, nil
		}
		return nil, errors.NewNotFoundErrorf("policy id %q", id)
	}
	if exist, ok := txn.db.policies[id]; ok {
		return exist, nil
	}
	return nil, errors.NewNotFoundErrorf("policy id %q", id)
}

func (txn *transaction) UpsertPolicy(id string, bs []byte) error {
	if !txn.write {
		return &storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: "policy write during read transaction",
		}
	}
	txn.policies[id] = policyUpdate{bs, false}
	return nil
}

func (txn *transaction) DeletePolicy(id string) error {
	if !txn.write {
		return &storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: "policy write during read transaction",
		}
	}
	txn.policies[id] = policyUpdate{nil, true}
	return nil
}

// update contains state associated with an update to be applied to the
// in-memory data store.
type update struct {
	path   storage.Path // data path modified by update
	remove bool         // indicates whether update removes the value at path
	value  interface{}  // value to add/replace at path (ignored if remove is true)
}

type updateAST struct {
	path   storage.Path // data path modified by update
	remove bool         // indicates whether update removes the value at path
	value  ast.Value    // value to add/replace at path (ignored if remove is true)
}

func newUpdate(data interface{}, op storage.PatchOp, path storage.Path, idx int, value interface{}) (*update, error) {

	switch data.(type) {
	case nil, bool, json.Number, string:
		return nil, errors.NewNotFoundError(path)
	}

	switch data := data.(type) {
	case map[string]interface{}:
		return newUpdateObject(data, op, path, idx, value)

	case []interface{}:
		return newUpdateArray(data, op, path, idx, value)
	}

	return nil, &storage.Error{
		Code:    storage.InternalErr,
		Message: "invalid data value encountered",
	}
}

func newUpdateAST(data ast.Value, op storage.PatchOp, path storage.Path, idx int, value ast.Value) (*updateAST, error) {

	switch data.(type) {
	case ast.Null, ast.Boolean, ast.Number, ast.String:
		return nil, errors.NewNotFoundError(path)
	}

	switch data := data.(type) {
	case ast.Object:
		return newUpdateObjectAST(data, op, path, idx, value)

	case *ast.Array:
		return newUpdateArrayAST(data, op, path, idx, value)
	}

	return nil, &storage.Error{
		Code:    storage.InternalErr,
		Message: "invalid data value encountered",
	}
}

func newUpdateArrayAST(data *ast.Array, op storage.PatchOp, path storage.Path, idx int, value ast.Value) (*updateAST, error) {

	if idx == len(path)-1 {
		if path[idx] == "-" || path[idx] == strconv.Itoa(data.Len()) {
			if op != storage.AddOp {
				return nil, invalidPatchError("%v: invalid patch path", path)
			}

			cpy := data.Copy()
			cpy = cpy.Append(ast.NewTerm(value))
			return &updateAST{path[:len(path)-1], false, cpy}, nil
		}

		pos, err := ptr.ValidateASTArrayIndex(data, path[idx], path)
		if err != nil {
			return nil, err
		}

		switch op {
		case storage.AddOp:
			var results []*ast.Term
			for i := 0; i < data.Len(); i++ {
				if i == pos {
					results = append(results, ast.NewTerm(value))
				}
				results = append(results, data.Elem(i))
			}

			return &updateAST{path[:len(path)-1], false, ast.NewArray(results...)}, nil

		case storage.RemoveOp:
			var results []*ast.Term
			for i := 0; i < data.Len(); i++ {
				if i != pos {
					results = append(results, data.Elem(i))
				}
			}
			return &updateAST{path[:len(path)-1], false, ast.NewArray(results...)}, nil

		default:
			var results []*ast.Term
			for i := 0; i < data.Len(); i++ {
				if i == pos {
					results = append(results, ast.NewTerm(value))
				} else {
					results = append(results, data.Elem(i))
				}
			}

			return &updateAST{path[:len(path)-1], false, ast.NewArray(results...)}, nil
		}
	}

	pos, err := ptr.ValidateASTArrayIndex(data, path[idx], path)
	if err != nil {
		return nil, err
	}

	return newUpdateAST(data.Elem(pos).Value, op, path, idx+1, value)
}

func newUpdateObjectAST(data ast.Object, op storage.PatchOp, path storage.Path, idx int, value ast.Value) (*updateAST, error) {
	key := ast.StringTerm(path[idx])
	val := data.Get(key)

	if idx == len(path)-1 {
		switch op {
		case storage.ReplaceOp, storage.RemoveOp:
			if val == nil {
				return nil, errors.NewNotFoundError(path)
			}
		}
		return &updateAST{path, op == storage.RemoveOp, value}, nil
	}

	if val != nil {
		return newUpdateAST(val.Value, op, path, idx+1, value)
	}

	return nil, errors.NewNotFoundError(path)
}

func newUpdateArray(data []interface{}, op storage.PatchOp, path storage.Path, idx int, value interface{}) (*update, error) {

	if idx == len(path)-1 {
		if path[idx] == "-" || path[idx] == strconv.Itoa(len(data)) {
			if op != storage.AddOp {
				return nil, invalidPatchError("%v: invalid patch path", path)
			}
			cpy := make([]interface{}, len(data)+1)
			copy(cpy, data)
			cpy[len(data)] = value
			return &update{path[:len(path)-1], false, cpy}, nil
		}

		pos, err := ptr.ValidateArrayIndex(data, path[idx], path)
		if err != nil {
			return nil, err
		}

		switch op {
		case storage.AddOp:
			cpy := make([]interface{}, len(data)+1)
			copy(cpy[:pos], data[:pos])
			copy(cpy[pos+1:], data[pos:])
			cpy[pos] = value
			return &update{path[:len(path)-1], false, cpy}, nil

		case storage.RemoveOp:
			cpy := make([]interface{}, len(data)-1)
			copy(cpy[:pos], data[:pos])
			copy(cpy[pos:], data[pos+1:])
			return &update{path[:len(path)-1], false, cpy}, nil

		default:
			cpy := make([]interface{}, len(data))
			copy(cpy, data)
			cpy[pos] = value
			return &update{path[:len(path)-1], false, cpy}, nil
		}
	}

	pos, err := ptr.ValidateArrayIndex(data, path[idx], path)
	if err != nil {
		return nil, err
	}

	return newUpdate(data[pos], op, path, idx+1, value)
}

func newUpdateObject(data map[string]interface{}, op storage.PatchOp, path storage.Path, idx int, value interface{}) (*update, error) {

	if idx == len(path)-1 {
		switch op {
		case storage.ReplaceOp, storage.RemoveOp:
			if _, ok := data[path[idx]]; !ok {
				return nil, errors.NewNotFoundError(path)
			}
		}
		return &update{path, op == storage.RemoveOp, value}, nil
	}

	if data, ok := data[path[idx]]; ok {
		return newUpdate(data, op, path, idx+1, value)
	}

	return nil, errors.NewNotFoundError(path)
}

func (u *update) Apply(data interface{}) interface{} {
	if len(u.path) == 0 {
		return u.value
	}
	parent, err := ptr.Ptr(data, u.path[:len(u.path)-1])
	if err != nil {
		panic(err)
	}
	key := u.path[len(u.path)-1]
	if u.remove {
		obj := parent.(map[string]interface{})
		delete(obj, key)
		return data
	}
	switch parent := parent.(type) {
	case map[string]interface{}:
		if parent == nil {
			parent = make(map[string]interface{}, 1)
		}
		parent[key] = u.value
	case []interface{}:
		idx, err := strconv.Atoi(key)
		if err != nil {
			panic(err)
		}
		parent[idx] = u.value
	}
	return data
}

func (u *updateAST) Apply(data ast.Value) ast.Value {
	if len(u.path) == 0 {
		return u.value
	}

	parent, err := ptr.PtrAST(data, u.path[:len(u.path)-1])
	if err != nil {
		panic(err)
	}
	key := u.path[len(u.path)-1]
	if u.remove {
		//obj := parent.(ast.Object)
		//obj.Insert(ast.StringTerm(key), ast.NullTerm()) // This should be a delete op from the map

		// For testing: Not performant
		x, err := ast.JSON(data.(ast.Value))
		if err != nil {
			panic(err)
		}

		xMap := x.(map[string]interface{})
		delete(xMap, key)

		v, err := ast.InterfaceToValue(xMap)
		if err != nil {
			panic(err)
		}

		return v
	}
	switch parent := parent.(type) {
	case ast.Object:
		if parent == nil {
			parent = ast.NewObject()
		}
		parent.Insert(ast.StringTerm(key), ast.NewTerm(u.value))
	case *ast.Array:
		idx, err := strconv.Atoi(key)
		if err != nil {
			panic(err)
		}
		parent.Set(idx, ast.NewTerm(u.value))
	}
	return data
}

func (u *update) Relative(path storage.Path) *update {
	cpy := *u
	cpy.path = cpy.path[len(path):]
	return &cpy
}

func (u *updateAST) Relative(path storage.Path) *updateAST {
	cpy := *u
	cpy.path = cpy.path[len(path):]
	return &cpy
}
