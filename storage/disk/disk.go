// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package disk provides disk-based implementation of the storage.Store
// interface.
//
// The disk.Store implementation uses an embedded key-value store to persist
// policies and data. Policy modules are stored as raw byte strings with one
// module per key. Data is mapped to the underlying key-value store with the
// assistance of caller-supplied "partitions". Partitions allow the caller to
// control the portions of the /data namespace that are mapped to individual
// keys. Operations that span multiple keys (e.g., a read against the entirety
// of /data) are more expensive than reads that target a specific key because
// the storage layer has to reconstruct the object from individual key-value
// pairs and page all of the data into memory. By supplying partitions that
// align with lookups in the policies, callers can optimize policy evaluation.
//
// Partitions are specified as a set of storage paths (e.g., {/foo/bar} declares
// a single partition at /foo/bar). Each partition tells the store that values
// under the partition path should be mapped to individual keys. Values that
// fall outside of the partitions are stored at adjacent keys without further
// splitting. For example, given the partition set {/foo/bar}, /foo/bar/abcd and
// /foo/bar/efgh are be written to separate keys. All other values under /foo
// are not split any further (e.g., all values under /foo/baz would be written
// to a single key). Similarly, values that fall outside of partitions are
// stored under individual keys at the root (e.g., the full extent of the value
// at /qux would be stored under one key.)
//
// All keys written by the disk.Store implementation are prefixed as follows:
//
//   /<schema_version>/<partition_version>/<type>
//
// The <schema_version> value represents the version of the schema understood by
// this version of OPA. Currently this is always set to 1. The
// <partition_version> value represents the version of the partition layout
// supplied by the caller. Currently this is always set to 1. Currently, the
// disk.Store implementation only supports _additive_ changes to the
// partitioning layout, i.e., new partitions can be added as long as they do not
// overlap with existing unpartitioned data. The <type> value is either "data"
// or "policies" depending on the value being stored.
//
// The disk.Store implementation attempts to be compatible with the inmem.store
// implementation however there are some minor differences:
//
// * Writes that add partitioned values implicitly create an object hierarchy
// containing the value (e.g., `add /foo/bar/abcd` implicitly creates the
// structure `{"foo": {"bar": {"abcd": ...}}}`). This is unavoidable because of
// how nested /data values are mapped to key-value pairs.
//
// * Trigger events do not include a set of changed paths because the underlying
// key-value store does not make them available.
package disk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

// TODO(tsandall): deal w/ slashes in paths
// TODO(tsandall): support multi-level partitioning for use cases like k8s
// TODO(tsandall): add support for migrations

// Options contains parameters that configure the disk-based store.
type Options struct {
	Dir        string         // specifies directory to store data inside of
	Partitions []storage.Path // data prefixes that enable efficient layout
}

// Store provides a disk-based implementation of the storage.Store interface.
type Store struct {
	db         *badger.DB           // underlying key-value store
	xid        uint64               // next transaction id
	mu         sync.Mutex           // synchronizes trigger execution
	pm         *pathMapper          // maps logical storage paths to underlying store keys
	partitions *partitionTrie       // data structure to support path mapping
	triggers   map[*handle]struct{} // registered triggers
}

const (
	// metadataKey is a special value in the store for tracking schema versions.
	metadataKey = "metadata"

	// supportedSchemaVersion represents the version of the store supported by
	// this OPA.
	supportedSchemaVersion int64 = 1

	// basePartitionVersion represents the version of the caller-supplied data
	// layout (aka partitioning).
	basePartitionVersion int64 = 1
)

type metadata struct {
	SchemaVersion    *int64         `json:"schema_version"`    // OPA-controlled data schema version
	PartitionVersion *int64         `json:"partition_version"` // caller-supplied data layout version
	Partitions       []storage.Path `json:"partitions"`        // caller-supplied data layout
}

// New returns a new disk-based store based on the provided options.
func New(ctx context.Context, opts Options) (*Store, error) {

	partitions := make(pathSet, len(opts.Partitions))
	copy(partitions, opts.Partitions)
	partitions = partitions.Sorted()

	if !partitions.IsDisjoint() {
		return nil, &storage.Error{
			Code:    storage.InternalErr,
			Message: fmt.Sprintf("partitions are overlapped: %v", opts.Partitions),
		}
	}

	db, err := badger.Open(badger.DefaultOptions(opts.Dir).WithLogger(nil))
	if err != nil {
		return nil, wrapError(err)
	}

	store := &Store{
		db:         db,
		partitions: buildPartitionTrie(partitions),
		triggers:   map[*handle]struct{}{},
	}

	return store, db.Update(func(txn *badger.Txn) error {
		return store.init(ctx, txn, partitions)
	})
}

