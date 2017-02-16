// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"sync"

	"github.com/open-policy-agent/opa/ast"
)

// Config represents the configuration for the policy engine's storage layer.
type Config struct {
	Builtin   Store
	PolicyDir string
}

// InMemoryConfig returns a new Config for an in-memory storage layer.
func InMemoryConfig() Config {
	return Config{
		Builtin:   NewDataStore(),
		PolicyDir: "",
	}
}

// InMemoryWithJSONConfig returns a new Config for an in-memory storage layer
// using existing JSON data. This is primarily for test purposes.
func InMemoryWithJSONConfig(data map[string]interface{}) Config {
	return Config{
		Builtin:   NewDataStoreFromJSONObject(data),
		PolicyDir: "",
	}
}

// WithPolicyDir returns a new Config with the policy directory configured.
func (c Config) WithPolicyDir(dir string) Config {
	c.PolicyDir = dir
	return c
}

// Storage represents the policy engine's storage layer.
type Storage struct {
	builtin     Store
	indices     *indices
	mounts      []*mount
	policyStore *policyStore

	// TODO(tsandall): currently we serialize all transactions; this means we
	// only have to keep track of a single set of stores active in the
	// transaction. In the future, we will allow concurrent transactions, in
	// which case most of this will have to be refactored.
	mtx    sync.Mutex
	active map[string]struct{}
	txn    transaction
}

type mount struct {
	path    Path
	backend Store
}

// New returns a new instance of the policy engine's storage layer.
func New(config Config) *Storage {
	return &Storage{
		builtin:     config.Builtin,
		indices:     newIndices(),
		policyStore: newPolicyStore(config.PolicyDir),
		active:      map[string]struct{}{},
	}
}

// Open initializes the storage layer. Open should normally be called
// immediately after instantiating a new instance of the storage layer. If the
// storage layer is configured to use in-memory storage and is not persisting
// policy modules to disk, the call to Open() may be omitted.
func (s *Storage) Open(ctx context.Context) error {

	txn, err := s.NewTransaction(ctx)
	if err != nil {
		return err
	}

	defer s.Close(ctx, txn)

	return s.policyStore.Open(txn, loadPolicies)
}

// ListPolicies returns a map of policy modules that have been loaded into the
// storage layer.
func (s *Storage) ListPolicies(txn Transaction) map[string]*ast.Module {
	return s.policyStore.List()
}

// GetPolicy returns the policy module with the given id. The return value
// includes the raw []byte representation of the policy if it was provided when
// inserting the policy module.
func (s *Storage) GetPolicy(txn Transaction, id string) (*ast.Module, []byte, error) {
	mod, err := s.policyStore.Get(id)
	if err != nil {
		return nil, nil, err
	}
	bs, err := s.policyStore.GetRaw(id)
	if err != nil {
		return nil, nil, err
	}
	return mod, bs, nil
}

// InsertPolicy upserts a policy module into the storage layer. If the policy
// module already exists, it is replaced. If the persist flag is true, the
// storage layer will attempt to write the raw policy module content to disk.
func (s *Storage) InsertPolicy(txn Transaction, id string, module *ast.Module, raw []byte, persist bool) error {
	return s.policyStore.Add(id, module, raw, persist)
}

// DeletePolicy removes a policy from the storage layer.
func (s *Storage) DeletePolicy(txn Transaction, id string) error {
	return s.policyStore.Remove(id)
}

// Mount adds a store into the storage layer at the given path. If the path
// conflicts with an existing mount, an error is returned.
func (s *Storage) Mount(backend Store, path Path) error {

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, m := range s.mounts {
		if path.HasPrefix(m.path) || m.path.HasPrefix(path) {
			return mountConflictError()
		}
	}

	m := &mount{
		path:    path,
		backend: backend,
	}

	s.mounts = append(s.mounts, m)
	return nil
}

