// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package disk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	badger "github.com/dgraph-io/badger/v3"

	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/internal/errors"
	"github.com/open-policy-agent/opa/storage/internal/ptr"
	"github.com/open-policy-agent/opa/util"
)

const (
	readValueBytesCounter = "disk_read_bytes"
	readKeysCounter       = "disk_read_keys"
	writtenKeysCounter    = "disk_written_keys"
	deletedKeysCounter    = "disk_deleted_keys"

	commitTimer = "disk_commit"
	readTimer   = "disk_read"
	writeTimer  = "disk_write"
)

type transaction struct {
	underlying *badger.Txn          // handle for the underlying badger transaction
	partitions *partitionTrie       // index for partitioning structure in underlying store
	pm         *pathMapper          // used for mapping between logical storage paths and actual storage keys
	db         *Store               // handle for the database this transaction was created on
	xid        uint64               // unique id for this transaction
	stale      bool                 // bit to indicate if the transaction was already aborted/committed
	write      bool                 // bit to indicate if the transaction may perform writes
	event      storage.TriggerEvent // constructed as we go, supplied by the caller to be included in triggers
	metrics    metrics.Metrics      // per-transaction metrics
}

func newTransaction(xid uint64, write bool, underlying *badger.Txn, context *storage.Context, pm *pathMapper, trie *partitionTrie, db *Store) *transaction {

	// Even if the caller is not interested, these will contribute
	// to the prometheus metrics on commit.
	var m metrics.Metrics
	if context != nil {
		m = context.Metrics()
	}
	if m == nil {
		m = metrics.New()
	}

	return &transaction{
		underlying: underlying,
		partitions: trie,
		pm:         pm,
		db:         db,
		xid:        xid,
		stale:      false,
		write:      write,
		event: storage.TriggerEvent{
			Context: context,
		},
		metrics: m,
	}
}

func (txn *transaction) ID() uint64 {
	return txn.xid
}

// Commit will commit the underlying transaction, and forward the per-transaction
// metrics into prometheus metrics.
// NOTE(sr): aborted transactions are not measured
func (txn *transaction) Commit(context.Context) (storage.TriggerEvent, error) {
	txn.stale = true
	txn.metrics.Timer(commitTimer).Start()
	err := wrapError(txn.underlying.Commit())
	txn.metrics.Timer(commitTimer).Stop()

	if err != nil {
		return txn.event, err
	}

	m := txn.metrics.All()
	if txn.write {
		forwardMetric(m, readKeysCounter, keysReadPerStoreWrite)
		forwardMetric(m, readKeysCounter, keysReadPerStoreWrite)
		forwardMetric(m, writtenKeysCounter, keysWrittenPerStoreWrite)
		forwardMetric(m, deletedKeysCounter, keysDeletedPerStoreWrite)
		forwardMetric(m, readValueBytesCounter, bytesReadPerStoreWrite)
	} else {
		forwardMetric(m, readKeysCounter, keysReadPerStoreRead)
		forwardMetric(m, readValueBytesCounter, bytesReadPerStoreRead)
	}
	return txn.event, nil
}

func (txn *transaction) Abort(context.Context) {
	txn.stale = true
	txn.underlying.Discard()
}

func (txn *transaction) Read(ctx context.Context, path storage.Path) (interface{}, error) {
	txn.metrics.Timer(readTimer).Start()
	defer txn.metrics.Timer(readTimer).Stop()

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

	return txn.readMultiple(ctx, i, key)
}

func (txn *transaction) readMultiple(ctx context.Context, offset int, prefix []byte) (interface{}, error) {

	result := map[string]interface{}{}

	it := txn.underlying.NewIterator(badger.IteratorOptions{Prefix: prefix})
	defer it.Close()

	var keybuf, valbuf []byte
	var count uint64

	for it.Rewind(); it.Valid(); it.Next() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		count++

		keybuf = it.Item().KeyCopy(keybuf)
		path, err := txn.pm.DataKey2Path(keybuf)
		if err != nil {
			return nil, err
		}

		valbuf, err = it.Item().ValueCopy(valbuf)
		if err != nil {
			return nil, wrapError(err)
		}
		txn.metrics.Counter(readValueBytesCounter).Add(uint64(len(valbuf)))

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

	txn.metrics.Counter(readKeysCounter).Add(count)

	if len(result) == 0 {
		return nil, errNotFound
	}

	return result, nil
}

func (txn *transaction) readOne(key []byte) (interface{}, error) {
	txn.metrics.Counter(readKeysCounter).Add(1)

	item, err := txn.underlying.Get(key)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, errNotFound
		}
		return nil, wrapError(err)
	}

	var val interface{}

	err = item.Value(func(bs []byte) error {
		txn.metrics.Counter(readValueBytesCounter).Add(uint64(len(bs)))
		return deserialize(bs, &val)
	})

	return val, wrapError(err)
}

