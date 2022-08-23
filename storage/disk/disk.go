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
// There is support for wildcards in partitions: {/foo/*} will cause /foo/bar/abc
// and /foo/buz/def to be written to separate keys. Multiple wildcards are
// supported (/tenants/*/users/*/bindings), and they can also appear at the end
// of a partition (/users/*).
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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	badger "github.com/dgraph-io/badger/v3"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

// TODO(tsandall): add support for migrations
// TODO(sr): validate partition patterns properly: if the partitions were {/foo/bar}
//           before and the new ones are {/foo/*}, it should be OK.

// a value log file be rewritten if half the space can be discarded
const valueLogGCDiscardRatio = 0.5

// Options contains parameters that configure the disk-based store.
type Options struct {
	Dir        string         // specifies directory to store data inside of
	Partitions []storage.Path // data prefixes that enable efficient layout
	Badger     string         // badger-internal configurables
}

// Store provides a disk-based implementation of the storage.Store interface.
type Store struct {
	db         *badger.DB           // underlying key-value store
	xid        uint64               // next transaction id
	rmu        sync.RWMutex         // reader-writer lock
	wmu        sync.Mutex           // writer lock
	pm         *pathMapper          // maps logical storage paths to underlying store keys
	partitions *partitionTrie       // data structure to support path mapping
	triggers   map[*handle]struct{} // registered triggers
	gcTicker   *time.Ticker         // gc ticker
	close      chan struct{}        // close-only channel for stopping the GC goroutine
	backupDB   *badger.DB           // backup of the underlying key-value store
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

	// symlink to directory path where badger write its files to
	symlinkKey = "active"
)

type metadata struct {
	SchemaVersion    *int64         `json:"schema_version"`    // OPA-controlled data schema version
	PartitionVersion *int64         `json:"partition_version"` // caller-supplied data layout version
	Partitions       []storage.Path `json:"partitions"`        // caller-supplied data layout
}

// systemPartition is the partition we add automatically: no user-defined partition
// should apply to the /system path.
const systemPartition = "/system/*"

// New returns a new disk-based store based on the provided options.
func New(ctx context.Context, logger logging.Logger, prom prometheus.Registerer, opts Options) (*Store, error) {

	partitions := make(pathSet, len(opts.Partitions))
	copy(partitions, opts.Partitions)
	partitions = partitions.Sorted()

	if !partitions.IsDisjoint() {
		return nil, &storage.Error{
			Code:    storage.InternalErr,
			Message: fmt.Sprintf("partitions are overlapped: %v", opts.Partitions),
		}
	}

	partitions = append(partitions, storage.MustParsePath(systemPartition))
	if !partitions.IsDisjoint() {
		return nil, &storage.Error{
			Code:    storage.InternalErr,
			Message: fmt.Sprintf("system partitions are managed: %v", opts.Partitions),
		}
	}

	options, err := badgerConfigFromOptions(opts)
	if err != nil {
		return nil, wrapError(err)
	}

	options = options.WithLogger(&wrap{logger})
	db, err := badger.Open(options)
	if err != nil {
		return nil, wrapError(err)
	}

	if prom != nil {
		if err := initPrometheus(prom); err != nil {
			return nil, err
		}
	}

	store := &Store{
		db:         db,
		partitions: buildPartitionTrie(partitions),
		triggers:   map[*handle]struct{}{},
		close:      make(chan struct{}),
		gcTicker:   time.NewTicker(time.Minute),
	}

	go store.GC(logger)

	if err := db.Update(func(txn *badger.Txn) error {
		return store.init(ctx, txn, partitions)
	}); err != nil {
		store.Close(ctx)
		return nil, err
	}

	return store, store.diagnostics(ctx, partitions, logger)
}

func (db *Store) GC(logger logging.Logger) {
	for {
		select {
		case <-db.close:
			return
		case <-db.gcTicker.C:
			for err := error(nil); err == nil; err = db.db.RunValueLogGC(valueLogGCDiscardRatio) {
				logger.Debug("RunValueLogGC: err=%v", err)
			}
		}
	}
}

