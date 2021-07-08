// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	badger "github.com/dgraph-io/badger/v3"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/internal/errors"
	"github.com/open-policy-agent/opa/storage/internal/ptr"
	"github.com/open-policy-agent/opa/util"
)

type transaction struct {
	underlying *badger.Txn      // handle for the underlying badger transaction
	partitions *partitionTrie   // index for partitioning structure in underlying store
	pm         *pathMapper      // used for mapping between logical storage paths and actual storage keys
	db         *Store           // handle for the database this transaction was created on
	xid        uint64           // unique id for this transaction
	stale      bool             // bit to indicate if the transaction was already aborted/committed
	write      bool             // bit to indicate if the transaction may perform writes
	context    *storage.Context // context supplied by the caller to be included in triggers
}

func newTransaction(xid uint64, write bool, underlying *badger.Txn, context *storage.Context, pm *pathMapper, trie *partitionTrie, db *Store) *transaction {
	return &transaction{
		underlying: underlying,
		partitions: trie,
		pm:         pm,
		db:         db,
		xid:        xid,
		stale:      false,
		write:      write,
		context:    context,
	}
}

func (txn *transaction) ID() uint64 {
	return txn.xid
}

func (txn *transaction) Commit(context.Context) (storage.TriggerEvent, error) {
	// TODO(tsandall): This implementation does not provide any data or policy
	// events because they are of minimal value given that the transcation
	// cannot be used for reads once the commit finishes. This differs from the
	// in-memory store.
	//
	// We should figure out how to remove the code in the plugin manager that
	// performs reads on committed transactions.
	txn.stale = true
	return storage.TriggerEvent{Context: txn.context}, wrapError(txn.underlying.Commit())
}

func (txn *transaction) Abort(context.Context) {
	txn.stale = true
	txn.underlying.Discard()
}

func (txn *transaction) Read(_ context.Context, path storage.Path) (interface{}, error) {

	i, node := txn.partitions.Find(path)

	if node == nil {

		key, err := txn.pm.DataPath2Key(path[:i])
		if err != nil {
			return nil, err
		}

		value, err := txn.readOne(key)
		if err != nil {
			return nil, err
		}

		return ptr.Ptr(value, path[i:])
	}

	key, err := txn.pm.DataPrefix2Key(path[:i])
	if err != nil {
		return nil, err
	}

	return txn.readMultiple(i, key)
}

func (txn *transaction) readMultiple(offset int, prefix []byte) (interface{}, error) {

	result := map[string]interface{}{}

	it := txn.underlying.NewIterator(badger.IteratorOptions{Prefix: prefix})
	defer it.Close()

	var keybuf, valbuf []byte

	for it.Rewind(); it.Valid(); it.Next() {

		keybuf = it.Item().KeyCopy(keybuf)
		path, err := txn.pm.DataKey2Path(keybuf)
		if err != nil {
			return nil, err
		}

		valbuf, err = it.Item().ValueCopy(valbuf)
		if err != nil {
			return nil, wrapError(err)
		}

		var value interface{}
		if err := deserialize(valbuf, &value); err != nil {
			return nil, err
		}

		node := result

		for i := offset; i < len(path)-1; i++ {
			child, ok := node[path[i]]
			if !ok {
				child = map[string]interface{}{}
				node[path[i]] = child
			}
			childObj, ok := child.(map[string]interface{})
			if !ok {
				return nil, &storage.Error{Code: storage.InternalErr, Message: fmt.Sprintf("corrupt key-value: %s", keybuf)}
			}
			node = childObj
		}

		node[path[len(path)-1]] = value
	}

	if len(result) == 0 {
		return nil, errNotFound
	}

	return result, nil
}

func (txn *transaction) readOne(key []byte) (interface{}, error) {

	item, err := txn.underlying.Get(key)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, errNotFound
		}
		return nil, wrapError(err)
	}

	var val interface{}

	err = item.Value(func(bs []byte) error {
		return deserialize(bs, &val)
	})

	return val, wrapError(err)
}

type update struct {
	key    []byte
	value  []byte
	delete bool
}

func (txn *transaction) Write(_ context.Context, op storage.PatchOp, path storage.Path, value interface{}) error {

	updates, err := txn.partitionWrite(op, path, value)
	if err != nil {
		return err
	}

	for _, u := range updates {
		if u.delete {
			if err := txn.underlying.Delete(u.key); err != nil {
				return wrapError(err)
			}
		} else {
			if err := txn.underlying.Set(u.key, u.value); err != nil {
				return wrapError(err)
			}
		}
	}

	return nil
}

func (txn *transaction) partitionWrite(op storage.PatchOp, path storage.Path, value interface{}) ([]update, error) {

	i, node := txn.partitions.Find(path)

	if node == nil {
		if len(path) < i {
			panic("unreachable")
		}

		if len(path) == i {
			return txn.partitionWriteOne(op, path, value)
		}

		key, err := txn.pm.DataPath2Key(path[:i])
		if err != nil {
			return nil, err
		}

		curr, err := txn.readOne(key)
		if err != nil {
			return nil, err
		}

		modified, err := patch(curr, op, path[i:], value)
		if err != nil {
			return nil, err
		}

		bs, err := serialize(modified)
		if err != nil {
			return nil, err
		}

		return []update{{key: key, value: bs}}, nil
	}

	key, err := txn.pm.DataPrefix2Key(path)
	if err != nil {
		return nil, err
	}

	it := txn.underlying.NewIterator(badger.IteratorOptions{Prefix: key})
	defer it.Close()

	var updates []update

	for it.Rewind(); it.Valid(); it.Next() {
		updates = append(updates, update{key: it.Item().KeyCopy(nil), delete: true})
	}

	if op == storage.RemoveOp {
		return updates, nil
	}

	return txn.partitionWriteMultiple(node, path, value, updates)
}