type update struct {
	key    []byte
	value  []byte
	data   interface{}
	delete bool
}

func (txn *transaction) Write(_ context.Context, op storage.PatchOp, path storage.Path, value interface{}) error {
	txn.metrics.Timer(writeTimer).Start()
	defer txn.metrics.Timer(writeTimer).Stop()

	updates, err := txn.partitionWrite(op, path, value)
	if err != nil {
		return err
	}

	for _, u := range updates {
		if u.delete {
			if err := txn.underlying.Delete(u.key); err != nil {
				return err
			}
			txn.metrics.Counter(deletedKeysCounter).Add(1)
		} else {
			if err := txn.underlying.Set(u.key, u.value); err != nil {
				return err
			}
			txn.metrics.Counter(writtenKeysCounter).Add(1)
		}

		txn.event.Data = append(txn.event.Data, storage.DataEvent{
			Path:    path,   // ?
			Data:    u.data, // nil if delete == true
			Removed: u.delete,
		})
	}
	return nil
}

func (txn *transaction) partitionWrite(op storage.PatchOp, path storage.Path, value interface{}) ([]update, error) {

	if op == storage.RemoveOp && len(path) == 0 {
		return nil, &storage.Error{
			Code:    storage.InvalidPatchErr,
			Message: "root cannot be removed",
		}
	}

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
		if err != nil && err != errNotFound {
			return nil, err
		}

		modified, err := patch(curr, op, path, i, value)
		if err != nil {
			return nil, err
		}

		bs, err := serialize(modified)
		if err != nil {
			return nil, err
		}
		return []update{{key: key, value: bs, data: modified}}, nil
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
		txn.metrics.Counter(readKeysCounter).Add(1)
	}

	if op == storage.RemoveOp {
		return updates, nil
	}

	return txn.partitionWriteMultiple(node, path, value, updates)
}

func (txn *transaction) partitionWriteMultiple(node *partitionTrie, path storage.Path, value interface{}, result []update) ([]update, error) {
	// NOTE(tsandall): value must be an object so that it can be partitioned; in
	// the future, arrays could be supported but that requires investigation.

	switch v := value.(type) {
	case map[string]interface{}:
		bs, err := serialize(v)
		if err != nil {
			return nil, err
		}
		return txn.doPartitionWriteMultiple(node, path, bs, result)
	case map[string]json.RawMessage:
		bs, err := serialize(v)
		if err != nil {
			return nil, err
		}
		return txn.doPartitionWriteMultiple(node, path, bs, result)
	case json.RawMessage:
		return txn.doPartitionWriteMultiple(node, path, v, result)
	case []uint8:
		return txn.doPartitionWriteMultiple(node, path, v, result)
	}

	return nil, &storage.Error{Code: storage.InvalidPatchErr, Message: "value cannot be partitioned"}
}

func (txn *transaction) doPartitionWriteMultiple(node *partitionTrie, path storage.Path, bs []byte, result []update) ([]update, error) {
	var obj map[string]json.RawMessage
	err := util.Unmarshal(bs, &obj)
	if err != nil {
		return nil, &storage.Error{Code: storage.InvalidPatchErr, Message: "value cannot be partitioned"}
	}

	for k, v := range obj {
		child := append(path, k)
		next, ok := node.partitions[k]
		if !ok { // try wildcard
			next, ok = node.partitions[pathWildcard]
		}
		if ok {
			var err error
			result, err = txn.partitionWriteMultiple(next, child, v, result)
			if err != nil {
				return nil, err
			}
			continue
		}

		key, err := txn.pm.DataPath2Key(child)
		if err != nil {
			return nil, err
		}
		bs, err := serialize(v)
		if err != nil {
			return nil, err
		}
		result = append(result, update{key: key, value: bs, data: v})
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

	return []update{{key: key, value: val, data: value}}, nil
}

func (txn *transaction) ListPolicies(ctx context.Context) ([]string, error) {

	var result []string

	it := txn.underlying.NewIterator(badger.IteratorOptions{
		Prefix: txn.pm.PolicyIDPrefix(),
	})

	defer it.Close()

	var key []byte

	for it.Rewind(); it.Valid(); it.Next() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		txn.metrics.Counter(readKeysCounter).Add(1)
		item := it.Item()
		key = item.KeyCopy(key)
		result = append(result, txn.pm.PolicyKey2ID(key))
	}

	return result, nil
}

func (txn *transaction) GetPolicy(_ context.Context, id string) ([]byte, error) {
	txn.metrics.Counter(readKeysCounter).Add(1)
	item, err := txn.underlying.Get(txn.pm.PolicyID2Key(id))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, errors.NewNotFoundErrorf("policy id %q", id)
		}
		return nil, err
	}
	bs, err := item.ValueCopy(nil)
	txn.metrics.Counter(readValueBytesCounter).Add(uint64(len(bs)))
	return bs, wrapError(err)
}