// Close finishes the DB connection and allows other processes to acquire it.
func (db *Store) Close(context.Context) error {
	db.gcTicker.Stop()
	return wrapError(db.db.Close())
}

// If the log level is debug, we'll output the badger logs in their corresponding
// log levels; if it's not debug, we'll suppress all badger logs.
type wrap struct {
	l logging.Logger
}

func (w *wrap) debugDo(f func(string, ...interface{}), fmt string, as ...interface{}) {
	if w.l.GetLevel() >= logging.Debug {
		f("badger: "+fmt, as...)
	}
}

func (w *wrap) Debugf(f string, as ...interface{})   { w.debugDo(w.l.Debug, f, as...) }
func (w *wrap) Infof(f string, as ...interface{})    { w.debugDo(w.l.Info, f, as...) }
func (w *wrap) Warningf(f string, as ...interface{}) { w.debugDo(w.l.Warn, f, as...) }
func (w *wrap) Errorf(f string, as ...interface{})   { w.debugDo(w.l.Error, f, as...) }

// NewTransaction implements the storage.Store interface.
func (db *Store) NewTransaction(ctx context.Context, params ...storage.TransactionParams) (storage.Transaction, error) {
	var write bool
	var context *storage.Context

	if len(params) > 0 {
		write = params[0].Write
		context = params[0].Context
	}

	xid := atomic.AddUint64(&db.xid, uint64(1))
	if write {
		db.wmu.Lock() // only one concurrent write txn
	} else {
		db.rmu.RLock()
	}
	underlying := db.db.NewTransaction(write)

	return newTransaction(xid, write, underlying, context, db.pm, db.partitions, db), nil
}

// Truncate implements the storage.Store interface. This method must be called within a transaction.
func (db *Store) Truncate(ctx context.Context, txn storage.Transaction, params storage.TransactionParams, it storage.Iterator) error {

	// backup the existing store
	currentDB, err := db.backupAndLoadDB()
	if err != nil {
		return wrapError(err)
	}

	db.backupDB = currentDB

	// commit in-flight txn on the existing store
	uTxn, err := db.underlying(txn)
	if err != nil {
		return err
	}

	_, err = uTxn.Commit(ctx)
	if err != nil {
		return wrapError(err)
	}

	// write new bundle policy and data into the existing DB
	underlying := db.db.NewTransaction(true)
	xid := atomic.AddUint64(&db.xid, uint64(1))
	underlyingTxn := newTransaction(xid, true, underlying, params.Context, db.pm, db.partitions, db)

	// For backwards compatibility, check if `RootOverwrite` was configured.
	if params.RootOverwrite || overwriteRoot(params.BasePaths) {
		newPath, ok := storage.ParsePathEscaped("/")
		if !ok {
			return fmt.Errorf("storage path invalid: %v", newPath)
		}

		sTxn, err := db.doTruncateData(ctx, underlyingTxn, db.db, params, newPath, map[string]interface{}{})
		if err != nil {
			return wrapError(err)
		}

		if sTxn != nil {
			underlyingTxn = sTxn
		}
	}

	for {
		var update *storage.Update

		update, err = it.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return wrapError(err)
		}

		if update.IsPolicy {
			err = underlyingTxn.UpsertPolicy(ctx, strings.TrimLeft(update.Path.String(), "/"), update.Value)
			if err != nil {
				if err != badger.ErrTxnTooBig {
					return wrapError(err)
				}

				_, err = underlyingTxn.Commit(ctx)
				if err != nil {
					return wrapError(err)
				}

				underlying = db.db.NewTransaction(true)
				xid = atomic.AddUint64(&db.xid, uint64(1))
				underlyingTxn = newTransaction(xid, true, underlying, params.Context, db.pm, db.partitions, db)

				if err = underlyingTxn.UpsertPolicy(ctx, strings.TrimLeft(update.Path.String(), "/"), update.Value); err != nil {
					return wrapError(err)
				}
			}
		} else {
			if len(update.Path) > 0 {
				sTxn, err := db.doTruncateData(ctx, underlyingTxn, db.db, params, update.Path, update.Value)
				if err != nil {
					return wrapError(err)
				}

				if sTxn != nil {
					underlyingTxn = sTxn
				}
			} else {
				for _, root := range params.BasePaths {
					newPath, ok := storage.ParsePathEscaped("/" + root)
					if !ok {
						return fmt.Errorf("storage path invalid: %v", newPath)
					}

					value, ok, err := lookup(newPath, update.Value)
					if err != nil {
						return err
					}

					if ok {
						if len(newPath) > 0 {
							if err := storage.MakeDir(ctx, db, underlyingTxn, newPath[:len(newPath)-1]); err != nil {
								return err
							}
						}

						sTxn, err := db.doTruncateData(ctx, underlyingTxn, db.db, params, newPath, value)
						if err != nil {
							return wrapError(err)
						}

						if sTxn != nil {
							underlyingTxn = sTxn
						}
					}
				}
			}
		}
	}

	if err != nil && err != io.EOF {
		return wrapError(err)
	}

	// commit active transaction on existing store
	_, err = underlyingTxn.Commit(ctx)
	if err != nil {
		return wrapError(err)
	}

	// Open write txn on the existing store in-case there are more write operations.
	// The caller will either commit or abort this transaction
	uTxn.stale = false
	uTxn.underlying = db.db.NewTransaction(true)

	return nil
}