// Unmount removes a store from the storage layer. If the path does not locate
// an existing mount, an error is returned.
func (s *Storage) Unmount(path Path) error {

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for i := range s.mounts {
		if s.mounts[i].path.Equal(path) {
			s.mounts = append(s.mounts[:i], s.mounts[i+1:]...)
			return nil
		}
	}

	return notFoundError(path)
}

// Read fetches the value in storage referred to by path. The path may refer to
// multiple stores in which case the storage layer will fetch the values from
// each store and then stitch together the result.
func (s *Storage) Read(ctx context.Context, txn Transaction, path Path) (interface{}, error) {

	type hole struct {
		path []string
		doc  interface{}
	}

	holes := []hole{}

	for _, mount := range s.mounts {

		// Check if read is against this mount (alone)
		if path.HasPrefix(mount.path) {
			if err := s.lazyActivate(ctx, mount.backend, txn, nil); err != nil {
				return nil, err
			}
			return mount.backend.Read(ctx, txn, path[len(mount.path):])
		}

		// Check if read is over this mount (and possibly others)
		if mount.path.HasPrefix(path) {
			if err := s.lazyActivate(ctx, mount.backend, txn, nil); err != nil {
				return nil, err
			}
			node, err := mount.backend.Read(ctx, txn, Path{})
			if err != nil {
				return nil, err
			}
			prefix := mount.path[len(path):]
			holes = append(holes, hole{prefix, node})
		}
	}

	if err := s.lazyActivate(ctx, s.builtin, txn, nil); err != nil {
		return nil, err
	}

	doc, err := s.builtin.Read(ctx, txn, path)
	if err != nil {
		return nil, err
	}

	// Fill holes in built-in document with any documents obtained from mounted
	// stores. The mounts imply a hierarchy of objects, so traverse each mount
	// path and create that hierarchy as necessary.
	for _, hole := range holes {

		p := hole.path
		curr := doc.(map[string]interface{})

		for _, s := range p[:len(p)-1] {
			next, ok := curr[s]
			if !ok {
				next = map[string]interface{}{}
				curr[s] = next
			}
			curr = next.(map[string]interface{})
		}

		curr[p[len(p)-1]] = hole.doc
	}

	return doc, nil
}

// Write updates a value in storage.
func (s *Storage) Write(ctx context.Context, txn Transaction, op PatchOp, path Path, value interface{}) error {

	if err := s.lazyActivate(ctx, s.builtin, txn, nil); err != nil {
		return err
	}

	return s.builtin.Write(ctx, txn, op, path, value)
}

// NewTransaction returns a new Transaction with default parameters.
func (s *Storage) NewTransaction(ctx context.Context) (Transaction, error) {
	return s.NewTransactionWithParams(ctx, TransactionParams{})
}

// NewTransactionWithParams returns a new Transaction.
func (s *Storage) NewTransactionWithParams(ctx context.Context, params TransactionParams) (Transaction, error) {

	s.mtx.Lock()
	s.txn++
	txn := s.txn

	if err := s.notifyStoresBegin(ctx, txn, params.Paths); err != nil {
		return nil, err
	}

	return txn, nil
}

// Close completes a transaction.
func (s *Storage) Close(ctx context.Context, txn Transaction) {
	s.notifyStoresClose(ctx, txn)
	s.mtx.Unlock()
}

// BuildIndex causes the storage layer to create an index for the given
// reference over the snapshot identified by the transaction.
func (s *Storage) BuildIndex(ctx context.Context, txn Transaction, ref ast.Ref) error {

	path, err := NewPathForRef(ref.GroundPrefix())
	if err != nil {
		return indexingNotSupportedError()
	}

	// TODO(tsandall): for now we prevent indexing against stores other than the
	// built-in. This will be revisited in the future. To determine the
	// reference touches an external store, we collect the ground portion of
	// the reference and see if it matches any mounts.
	for _, mount := range s.mounts {
		if path.HasPrefix(mount.path) || mount.path.HasPrefix(path) {
			return indexingNotSupportedError()
		}
	}

	return s.indices.Build(ctx, s.builtin, txn, ref)
}