func (txn *transaction) UpsertPolicy(_ context.Context, id string, bs []byte) error {
	if err := txn.underlying.Set(txn.pm.PolicyID2Key(id), bs); err != nil {
		return wrapError(err)
	}
	txn.metrics.Counter(writtenKeysCounter).Add(1)
	txn.event.Policy = append(txn.event.Policy, storage.PolicyEvent{
		ID:   id,
		Data: bs,
	})
	return nil
}

func (txn *transaction) DeletePolicy(_ context.Context, id string) error {
	if err := txn.underlying.Delete(txn.pm.PolicyID2Key(id)); err != nil {
		return wrapError(err)
	}
	txn.metrics.Counter(deletedKeysCounter).Add(1)
	txn.event.Policy = append(txn.event.Policy, storage.PolicyEvent{
		ID:      id,
		Removed: true,
	})
	return nil
}

func serialize(value interface{}) ([]byte, error) {
	val, ok := value.([]byte)
	if ok {
		return val, nil
	}

	bs, err := json.Marshal(value)
	return bs, wrapError(err)
}

func deserialize(bs []byte, result interface{}) error {
	d := util.NewJSONDecoder(bytes.NewReader(bs))
	return wrapError(d.Decode(&result))
}

func patch(data interface{}, op storage.PatchOp, path storage.Path, idx int, value interface{}) (interface{}, error) {
	if idx == len(path) {
		panic("unreachable")
	}

	val := value
	switch v := value.(type) {
	case json.RawMessage:
		var obj map[string]json.RawMessage
		err := util.Unmarshal(v, &obj)
		if err == nil {
			val = obj
		} else {
			var obj interface{}
			err := util.Unmarshal(v, &obj)
			if err != nil {
				return nil, err
			}
			val = obj
		}
	case []uint8:
		var obj map[string]json.RawMessage
		err := util.Unmarshal(v, &obj)
		if err == nil {
			val = obj
		} else {
			var obj interface{}
			err := util.Unmarshal(v, &obj)
			if err != nil {
				return nil, err
			}
			val = obj
		}
	}

	// Base case: mutate the data value in-place.
	if len(path) == idx+1 { // last element
		switch x := data.(type) {
		case map[string]interface{}:
			key := path[len(path)-1]
			switch op {
			case storage.RemoveOp:
				if _, ok := x[key]; !ok {
					return nil, errors.NewNotFoundError(path)
				}
				delete(x, key)
				return x, nil
			case storage.ReplaceOp:
				if _, ok := x[key]; !ok {
					return nil, errors.NewNotFoundError(path)
				}
				x[key] = val
				return x, nil
			case storage.AddOp:
				x[key] = val
				return x, nil
			}
		case []interface{}:
			switch op {
			case storage.AddOp:
				if path[idx] == "-" || path[idx] == strconv.Itoa(len(x)) {
					return append(x, val), nil
				}
				i, err := ptr.ValidateArrayIndexForWrite(x, path[idx], idx, path)
				if err != nil {
					return nil, err
				}
				// insert at i
				return append(x[:i], append([]interface{}{val}, x[i:]...)...), nil
			case storage.ReplaceOp:
				i, err := ptr.ValidateArrayIndexForWrite(x, path[idx], idx, path)
				if err != nil {
					return nil, err
				}
				x[i] = val
				return x, nil
			case storage.RemoveOp:
				i, err := ptr.ValidateArrayIndexForWrite(x, path[idx], idx, path)
				if err != nil {
					return nil, err

				}
				return append(x[:i], x[i+1:]...), nil // i is skipped
			default:
				panic("unreachable")
			}
		case nil: // data wasn't set before
			return map[string]interface{}{path[idx]: val}, nil
		default:
			return nil, errors.NewNotFoundError(path)
		}
	}

	// Recurse on the value located at the next part of the path.
	key := path[idx]

	switch x := data.(type) {
	case map[string]interface{}:
		modified, err := patch(x[key], op, path, idx+1, val)
		if err != nil {
			return nil, err
		}
		x[key] = modified
		return x, nil
	case []interface{}:
		i, err := ptr.ValidateArrayIndexForWrite(x, path[idx], idx+1, path)
		if err != nil {
			return nil, err
		}
		modified, err := patch(x[i], op, path, idx+1, val)
		if err != nil {
			return nil, err
		}
		x[i] = modified
		return x, nil
	case nil: // data isn't there yet
		y := make(map[string]interface{}, 1)
		modified, err := patch(nil, op, path, idx+1, val)
		if err != nil {
			return nil, err
		}
		y[key] = modified
		return y, nil
	default:
		return nil, errors.NewNotFoundError(path)
	}
}
