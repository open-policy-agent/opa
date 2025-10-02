// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem

import (
	"container/list"
	"encoding/json"
	"slices"
	"strconv"

	"github.com/open-policy-agent/opa/internal/deepcopy"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/internal/errors"
	"github.com/open-policy-agent/opa/v1/storage/internal/ptr"
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
	db       *store
	updates  *list.List
	context  *storage.Context
	policies map[string]policyUpdate
	xid      uint64
	write    bool
	stale    bool
}

type policyUpdate struct {
	value  []byte
	remove bool
}

func (txn *transaction) ID() uint64 {
	return txn.xid
}

func (txn *transaction) Write(op storage.PatchOp, path storage.Path, value any) error {
	if !txn.write {
		return &storage.Error{Code: storage.InvalidTransactionErr, Message: "data write during read transaction"}
	}

	if txn.updates == nil {
		txn.updates = list.New()
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
					return errors.NotFoundErr
				}
			}
			// If the last update has the same path and value, we have nothing to do.
			if txn.db.returnASTValuesOnRead {
				if astValue, ok := update.Value().(ast.Value); ok {
					if equalsValue(value, astValue) {
						return nil
					}
				}
			} else if comparableEquals(update.Value(), value) {
				return nil
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
				return errors.NotFoundErr
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

	update, err := txn.db.newUpdate(txn.db.data, op, path, 0, value)
	if err != nil {
		return err
	}

	txn.updates.PushFront(update)
	return nil
}

func comparableEquals(a, b any) bool {
	switch a := a.(type) {
	case nil:
		return b == nil
	case bool:
		if vb, ok := b.(bool); ok {
			return vb == a
		}
	case string:
		if vs, ok := b.(string); ok {
			return vs == a
		}
	case json.Number:
		if vn, ok := b.(json.Number); ok {
			return vn == a
		}
	}
	return false
}

