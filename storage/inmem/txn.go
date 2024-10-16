// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem

import (
	"container/list"
	"encoding/json"
	"strconv"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/deepcopy"
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

	if len(path) == 0 {
		return txn.updateRoot(op, value)
	}

	for curr := txn.updates.Front(); curr != nil; {
		update := curr.Value.(dataUpdate)

		// Check if new update masks existing update exactly. In this case, the
		// existing update can be removed and no other updates have to be
		// visited (because no two updates overlap.)
		if update.Path().Equal(path) {
			if update.Remove() {
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
		if update.Path().HasPrefix(path) {
			remove := curr
			curr = curr.Next()
			txn.updates.Remove(remove)
			continue
		}

		// Check if new update modifies existing update. In this case, the
		// existing update is mutated.
		if path.HasPrefix(update.Path()) {
			if update.Remove() {
				return errors.NewNotFoundError(path)
			}
			suffix := path[len(update.Path()):]
			newUpdate, err := txn.db.newUpdate(update.Value(), op, suffix, 0, value)
			if err != nil {
				return err
			}
			update.Set(newUpdate.Apply(update.Value()))
			return nil
		}

		curr = curr.Next()
	}

	//update, err := newUpdateAST(txn.db.value(), op, path, 0, v)
	update, err := txn.db.newUpdate(txn.db.value(), op, path, 0, value)
	if err != nil {
		return err
	}

	txn.updates.PushFront(update)
	return nil
}

func (txn *transaction) updateRoot(op storage.PatchOp, value interface{}) error {
	if op == storage.RemoveOp {
		return invalidPatchError(rootCannotBeRemovedMsg)
	}

	var update any
	if txn.db.returnASTValuesOnRead {
		valueAST, err := interfaceToValue(value)
		if err != nil {
			return err
		}
		if _, ok := valueAST.(ast.Object); !ok {
			return invalidPatchError(rootMustBeObjectMsg)
		}

		update = &updateAST{
			path:   storage.Path{},
			remove: false,
			value:  valueAST,
		}
	} else {
		if _, ok := value.(map[string]interface{}); !ok {
			return invalidPatchError(rootMustBeObjectMsg)
		}

		update = &updateRaw{
			path:   storage.Path{},
			remove: false,
			value:  value,
		}
	}

	txn.updates.Init()
	txn.updates.PushFront(update)
	return nil
}

func (txn *transaction) Commit() (result storage.TriggerEvent) {
	result.Context = txn.context
	for curr := txn.updates.Front(); curr != nil; curr = curr.Next() {
		action := curr.Value.(dataUpdate)
		updated := action.Apply(txn.db.value())
		txn.db.set(updated)

		result.Data = append(result.Data, storage.DataEvent{
			Path:    action.Path(),
			Data:    action.Value(),
			Removed: action.Remove(),
		})
	}
	for id, upd := range txn.policies {
		if upd.remove {
			delete(txn.db.policies, id)
		} else {
			txn.db.policies[id] = upd.value
		}

		result.Policy = append(result.Policy, storage.PolicyEvent{
			ID:      id,
			Data:    upd.value,
			Removed: upd.remove,
		})
	}
	return result
}

func pointer(v interface{}, path storage.Path) (interface{}, error) {
	if v, ok := v.(ast.Value); ok {
		return ptr.PtrAST(v, path)
	}
	return ptr.Ptr(v, path)
}

func deepcpy(v interface{}) interface{} {
	if v, ok := v.(ast.Value); ok {
		var cpy ast.Value

		switch data := v.(type) {
		case ast.Object:
			cpy = data.Copy()
		case *ast.Array:
			cpy = data.Copy()
		}

		return cpy
	}
	return deepcopy.DeepCopy(v)
}

func (txn *transaction) Read(path storage.Path) (interface{}, error) {

	if !txn.write {
		//return ptr.Ptr(txn.db.data, path)
		return pointer(txn.db.value(), path)
	}

	var merge []dataUpdate

	for curr := txn.updates.Front(); curr != nil; curr = curr.Next() {

		upd := curr.Value.(dataUpdate)

		if path.HasPrefix(upd.Path()) {
			if upd.Remove() {
				return nil, errors.NewNotFoundError(path)
			}
			return pointer(upd.Value(), path[len(upd.Path()):])
		}

		if upd.Path().HasPrefix(path) {
			merge = append(merge, upd)
		}
	}

	data, err := pointer(txn.db.value(), path)

	if err != nil {
		return nil, err
	}

	if len(merge) == 0 {
		return data, nil
	}

	cpy := deepcpy(data)

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

type dataUpdate interface {
	// FIXME: make all methods private
	Path() storage.Path
	Remove() bool
	Apply(interface{}) interface{}
	Relative(path storage.Path) dataUpdate
	Set(interface{})
	Value() interface{}
}

// update contains state associated with an update to be applied to the
// in-memory data store.
type updateRaw struct {
	path   storage.Path // data path modified by update
	remove bool         // indicates whether update removes the value at path
	value  interface{}  // value to add/replace at path (ignored if remove is true)
}

func (u *updateRaw) Remove() bool {
	return u.remove
}

type updateAST struct {
	path   storage.Path // data path modified by update
	remove bool         // indicates whether update removes the value at path
	value  ast.Value    // value to add/replace at path (ignored if remove is true)
}

func (u *updateAST) Path() storage.Path {
	return u.path
}

func (u *updateAST) Remove() bool {
	return u.remove
}

func (db *store) value() interface{} {
	if db.returnASTValuesOnRead {
		return db.dataAST
	}
	return db.data
}

// FIXME: use stronger typing here
func (db *store) set(data interface{}) {
	if db.returnASTValuesOnRead {
		db.dataAST = data.(ast.Object)
		return
	}
	db.data = data.(map[string]interface{})
}

func interfaceToValue(v interface{}) (ast.Value, error) {
	if v, ok := v.(ast.Value); ok {
		return v, nil
	}
	return ast.InterfaceToValue(v)
}

func (db *store) newUpdate(data interface{}, op storage.PatchOp, path storage.Path, idx int, value interface{}) (dataUpdate, error) {
	if db.returnASTValuesOnRead {
		astData, err := interfaceToValue(data)
		if err != nil {
			return nil, err
		}
		astValue, err := interfaceToValue(value)
		if err != nil {
			return nil, err
		}
		return newUpdateAST(astData, op, path, idx, astValue)
	}
	return newUpdateRaw(data, op, path, idx, value)
}

func newUpdateRaw(data interface{}, op storage.PatchOp, path storage.Path, idx int, value interface{}) (dataUpdate, error) {

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

func newUpdateAST(data interface{}, op storage.PatchOp, path storage.Path, idx int, value ast.Value) (*updateAST, error) {

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

func newUpdateArray(data []interface{}, op storage.PatchOp, path storage.Path, idx int, value interface{}) (dataUpdate, error) {

	if idx == len(path)-1 {
		if path[idx] == "-" || path[idx] == strconv.Itoa(len(data)) {
			if op != storage.AddOp {
				return nil, invalidPatchError("%v: invalid patch path", path)
			}
			cpy := make([]interface{}, len(data)+1)
			copy(cpy, data)
			cpy[len(data)] = value
			return &updateRaw{path[:len(path)-1], false, cpy}, nil
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
			return &updateRaw{path[:len(path)-1], false, cpy}, nil

		case storage.RemoveOp:
			cpy := make([]interface{}, len(data)-1)
			copy(cpy[:pos], data[:pos])
			copy(cpy[pos:], data[pos+1:])
			return &updateRaw{path[:len(path)-1], false, cpy}, nil

		default:
			cpy := make([]interface{}, len(data))
			copy(cpy, data)
			cpy[pos] = value
			return &updateRaw{path[:len(path)-1], false, cpy}, nil
		}
	}

	pos, err := ptr.ValidateArrayIndex(data, path[idx], path)
	if err != nil {
		return nil, err
	}

	return newUpdateRaw(data[pos], op, path, idx+1, value)
}

func newUpdateObject(data map[string]interface{}, op storage.PatchOp, path storage.Path, idx int, value interface{}) (dataUpdate, error) {

	if idx == len(path)-1 {
		switch op {
		case storage.ReplaceOp, storage.RemoveOp:
			if _, ok := data[path[idx]]; !ok {
				return nil, errors.NewNotFoundError(path)
			}
		}
		return &updateRaw{path, op == storage.RemoveOp, value}, nil
	}

	if data, ok := data[path[idx]]; ok {
		return newUpdateRaw(data, op, path, idx+1, value)
	}

	return nil, errors.NewNotFoundError(path)
}

func (u *updateRaw) Path() storage.Path {
	return u.path
}

func (u *updateRaw) Apply(data interface{}) interface{} {
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

func (u *updateRaw) Set(v interface{}) {
	u.value = v
}

func (u *updateRaw) Value() interface{} {
	return u.value
}

func (u *updateAST) Apply(v interface{}) interface{} {
	if len(u.path) == 0 {
		return u.value
	}

	data, ok := v.(ast.Value)
	if !ok {
		panic("illegal value type")
	}

	if u.remove {
		return removeInAst(data, u.path)
	}

	parent, err := ptr.PtrAST(data, u.path[:len(u.path)-1])
	if err != nil {
		panic(err)
	}

	key := u.path[len(u.path)-1]
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
	// FIXME: This will return data with updated values, but not updated hashes.
	// We probably need to recursively rebuild the AST.
	return data
}

func removeInAst(value ast.Value, path storage.Path) ast.Value {
	if len(path) == 0 {
		return value
	}

	switch value := value.(type) {
	case ast.Object:
		return removeInAstObject(value, path)
	case *ast.Array:
		return removeInAstArray(value, path)
	default:
		panic("illegal value type")
	}
}

func removeInAstObject(obj ast.Object, path storage.Path) ast.Object {
	key := ast.StringTerm(path[0])

	if len(path) == 1 {
		var items [][2]*ast.Term
		obj.Foreach(func(k *ast.Term, v *ast.Term) {
			if k.Equal(key) {
				return
			}
			items = append(items, [2]*ast.Term{k, v})
		})
		return ast.NewObject(items...)
	}

	if child := obj.Get(key); child != nil {
		updatedChild := removeInAst(child.Value, path[1:])
		obj.Insert(key, ast.NewTerm(updatedChild))
	}

	return obj
}

func removeInAstArray(arr *ast.Array, path storage.Path) *ast.Array {
	idx, err := strconv.Atoi(path[0])
	if err != nil {
		// We expect the path to be valid at this point.
		return arr
	}

	if idx < 0 || idx >= arr.Len() {
		return arr
	}

	if len(path) == 1 {
		var elems []*ast.Term
		for i := 0; i < arr.Len(); i++ {
			if i == idx {
				continue
			}
			elems = append(elems, arr.Elem(i))
		}
		return ast.NewArray(elems...)
	}

	updatedChild := removeInAst(arr.Elem(idx).Value, path[1:])
	arr.Set(idx, ast.NewTerm(updatedChild))
	return arr
}

func (u *updateAST) Set(v interface{}) {
	if v, ok := v.(ast.Value); ok {
		u.value = v
	}
	//panic("illegal value type") // FIXME: do conversion?
}

func (u *updateAST) Value() interface{} {
	return u.value
}

func (u *updateRaw) Relative(path storage.Path) dataUpdate {
	cpy := *u
	cpy.path = cpy.path[len(path):]
	return &cpy
}

func (u *updateAST) Relative(path storage.Path) dataUpdate {
	cpy := *u
	cpy.path = cpy.path[len(path):]
	return &cpy
}