func (txn *transaction) partitionWriteMultiple(node *partitionTrie, path storage.Path, value interface{}, result []update) ([]update, error) {

	// NOTE(tsandall): value must be an object so that it can be partitioned; in
	// the future, arrays could be supported but that requires investigation.
	obj, ok := value.(map[string]interface{})
	if !ok {
		return nil, &storage.Error{Code: storage.InvalidPatchErr, Message: "value cannot be partitioned"}
	}

	for k, v := range obj {
		child := append(path, k)
		next, ok := node.partitions[k]
		if !ok {
			key, err := txn.pm.DataPath2Key(child)
			if err != nil {
				return nil, err
			}
			bs, err := serialize(v)
			if err != nil {
				return nil, err
			}
			result = append(result, update{key: key, value: bs})
		} else {
			var err error
			result, err = txn.partitionWriteMultiple(next, child, v, result)
			if err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

func (txn *transaction) partitionWriteOne(op storage.PatchOp, path storage.Path, value interface{}) ([]update, error) {
	key, err := txn.pm.DataPath2Key(path)
	if err != nil {
		return nil, err
	}

	if op == storage.RemoveOp {
		return []update{{key: key, delete: true}}, nil
	}

	val, err := serialize(value)
	if err != nil {
		return nil, err
	}

	return []update{{key: key, value: val}}, nil
}

func (txn *transaction) ListPolicies(context.Context) ([]string, error) {

	var result []string

	it := txn.underlying.NewIterator(badger.IteratorOptions{
		Prefix: txn.pm.PolicyIDPrefix(),
	})

	defer it.Close()

	var key []byte

	for it.Rewind(); it.Valid(); it.Next() {
		item := it.Item()
		key = item.KeyCopy(key)
		result = append(result, txn.pm.PolicyKey2ID(key))
	}

	return result, nil
}

func (txn *transaction) GetPolicy(_ context.Context, id string) ([]byte, error) {
	item, err := txn.underlying.Get(txn.pm.PolicyID2Key(id))
	if err != nil {
		return nil, err
	}
	bs, err := item.ValueCopy(nil)
	return bs, wrapError(err)
}

func (txn *transaction) UpsertPolicy(_ context.Context, id string, bs []byte) error {
	return wrapError(txn.underlying.Set(txn.pm.PolicyID2Key(id), bs))
}

func (txn *transaction) DeletePolicy(_ context.Context, id string) error {
	return wrapError(txn.underlying.Delete(txn.pm.PolicyID2Key(id)))
}

func serialize(value interface{}) ([]byte, error) {
	bs, err := json.Marshal(value)
	return bs, wrapError(err)
}

func deserialize(bs []byte, result interface{}) error {
	d := util.NewJSONDecoder(bytes.NewReader(bs))
	return wrapError(d.Decode(&result))
}

func patch(data interface{}, op storage.PatchOp, path storage.Path, value interface{}) (interface{}, error) {

	if len(path) == 0 {
		panic("unreachable")
	}

	// Base case: mutate the data value in-place.
	if len(path) == 1 {
		switch x := data.(type) {
		case map[string]interface{}:
			switch op {
			case storage.RemoveOp:
				key := path[len(path)-1]
				if _, ok := x[key]; !ok {
					return nil, errors.NewNotFoundError(path)
				}
				delete(x, key)
				return x, nil
			case storage.ReplaceOp:
				key := path[len(path)-1]
				if _, ok := x[key]; !ok {
					return nil, errors.NewNotFoundError(path)
				}
				x[key] = value
				return x, nil
			case storage.AddOp:
				key := path[len(path)-1]
				x[key] = value
				return x, nil
			}
		case []interface{}:
			if path[0] == "-" {
				return append(x, value), nil
			}
			idx, err := ptr.ValidateArrayIndex(x, path[0], path)
			if err != nil {
				return nil, err
			}
			x[idx] = value
			return x, nil
		default:
			return nil, errors.NewNotFoundError(path)
		}
	}

	// Recurse on the value located at the next part of the path.
	key := path[0]

	switch x := data.(type) {
	case map[string]interface{}:
		child, ok := x[key]
		if !ok {
			return nil, errors.NewNotFoundError(path)
		}
		modified, err := patch(child, op, path[1:], value)
		if err != nil {
			return nil, err
		}
		x[key] = modified
		return x, nil
	case []interface{}:
		idx, err := ptr.ValidateArrayIndex(x, path[0], path)
		if err != nil {
			return nil, err
		}
		modified, err := patch(x[idx], op, path[1:], value)
		if err != nil {
			return nil, err
		}
		x[idx] = modified
		return x, nil
	default:
		return nil, errors.NewNotFoundError(path)
	}

}