// Close finishes the DB connection and allows other processes to acquire it.
func (db *Store) Close(context.Context) error {
	return wrapError(db.db.Close())
}

// NewTransaction implements the storage.Store interface.
func (db *Store) NewTransaction(ctx context.Context, params ...storage.TransactionParams) (storage.Transaction, error) {
	var write bool
	var context *storage.Context

	if len(params) > 0 {
		write = params[0].Write
		context = params[0].Context
	}

	xid := atomic.AddUint64(&db.xid, uint64(1))
	underlying := db.db.NewTransaction(write)

	return newTransaction(xid, write, underlying, context, db.pm, db.partitions, db), nil
}

// Commit implements the storage.Store interface.
func (db *Store) Commit(ctx context.Context, txn storage.Transaction) error {
	underlying, err := db.underlying(txn)
	if err != nil {
		return err
	}
	if underlying.write {
		event, err := underlying.Commit(ctx)
		if err != nil {
			return err
		}
		db.mu.Lock()
		defer db.mu.Unlock()
		for h := range db.triggers {
			h.cb(ctx, txn, event)
		}
	} else {
		underlying.Abort(ctx)
	}
	return nil
}

// Abort implements the storage.Store interface.
func (db *Store) Abort(ctx context.Context, txn storage.Transaction) {
	underlying, err := db.underlying(txn)
	if err != nil {
		panic(err)
	}
	underlying.Abort(ctx)
}

// ListPolicies implements the storage.Policy interface.
func (db *Store) ListPolicies(ctx context.Context, txn storage.Transaction) ([]string, error) {
	underlying, err := db.underlying(txn)
	if err != nil {
		return nil, err
	}
	return underlying.ListPolicies(ctx)
}

// GetPolicy implements the storage.Policy interface.
func (db *Store) GetPolicy(ctx context.Context, txn storage.Transaction, id string) ([]byte, error) {
	underlying, err := db.underlying(txn)
	if err != nil {
		return nil, err
	}
	return underlying.GetPolicy(ctx, id)
}

// UpsertPolicy implements the storage.Policy interface.
func (db *Store) UpsertPolicy(ctx context.Context, txn storage.Transaction, id string, bs []byte) error {
	underlying, err := db.underlying(txn)
	if err != nil {
		return err
	}
	return underlying.UpsertPolicy(ctx, id, bs)
}

// DeletePolicy implements the storage.Policy interface.
func (db *Store) DeletePolicy(ctx context.Context, txn storage.Transaction, id string) error {
	underlying, err := db.underlying(txn)
	if err != nil {
		return err
	}
	if _, err := underlying.GetPolicy(ctx, id); err != nil {
		return err
	}
	return underlying.DeletePolicy(ctx, id)
}

// Register implements the storage.Trigger interface.
func (db *Store) Register(_ context.Context, txn storage.Transaction, config storage.TriggerConfig) (storage.TriggerHandle, error) {
	underlying, err := db.underlying(txn)
	if err != nil {
		return nil, err
	}
	if !underlying.write {
		return nil, &storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: "triggers must be registered with a write transaction",
		}
	}
	h := &handle{db: db, cb: config.OnCommit}
	db.mu.Lock()
	defer db.mu.Unlock()
	db.triggers[h] = struct{}{}
	return h, nil
}

// Read implements the storage.Store interface.
func (db *Store) Read(ctx context.Context, txn storage.Transaction, path storage.Path) (interface{}, error) {
	underlying, err := db.underlying(txn)
	if err != nil {
		return nil, err
	}
	return underlying.Read(ctx, path)
}

// Write implements the storage.Store interface.
func (db *Store) Write(ctx context.Context, txn storage.Transaction, op storage.PatchOp, path storage.Path, value interface{}) error {
	underlying, err := db.underlying(txn)
	if err != nil {
		return err
	}
	val := util.Reference(value)
	if err := util.RoundTrip(val); err != nil {
		return wrapError(err)
	}
	return underlying.Write(ctx, op, path, *val)
}