func (txn *transaction) updateRoot(op storage.PatchOp, value any) error {
	if op == storage.RemoveOp {
		return errors.RootCannotBeRemovedErr
	}

	var update any
	if txn.db.returnASTValuesOnRead {
		valueAST, err := ast.InterfaceToValue(value)
		if err != nil {
			return err
		}
		if _, ok := valueAST.(ast.Object); !ok {
			return errors.RootMustBeObjectErr
		}

		update = &updateAST{
			path:   storage.RootPath,
			remove: false,
			value:  valueAST,
		}
	} else {
		if _, ok := value.(map[string]any); !ok {
			return errors.RootMustBeObjectErr
		}

		update = &updateRaw{
			path:   storage.RootPath,
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

	if txn.updates != nil {
		if len(txn.db.triggers) > 0 {
			result.Data = slices.Grow(result.Data, txn.updates.Len())
		}

		for curr := txn.updates.Front(); curr != nil; curr = curr.Next() {
			action := curr.Value.(dataUpdate)
			txn.db.data = action.Apply(txn.db.data)

			if len(txn.db.triggers) > 0 {
				result.Data = append(result.Data, storage.DataEvent{
					Path:    action.Path(),
					Data:    action.Value(),
					Removed: action.Remove(),
				})
			}
		}
	}

	if len(txn.policies) > 0 && len(txn.db.triggers) > 0 {
		result.Policy = slices.Grow(result.Policy, len(txn.policies))
	}

	for id, upd := range txn.policies {
		if upd.remove {
			delete(txn.db.policies, id)
		} else {
			txn.db.policies[id] = upd.value
		}

		if len(txn.db.triggers) > 0 {
			result.Policy = append(result.Policy, storage.PolicyEvent{
				ID:      id,
				Data:    upd.value,
				Removed: upd.remove,
			})
		}
	}
	return result
}

func pointer(v any, path storage.Path) (any, error) {
	if v, ok := v.(ast.Value); ok {
		return ptr.ValuePtr(v, path)
	}
	return ptr.Ptr(v, path)
}

func deepcpy(v any) any {
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

func (txn *transaction) Read(path storage.Path) (any, error) {
	if !txn.write || txn.updates == nil {
		return pointer(txn.db.data, path)
	}

	var merge []dataUpdate

	for curr := txn.updates.Front(); curr != nil; curr = curr.Next() {

		upd := curr.Value.(dataUpdate)

		if path.HasPrefix(upd.Path()) {
			if upd.Remove() {
				return nil, errors.NotFoundErr
			}
			return pointer(upd.Value(), path[len(upd.Path()):])
		}

		if upd.Path().HasPrefix(path) {
			merge = append(merge, upd)
		}
	}

	data, err := pointer(txn.db.data, path)

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

func (txn *transaction) ListPolicies() (ids []string) {
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
	if txn.policies != nil {
		if update, ok := txn.policies[id]; ok {
			if !update.remove {
				return update.value, nil
			}
			return nil, errors.NewNotFoundErrorf("policy id %q", id)
		}
	}
	if exist, ok := txn.db.policies[id]; ok {
		return exist, nil
	}
	return nil, errors.NewNotFoundErrorf("policy id %q", id)
}

func (txn *transaction) UpsertPolicy(id string, bs []byte) error {
	return txn.updatePolicy(id, policyUpdate{bs, false})
}

func (txn *transaction) DeletePolicy(id string) error {
	return txn.updatePolicy(id, policyUpdate{nil, true})
}

func (txn *transaction) updatePolicy(id string, update policyUpdate) error {
	if !txn.write {
		return &storage.Error{Code: storage.InvalidTransactionErr, Message: "policy write during read transaction"}
	}

	if txn.policies == nil {
		txn.policies = map[string]policyUpdate{id: update}
	} else {
		txn.policies[id] = update
	}

	return nil
}

type dataUpdate interface {
	Path() storage.Path
	Remove() bool
	Apply(any) any
	Relative(path storage.Path) dataUpdate
	Set(any)
	Value() any
}

// update contains state associated with an update to be applied to the
// in-memory data store.
type updateRaw struct {
	path   storage.Path // data path modified by update
	remove bool         // indicates whether update removes the value at path
	value  any          // value to add/replace at path (ignored if remove is true)
}

func equalsValue(a any, v ast.Value) bool {
	if a, ok := a.(ast.Value); ok {
		return a.Compare(v) == 0
	}
	switch a := a.(type) {
	case nil:
		return v == ast.NullValue
	case bool:
		if vb, ok := v.(ast.Boolean); ok {
			return bool(vb) == a
		}
	case string:
		if vs, ok := v.(ast.String); ok {
			return string(vs) == a
		}
	}

	return false
}

func (db *store) newUpdate(data any, op storage.PatchOp, path storage.Path, idx int, value any) (dataUpdate, error) {
	if db.returnASTValuesOnRead {
		astData, err := ast.InterfaceToValue(data)
		if err != nil {
			return nil, err
		}
		astValue, err := ast.InterfaceToValue(value)
		if err != nil {
			return nil, err
		}
		return newUpdateAST(astData, op, path, idx, astValue)
	}
	return newUpdateRaw(data, op, path, idx, value)
}

func newUpdateRaw(data any, op storage.PatchOp, path storage.Path, idx int, value any) (dataUpdate, error) {
	switch data.(type) {
	case nil, bool, json.Number, string:
		return nil, errors.NotFoundErr
	}

	switch data := data.(type) {
	case map[string]any:
		return newUpdateObject(data, op, path, idx, value)

	case []any:
		return newUpdateArray(data, op, path, idx, value)
	}

	return nil, &storage.Error{
		Code:    storage.InternalErr,
		Message: "invalid data value encountered",
	}
}

func newUpdateArray(data []any, op storage.PatchOp, path storage.Path, idx int, value any) (dataUpdate, error) {
	if idx == len(path)-1 {
		if path[idx] == "-" || path[idx] == strconv.Itoa(len(data)) {
			if op != storage.AddOp {
				return nil, errors.NewInvalidPatchError("%v: invalid patch path", path)
			}
			cpy := make([]any, len(data)+1)
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
			cpy := make([]any, len(data)+1)
			copy(cpy[:pos], data[:pos])
			copy(cpy[pos+1:], data[pos:])
			cpy[pos] = value
			return &updateRaw{path[:len(path)-1], false, cpy}, nil

		case storage.RemoveOp:
			cpy := make([]any, len(data)-1)
			copy(cpy[:pos], data[:pos])
			copy(cpy[pos:], data[pos+1:])
			return &updateRaw{path[:len(path)-1], false, cpy}, nil

		default:
			cpy := make([]any, len(data))
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

func newUpdateObject(data map[string]any, op storage.PatchOp, path storage.Path, idx int, value any) (dataUpdate, error) {

	if idx == len(path)-1 {
		switch op {
		case storage.ReplaceOp, storage.RemoveOp:
			if _, ok := data[path[idx]]; !ok {
				return nil, errors.NotFoundErr
			}
		}
		return &updateRaw{path, op == storage.RemoveOp, value}, nil
	}

	if data, ok := data[path[idx]]; ok {
		return newUpdateRaw(data, op, path, idx+1, value)
	}

	return nil, errors.NotFoundErr
}

func (u *updateRaw) Remove() bool {
	return u.remove
}

func (u *updateRaw) Path() storage.Path {
	return u.path
}

func (u *updateRaw) Apply(data any) any {
	if len(u.path) == 0 {
		return u.value
	}
	parent, err := ptr.Ptr(data, u.path[:len(u.path)-1])
	if err != nil {
		panic(err)
	}
	key := u.path[len(u.path)-1]
	if u.remove {
		obj := parent.(map[string]any)
		delete(obj, key)
		return data
	}
	switch parent := parent.(type) {
	case map[string]any:
		if parent == nil {
			parent = make(map[string]any, 1)
		}
		parent[key] = u.value
	case []any:
		idx, err := strconv.Atoi(key)
		if err != nil {
			panic(err)
		}
		parent[idx] = u.value
	}
	return data
}

func (u *updateRaw) Set(v any) {
	u.value = v
}

func (u *updateRaw) Value() any {
	return u.value
}

func (u *updateRaw) Relative(path storage.Path) dataUpdate {
	cpy := *u
	cpy.path = cpy.path[len(path):]
	return &cpy
}