func (db *Store) doTruncateData(ctx context.Context, underlying *transaction, badgerdb *badger.DB,
	params storage.TransactionParams, path storage.Path, value interface{}) (*transaction, error) {

	err := underlying.Write(ctx, storage.AddOp, path, value)
	if err != nil {
		if err != badger.ErrTxnTooBig {
			return nil, wrapError(err)
		}

		_, err = underlying.Commit(ctx)
		if err != nil {
			return nil, wrapError(err)
		}

		txn := badgerdb.NewTransaction(true)
		xid := atomic.AddUint64(&db.xid, uint64(1))
		sTxn := newTransaction(xid, true, txn, params.Context, db.pm, db.partitions, db)

		if err = sTxn.Write(ctx, storage.AddOp, path, value); err != nil {
			return nil, wrapError(err)
		}

		return sTxn, nil
	}

	return nil, nil
}

func (db *Store) backupAndLoadDB() (*badger.DB, error) {
	currDir := db.db.Opts().Dir

	// backup db
	backupDir, err := ioutil.TempDir(path.Dir(currDir), "backup")
	if err != nil {
		return nil, wrapError(err)
	}

	bak, err := ioutil.TempFile(backupDir, "badgerbak")
	if err != nil {
		return nil, wrapError(err)
	}

	_, err = db.db.Backup(bak, 0)
	if err != nil {
		return nil, wrapError(err)
	}

	// restore db
	newDBDir, err := ioutil.TempDir(path.Dir(currDir), "backup")
	if err != nil {
		return nil, wrapError(err)
	}

	opts := db.db.Opts().WithDir(newDBDir).WithValueDir(newDBDir)

	// open new db
	newDB, err := badger.Open(opts)
	if err != nil {
		return nil, wrapError(err)
	}

	bak, err = os.Open(bak.Name())
	if err != nil {
		return nil, err
	}
	defer bak.Close()

	err = newDB.Load(bak, 16)
	if err != nil {
		return nil, wrapError(err)
	}

	return newDB, wrapError(os.RemoveAll(backupDir))
}

func (db *Store) cleanup(oldDB *badger.DB) error {
	err := oldDB.Close()
	if err != nil {
		return wrapError(err)
	}

	return wrapError(os.RemoveAll(oldDB.Opts().Dir))
}

