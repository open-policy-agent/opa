// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import "github.com/open-policy-agent/opa/ast"

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
}

type mount struct {
	path    ast.Ref
	strpath []string
	backend Store
}

// New returns a new instance of the policy engine's storage layer.
func New(config Config) *Storage {
	return &Storage{
		builtin:     config.Builtin,
		indices:     newIndices(),
		policyStore: newPolicyStore(config.Builtin, config.PolicyDir),
	}
}

// Open initializes the storage layer. Open should normally be called
// immediately after instantiating a new instance of the storage layer. If the
// storage layer is configured to use in-memory storage and is not persisting
// policy modules to disk, the call to Open() may be omitted.
func (s *Storage) Open() error {
	return s.policyStore.Open(loadPolicies)
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
func (s *Storage) Mount(backend Store, path ast.Ref) error {
	for _, m := range s.mounts {
		if path.HasPrefix(m.path) || m.path.HasPrefix(path) {
			return mountConflictError()
		}
	}
	spath := make([]string, len(path))
	for i, x := range path {
		switch v := x.Value.(type) {
		case ast.String:
			spath[i] = string(v)
		case ast.Var:
			spath[i] = string(v)
		default:
			return internalError("bad mount path: %v", path)
		}
	}
	m := &mount{
		path:    path,
		strpath: spath,
		backend: backend,
	}
	s.mounts = append(s.mounts, m)
	return nil
}

// Unmount removes a store from the storage layer. If the path does not locate
// an existing mount, an error is returned.
func (s *Storage) Unmount(path ast.Ref) error {
	for i := range s.mounts {
		if s.mounts[i].path.Equal(path) {
			s.mounts = append(s.mounts[:i], s.mounts[i+1:]...)
			return nil
		}
	}
	return notFoundRefError(path, "unmount")
}

type hole struct {
	path []string
	doc  interface{}
}

// Read fetches the value in storage referred to by path. The path may refer to
// multiple stores in which case the storage layer will fetch the values from
// each store and then stitch together the result.
func (s *Storage) Read(txn Transaction, path ast.Ref) (interface{}, error) {

	// TODO(tsandall): lazily call Begin() on backend if it has not been done so
	// already for this transaction.

	if !path.IsGround() {
		return nil, internalError("non-ground reference:", path)
	}

	holes := []hole{}

	for _, mount := range s.mounts {

		// Check if read is against this mount (alone)
		if path.HasPrefix(mount.path) {
			return mount.backend.Read(txn, path)
		}

		// Check if read is over this mount (and possibly others)
		if mount.path.HasPrefix(path) {
			node, err := mount.backend.Read(txn, mount.path)
			if err != nil {
				return nil, err
			}
			prefix := mount.strpath[len(path):]
			holes = append(holes, hole{prefix, node})
		}
	}

	doc, err := s.builtin.Read(txn, path)
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
func (s *Storage) Write(txn Transaction, op PatchOp, ref ast.Ref, value interface{}) error {
	return s.builtin.Write(txn, op, ref, value)
}

// NewTransaction returns a new transcation that can be used to perform reads
// against a consistent snapshot of the storage layer. The caller can provide a
// slice of references that may be read during the transaction.
func (s *Storage) NewTransaction(refs ...ast.Ref) (Transaction, error) {
	// TODO(tsandall):
	return invalidTXN, nil
}

// Close completes a transaction.
func (s *Storage) Close(txn Transaction) {
	// TODO(tsandall):
}

// BuildIndex causes the storage layer to create an index for the given
// reference over the snapshot identified by the transaction.
func (s *Storage) BuildIndex(txn Transaction, ref ast.Ref) error {

	// TODO(tsandall): for now we prevent indexing against stores other than the
	// built-in. This will be revisited in the future. To determine the
	// reference touches an external store, we collect the ground portion of
	// the reference and see if it matches any mounts.
	ground := ast.Ref{ref[0]}

	for _, x := range ref[1:] {
		if x.IsGround() {
			ground = append(ground, x)
		}
	}

	for _, mount := range s.mounts {
		if ground.HasPrefix(mount.path) {
			return indexingNotSupportedError()
		}
	}

	return s.indices.Build(s.builtin, txn, ref)
}

// IndexExists returns true if an index has been built for reference.
func (s *Storage) IndexExists(ref ast.Ref) bool {
	return s.indices.Get(ref) != nil
}

// Index invokes the iterator with bindings for each variable in the reference
// that if plugged into the reference, would locate a document with a matching
// value.
func (s *Storage) Index(txn Transaction, ref ast.Ref, value interface{}, iter func(*Bindings) error) error {

	idx := s.indices.Get(ref)
	if idx == nil {
		return indexNotFoundError()
	}

	return idx.Iter(value, iter)
}

// InsertPolicy upserts a policy module into storage inside a new transaction.
func InsertPolicy(store *Storage, id string, mod *ast.Module, raw []byte, persist bool) error {
	txn, err := store.NewTransaction()
	if err != nil {
		return err
	}
	defer store.Close(txn)
	return store.InsertPolicy(txn, id, mod, raw, persist)
}

// DeletePolicy removes a policy module from storage inside a new transaction.
func DeletePolicy(store *Storage, id string) error {
	txn, err := store.NewTransaction()
	if err != nil {
		return err
	}
	defer store.Close(txn)
	return store.DeletePolicy(txn, id)
}

// GetPolicy returns a policy module from storage inside a new transaction.
func GetPolicy(store *Storage, id string) (*ast.Module, []byte, error) {
	txn, err := store.NewTransaction()
	if err != nil {
		return nil, nil, err
	}
	defer store.Close(txn)
	return store.GetPolicy(txn, id)
}

// ReadOrDie is a helper function to read the path from storage. If the read
// fails for any reason, this function will panic. This function should only be
// used for tests.
func ReadOrDie(store *Storage, path ast.Ref) interface{} {
	txn, err := store.NewTransaction()
	if err != nil {
		panic(err)
	}
	defer store.Close(txn)
	node, err := store.Read(txn, path)
	if err != nil {
		panic(err)
	}
	return node
}

// NewTransactionOrDie is a helper function to create a new transaction. If the
// storage layer cannot create a new transaction, this function will panic. This
// function should only be used for tests.
func NewTransactionOrDie(store *Storage, refs ...ast.Ref) Transaction {
	txn, err := store.NewTransaction(refs...)
	if err != nil {
		panic(err)
	}
	return txn
}