// IndexExists returns true if an index has been built for reference.
func (s *Storage) IndexExists(ref ast.Ref) bool {
	return s.indices.Get(ref) != nil
}

// Index invokes the iterator with bindings for each variable in the reference
// that if plugged into the reference, would locate a document with a matching
// value.
func (s *Storage) Index(txn Transaction, ref ast.Ref, value interface{}, iter func(*ast.ValueMap) error) error {

	idx := s.indices.Get(ref)
	if idx == nil {
		return indexNotFoundError()
	}

	return idx.Iter(value, iter)
}

func (s *Storage) getStoreByID(id string) Store {
	if id == s.builtin.ID() {
		return s.builtin
	}
	for _, mount := range s.mounts {
		if mount.backend.ID() == id {
			return mount.backend
		}
	}
	return nil
}

func (s *Storage) lazyActivate(ctx context.Context, store Store, txn Transaction, paths []Path) error {

	id := store.ID()
	if _, ok := s.active[id]; ok {
		return nil
	}

	params := TransactionParams{}
	if err := store.Begin(ctx, txn, params); err != nil {
		return err
	}

	s.active[id] = struct{}{}
	return nil
}

func (s *Storage) notifyStoresBegin(ctx context.Context, txn Transaction, paths []Path) error {

	builtinID := s.builtin.ID()

	// Initialize the active set. After a store has been notified that a
	// transaction has started, it is added to this set. When a transaction is
	// closed, the set is consulted to determine which stores to notify.
	s.active = map[string]struct{}{}

	mounts := map[string]Path{}
	for _, mount := range s.mounts {
		mounts[mount.backend.ID()] = mount.path
	}

	grouped := groupPathsByStore(builtinID, mounts, paths)

	for id, groupedPaths := range grouped {
		params := TransactionParams{
			Paths: groupedPaths,
		}
		if err := s.getStoreByID(id).Begin(ctx, txn, params); err != nil {
			return err
		}
		s.active[id] = struct{}{}
	}

	return nil
}

func (s *Storage) notifyStoresClose(ctx context.Context, txn Transaction) {
	for id := range s.active {
		s.getStoreByID(id).Close(ctx, txn)
	}
	s.active = nil
}

// InsertPolicy upserts a policy module into storage inside a new transaction.
func InsertPolicy(ctx context.Context, store *Storage, id string, mod *ast.Module, raw []byte, persist bool) error {
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx, txn)
	return store.InsertPolicy(txn, id, mod, raw, persist)
}

// DeletePolicy removes a policy module from storage inside a new transaction.
func DeletePolicy(ctx context.Context, store *Storage, id string) error {
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		return err
	}
	defer store.Close(ctx, txn)
	return store.DeletePolicy(txn, id)
}

// GetPolicy returns a policy module from storage inside a new transaction.
func GetPolicy(ctx context.Context, store *Storage, id string) (*ast.Module, []byte, error) {
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer store.Close(ctx, txn)
	return store.GetPolicy(txn, id)
}

// NewTransactionOrDie is a helper function to create a new transaction. If the
// storage layer cannot create a new transaction, this function will panic. This
// function should only be used for tests.
func NewTransactionOrDie(ctx context.Context, store *Storage) Transaction {
	txn, err := store.NewTransaction(ctx)
	if err != nil {
		panic(err)
	}
	return txn
}

func groupPathsByStore(builtinID string, mounts map[string]Path, paths []Path) map[string][]Path {

	r := map[string][]Path{}

	for _, path := range paths {
		sole := false
		for id, mountPath := range mounts {
			if path.HasPrefix(mountPath) {
				r[id] = append(r[id], path[len(mountPath):])
				sole = true
				break
			}
			if mountPath.HasPrefix(path) {
				r[id] = append(r[id], Path{})
			}
		}
		if !sole {
			// Read may span multiple stores, so by definition, built-in store
			// will be read.
			r[builtinID] = append(r[builtinID], path)
		}
	}

	return r
}