// Commit implements the storage.Store interface.
func (db *Store) Commit(ctx context.Context, txn storage.Transaction) error {
	underlying, err := db.underlying(txn)
	if err != nil {
		return err
	}
	if underlying.write {
		db.rmu.Lock() // blocks until all readers are done
		event, err := underlying.Commit(ctx)
		if err != nil {
			return err
		}
		write := false // read only txn
		readOnly := db.db.NewTransaction(write)
		xid := atomic.AddUint64(&db.xid, uint64(1))
		readTxn := newTransaction(xid, write, readOnly, nil, db.pm, db.partitions, db)
		for h := range db.triggers {
			h.cb(ctx, readTxn, event)
		}

		// cleanup backup db
		if db.backupDB != nil {
			if err := db.cleanup(db.backupDB); err != nil {
				panic(err)
			}
			db.backupDB = nil
		}

		db.rmu.Unlock()
		db.wmu.Unlock()
	} else { // committing read txn
		underlying.Abort(ctx)
		db.rmu.RUnlock()
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

	if underlying.write {

		if db.backupDB != nil {
			db.rmu.Lock()

			// update symlink to point to the backup db
			symlink := filepath.Join(path.Dir(db.backupDB.Opts().Dir), symlinkKey)
			// "active" -> "backupXXXX" is what we want, not
			// "active" -> "DIR/backupXXX", since that won't work when using a relative directory
			target := filepath.Base(db.backupDB.Opts().Dir)

			err = createSymlink(target, symlink)
			if err != nil {
				panic(err)
			}

			// swap db
			oldDb := db.db
			db.db = db.backupDB

			// cleanup existing db
			if err := db.cleanup(oldDb); err != nil {
				panic(err)
			}

			db.rmu.Unlock()
		}

		db.wmu.Unlock()
	} else {
		db.rmu.RUnlock()
	}
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
	delete(h.db.triggers, h)
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

	// It's OK to replace partitions with wildcard partitions that overlap them:
	// REMOVED: /foo/bar
	// ADDED:   /foo/*
	// and the like.
	replaced := make(pathSet, 0)
	replacements := make(pathSet, 0)
	for _, removed := range removedPartitions {
		for _, added := range addedPartitions {
			if isMatchedBy(removed, added) {
				replaced = append(replaced, removed)
				replacements = append(replacements, added)
			}
		}
	}

	rest := removedPartitions.Diff(replaced)
	if len(rest) > 0 {
		return &storage.Error{
			Code:    storage.InternalErr,
			Message: fmt.Sprintf("partitions are backwards incompatible (old: %v, new: %v, missing: %v)", oldPathSet, newPathSet, rest)}
	}

	for _, path := range addedPartitions.Diff(replacements) {
		if prefix, wildcard := hasWildcard(path); wildcard {
			path = prefix
		}
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

// MakeDir makes Store a storage.MakeDirer, to avoid the superfluous MakeDir
// steps -- MakeDir is implicit in the disk storage's data layout, since
//     {"foo": {"bar": {"baz": 10}}}
// writes value `10` to key `/foo/bar/baz`.
//
// Here, we only check if it's a write transaction, for consistency with
// other implementations, and do nothing.
func (db *Store) MakeDir(_ context.Context, txn storage.Transaction, path storage.Path) error {
	underlying, err := db.underlying(txn)
	if err != nil {
		return err
	}
	if !underlying.write {
		return &storage.Error{
			Code:    storage.InvalidTransactionErr,
			Message: "MakeDir must be called with a write transaction",
		}
	}
	return nil
}

// diagnostics prints relevant partition and database related information at
// debug level.
func (db *Store) diagnostics(ctx context.Context, partitions pathSet, logger logging.Logger) error {
	if logger.GetLevel() < logging.Debug {
		return nil
	}
	if len(partitions) == 1 { // '/system/*' is always present
		logger.Warn("no partitions configured")
		if err := db.logPrefixStatistics(ctx, storage.MustParsePath("/"), logger); err != nil {
			return err
		}
	}
	for _, partition := range partitions {
		if err := db.logPrefixStatistics(ctx, partition, logger); err != nil {
			return err
		}
	}
	return nil
}

func (db *Store) logPrefixStatistics(ctx context.Context, partition storage.Path, logger logging.Logger) error {

	if prefix, ok := hasWildcard(partition); ok {
		return db.logPrefixStatisticsWildcardPartition(ctx, prefix, partition, logger)
	}

	key, err := db.pm.DataPrefix2Key(partition)
	if err != nil {
		return err
	}

	opt := badger.DefaultIteratorOptions
	opt.PrefetchValues = false
	opt.Prefix = key

	var count, size uint64
	if err := db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(opt)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			if err := ctx.Err(); err != nil {
				return err
			}
			count++
			size += uint64(it.Item().EstimatedSize()) // key length + value length
		}
		return nil
	}); err != nil {
		return err
	}
	logger.Debug("partition %s: key count: %d (estimated size %d bytes)", partition, count, size)
	return nil
}