func (db *Store) underlying(txn storage.Transaction) (*transaction, error) {
	underlying, ok := txn.(*transaction)
	if !ok {
		return nil, &storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: fmt.Sprintf("unexpected transaction type %T", txn),
		}
	}
	if underlying.db != db {
		return nil, &storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: "unknown transaction",
		}
	}
	if underlying.stale {
		return nil, &storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: "stale transaction",
		}
	}
	return underlying, nil
}

type handle struct {
	db *Store
	cb func(context.Context, storage.Transaction, storage.TriggerEvent)
}

func (h *handle) Unregister(ctx context.Context, txn storage.Transaction) {
	underlying, err := h.db.underlying(txn)
	if err != nil {
		panic(err)
	}
	if !underlying.write {
		panic(&storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: "triggers must be unregistered with a write transaction",
		})
	}
	h.db.mu.Lock()
	delete(h.db.triggers, h)
	h.db.mu.Unlock()
}

func (db *Store) loadMetadata(txn *badger.Txn, m *metadata) (bool, error) {

	item, err := txn.Get([]byte(metadataKey))
	if err != nil {
		if err != badger.ErrKeyNotFound {
			return false, wrapError(err)
		}
		return false, nil
	}

	bs, err := item.ValueCopy(nil)
	if err != nil {
		return false, wrapError(err)
	}

	err = util.NewJSONDecoder(bytes.NewBuffer(bs)).Decode(m)
	if err != nil {
		return false, wrapError(err)
	}

	return true, nil
}

func (db *Store) setMetadata(txn *badger.Txn, m metadata) error {

	bs, err := json.Marshal(m)
	if err != nil {
		return wrapError(err)
	}

	return wrapError(txn.Set([]byte(metadataKey), bs))
}

func (db *Store) init(ctx context.Context, txn *badger.Txn, partitions []storage.Path) error {

	// Load existing metadata structure from the DB.
	var m metadata
	found, err := db.loadMetadata(txn, &m)
	if err != nil {
		return err
	}

	if found && *m.SchemaVersion != supportedSchemaVersion {
		return &storage.Error{
			Code:    storage.InternalErr,
			Message: fmt.Sprintf("unsupported schema version: %v (want %v)", *m.SchemaVersion, supportedSchemaVersion),
		}
	}

	// Initialize path mapper for operations on the DB.
	if found {
		db.pm = newPathMapper(*m.SchemaVersion, *m.PartitionVersion)
	} else {
		db.pm = newPathMapper(supportedSchemaVersion, basePartitionVersion)
	}

	schemaVersion := supportedSchemaVersion
	partitionVersion := basePartitionVersion

	// If metadata does not exist, finish initialization.
	if !found {
		return db.setMetadata(txn, metadata{
			SchemaVersion:    &schemaVersion,
			PartitionVersion: &partitionVersion,
			Partitions:       partitions,
		})
	}

	// Check for backwards incompatible changes to partition map.
	if err := db.validatePartitions(ctx, txn, m, partitions); err != nil {
		return err
	}

	// Assert updated metadata.
	return db.setMetadata(txn, metadata{
		SchemaVersion:    &schemaVersion,
		PartitionVersion: &partitionVersion,
		Partitions:       partitions,
	})
}

func (db *Store) validatePartitions(ctx context.Context, txn *badger.Txn, existing metadata, partitions []storage.Path) error {

	oldPathSet := pathSet(existing.Partitions)
	newPathSet := pathSet(partitions)
	removedPartitions := oldPathSet.Diff(newPathSet)
	addedPartitions := newPathSet.Diff(oldPathSet)

	if len(removedPartitions) > 0 {
		return &storage.Error{
			Code:    storage.InternalErr,
			Message: fmt.Sprintf("partitions are backwards incompatible (old: %v, new: %v, missing: %v)", oldPathSet.Sorted(), newPathSet.Sorted(), removedPartitions.Sorted())}
	}

	for _, path := range addedPartitions {
		for i := len(path); i > 0; i-- {
			key, err := db.pm.DataPath2Key(path[:i])
			if err != nil {
				return err
			}
			_, err = txn.Get(key)
			if err == nil {
				return &storage.Error{
					Code:    storage.InternalErr,
					Message: fmt.Sprintf("partitions are backwards incompatible (existing data: %v)", path[:i]),
				}
			} else if err != badger.ErrKeyNotFound {
				return wrapError(err)
			}
		}
	}

	return nil
}
