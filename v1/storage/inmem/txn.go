package inmem

import (
	"container/list"
	"encoding/json"
	"strconv"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
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
	policies *map[string]policyUpdate // Lazy pointer
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
		policies: nil, // Lazy initialization
		updates:  list.New(),
		context:  context,
	}
}

// Lazy initialization of policies
func (txn *transaction) initPolicies() {
	if txn.policies == nil {
		policies := make(map[string]policyUpdate, 2) // Start with small capacity
		txn.policies = &policies
	}
}

func (txn *transaction) ID() uint64 {
	return txn.xid
}

func (txn *transaction) Write(op storage.PatchOp, path storage.Path, value any) error {

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
					return NewNotFoundError(path)
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
				return NewNotFoundError(path)
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

func (txn *transaction) updateRoot(op storage.PatchOp, value any) error {
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
		if _, ok := value.(map[string]any); !ok {
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

	// Optimized preallocation
	if txn.updates.Len() > 0 {
		result.Data = make([]storage.DataEvent, 0, txn.updates.Len())
		for curr := txn.updates.Front(); curr != nil; curr = curr.Next() {
			action := curr.Value.(dataUpdate)
			txn.db.data = action.Apply(txn.db.data)
			result.Data = append(result.Data, storage.DataEvent{
				Path:    action.Path(),
				Data:    action.Value(),
				Removed: action.Remove(),
			})
		}
	}

	if txn.policies != nil && len(*txn.policies) > 0 {
		result.Policy = make([]storage.PolicyEvent, 0, len(*txn.policies))
		for id, upd := range *txn.policies {
			if upd.remove {
				delete(*txn.db.policies, id)
			} else {
				(*txn.db.policies)[id] = upd.value
			}

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
		return ValuePtr(v, path)
	}
	return Ptr(v, path)
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
	return DeepCopy(v)
}

func (txn *transaction) Read(path storage.Path) (any, error) {
	if !txn.write {
		return pointer(txn.db.data, path)
	}

	var merge []dataUpdate
	hasUpdates := false

	for curr := txn.updates.Front(); curr != nil; curr = curr.Next() {
		upd := curr.Value.(dataUpdate)

		if path.HasPrefix(upd.Path()) {
			if upd.Remove() {
				return nil, NewNotFoundError(path)
			}
			return pointer(upd.Value(), path[len(upd.Path()):])
		}

		if upd.Path().HasPrefix(path) {
			if !hasUpdates {
				// Lazy initialization only when necessary
				merge = make([]dataUpdate, 0, 4) // Start with small capacity
				hasUpdates = true
			}
			merge = append(merge, upd)
		}
	}

	data, err := pointer(txn.db.data, path)
	if err != nil {
		return nil, err
	}

	if !hasUpdates {
		return data, nil
	}

	cpy := deepcpy(data)
	for _, update := range merge {
		cpy = update.Relative(path).Apply(cpy)
	}

	return cpy, nil
}

func (txn *transaction) ListPolicies() []string {
	dbPolicyCount := len(*txn.db.policies)

	// Optimized capacity estimation
	var capacity int
	if txn.policies != nil {
		activeCount := 0
		removedCount := 0
		for _, update := range *txn.policies {
			if update.remove {
				removedCount++
			} else {
				activeCount++
			}
		}
		capacity = dbPolicyCount - removedCount + activeCount
	} else {
		capacity = dbPolicyCount
	}

	if capacity < 0 {
		capacity = 0
	}

	ids := make([]string, 0, capacity)

	for id := range *txn.db.policies {
		if txn.policies == nil || !(*txn.policies)[id].remove {
			if txn.policies != nil {
				if _, ok := (*txn.policies)[id]; ok {
					continue // Skip if there is an update
				}
			}
			ids = append(ids, id)
		}
	}

	if txn.policies != nil {
		for id, update := range *txn.policies {
			if !update.remove {
				ids = append(ids, id)
			}
		}
	}

	return ids
}

func (txn *transaction) GetPolicy(id string) ([]byte, error) {
	if txn.policies != nil {
		if update, ok := (*txn.policies)[id]; ok {
			if !update.remove {
				return update.value, nil
			}
			return nil, NewNotFoundErrorf("policy id %q", id)
		}
	}
	if exist, ok := (*txn.db.policies)[id]; ok {
		return exist, nil
	}
	return nil, NewNotFoundErrorf("policy id %q", id)
}

func (txn *transaction) UpsertPolicy(id string, bs []byte) error {
	if !txn.write {
		return &storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: "policy write during read transaction",
		}
	}
	txn.initPolicies()
	(*txn.policies)[id] = policyUpdate{bs, false}
	return nil
}

func (txn *transaction) DeletePolicy(id string) error {
	if !txn.write {
		return &storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: "policy write during read transaction",
		}
	}
	txn.initPolicies()
	(*txn.policies)[id] = policyUpdate{nil, true}
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

func (db *store) newUpdate(data any, op storage.PatchOp, path storage.Path, idx int, value any) (dataUpdate, error) {
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

func newUpdateRaw(data any, op storage.PatchOp, path storage.Path, idx int, value any) (dataUpdate, error) {

	switch data.(type) {
	case nil, bool, json.Number, string:
		return nil, NewNotFoundError(path)
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
				return nil, invalidPatchError("%v: invalid patch path", path)
			}
			cpy := make([]any, len(data)+1)
			copy(cpy, data)
			cpy[len(data)] = value
			return &updateRaw{path[:len(path)-1], false, cpy}, nil
		}

		pos, err := ValidateArrayIndex(data, path[idx], path)
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

	pos, err := ValidateArrayIndex(data, path[idx], path)
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
				return nil, NewNotFoundError(path)
			}
		}
		return &updateRaw{path, op == storage.RemoveOp, value}, nil
	}

	if data, ok := data[path[idx]]; ok {
		return newUpdateRaw(data, op, path, idx+1, value)
	}

	return nil, NewNotFoundError(path)
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
	parent, err := Ptr(data, u.path[:len(u.path)-1])
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