func hasWildcard(path storage.Path) (storage.Path, bool) {
	for i := range path {
		if path[i] == pathWildcard {
			return path[:i], true
		}
	}
	return nil, false
}

func (db *Store) logPrefixStatisticsWildcardPartition(ctx context.Context, prefix, partition storage.Path, logger logging.Logger) error {
	// we iterate all keys, and count things according to their concrete partition
	type diagInfo struct{ count, size uint64 }
	diag := map[string]*diagInfo{}

	key, err := db.pm.DataPrefix2Key(prefix)
	if err != nil {
		return err
	}

	opt := badger.DefaultIteratorOptions
	opt.PrefetchValues = false
	opt.Prefix = key
	if err := db.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(opt)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			if err := ctx.Err(); err != nil {
				return err
			}
			if part, ok := db.prefixInPattern(it.Item().Key(), partition); ok {
				p := part.String()
				if diag[p] == nil {
					diag[p] = &diagInfo{}
				}
				diag[p].count++
				diag[p].size += uint64(it.Item().EstimatedSize()) // key length + value length
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if len(diag) == 0 {
		logger.Debug("partition pattern %s: key count: 0 (estimated size 0 bytes)", toString(partition))
	}
	for part, diag := range diag {
		logger.Debug("partition %s (pattern %s): key count: %d (estimated size %d bytes)", part, toString(partition), diag.count, diag.size)
	}
	return nil
}

func (db *Store) prefixInPattern(key []byte, partition storage.Path) (storage.Path, bool) {
	var part storage.Path
	path, err := db.pm.DataKey2Path(key)
	if err != nil {
		return nil, false
	}
	for i := range partition {
		if path[i] != partition[i] && partition[i] != pathWildcard {
			return nil, false
		}
		part = append(part, path[i])
	}
	return part, true
}

func toString(path storage.Path) string {
	if len(path) == 0 {
		return "/"
	}
	buf := strings.Builder{}
	for _, p := range path {
		fmt.Fprintf(&buf, "/%s", p)
	}
	return buf.String()
}

// dataDir prefixes the configured storage location: what it returns is
// what we have badger write its files to. It is done to give us some
// wiggle room in the future should we need to put further files on the
// file system (like backups): we can then just use the opts.Dir.
func dataDir(dir string) (string, error) {

	symlink := filepath.Join(dir, symlinkKey)
	if _, err := os.Lstat(symlink); err == nil {
		return filepath.EvalSymlinks(symlink)
	}

	return filepath.Join(dir, "data"), nil
}

func createSymlink(target, symlink string) error {
	var lerr error

	if _, lerr = os.Lstat(symlink); lerr == nil {
		if err := os.Remove(symlink); err != nil {
			return err
		}

		if err := os.Symlink(target, symlink); err != nil {
			return err
		}
	} else if errors.Is(lerr, os.ErrNotExist) {
		if err := os.Symlink(target, symlink); err != nil {
			return err
		}

		return nil
	}

	return lerr
}

func lookup(path storage.Path, data []byte) (interface{}, bool, error) {
	var obj map[string]json.RawMessage
	err := util.Unmarshal(data, &obj)
	if err != nil {
		return nil, false, err
	}

	if len(path) == 0 {
		return obj, true, nil
	}

	for i := 0; i < len(path)-1; i++ {
		value, ok := obj[path[i]]
		if !ok {
			return nil, false, nil
		}

		var next map[string]json.RawMessage
		err := util.Unmarshal(value, &next)
		if err != nil {
			return nil, false, err
		}

		obj = next
	}

	value, ok := obj[path[len(path)-1]]
	return value, ok, nil
}

func overwriteRoot(roots []string) bool {
	for _, root := range roots {
		if root == "" {
			return true
		}
	}
	return false
}
