// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package materialized provides materialized view support for OPA.
// It allows policies under the system.store namespace to be evaluated
// when data is updated, with results stored back in the data document
// for efficient access during policy evaluation.
package materialized

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// contextKey is used to prevent recursive evaluation of system.store policies
type contextKey int

const evaluatingSystemStoreKey contextKey = 0

// Sentinel errors for package-level error types
var (
	// ErrManagerNotInitialized is returned when Manager is used without proper initialization
	ErrManagerNotInitialized = errors.New("manager not properly initialized")

	// ErrInvalidModuleOrRule is returned when module or rule parameters are invalid
	ErrInvalidModuleOrRule = errors.New("invalid module or rule")

	// ErrRuleEmptyName is returned when a rule has an empty name
	ErrRuleEmptyName = errors.New("rule has empty name")

	// ErrModuleNoPackage is returned when a module has no package declaration
	ErrModuleNoPackage = errors.New("module has no package")
)

// Object pools for frequently allocated types to reduce GC pressure
var (
	// refPool pools ast.Ref slices for query path construction
	// Used for temporary Refs that are NOT returned (no copy overhead)
	// Pre-allocate with reasonable capacity (typical path length)
	refPool = sync.Pool{
		New: func() any {
			ref := make(ast.Ref, 0, 8)
			return &ref
		},
	}

	// Interned constant strings to avoid repeated allocations
	// These are used frequently in path comparisons and construction
	internedStoreString = ast.String("store")

	// Pre-allocated empty object for parent path creation
	// Reused across all evaluations to avoid map allocations
	sharedEmptyMap = map[string]any{}

	// Pre-allocated constant Var for query construction
	// Reused across all rule evaluations
	itemVar = ast.Var("__item")
)

// getRefFromPool retrieves a Ref slice from the pool
func getRefFromPool() *ast.Ref {
	ref := refPool.Get().(*ast.Ref)
	*ref = (*ref)[:0] // Reset length while keeping capacity
	return ref
}

// putRefToPool returns a Ref slice to the pool
func putRefToPool(ref *ast.Ref) {
	if ref != nil && cap(*ref) <= 32 { // Only pool reasonably-sized slices
		refPool.Put(ref)
	}
}

// Manager handles evaluation of system.store policies to create materialized views
type Manager struct {
	compiler *ast.Compiler
	store    storage.Store
}

// NewManager creates a new materialized view manager
func NewManager(compiler *ast.Compiler, store storage.Store) *Manager {
	return &Manager{
		compiler: compiler,
		store:    store,
	}
}

// EvaluateSystemStore evaluates all policies under the system.store namespace
// and writes their results back to the store for use as materialized views.
//
// This should be called during bundle activation after data has been written
// but before the transaction is committed.
func (m *Manager) EvaluateSystemStore(ctx context.Context, txn storage.Transaction) error {
	if m.compiler == nil || m.store == nil {
		return ErrManagerNotInitialized
	}

	// Prevent recursive evaluation
	if isEvaluatingSystemStore(ctx) {
		return nil
	}
	ctx = markEvaluatingSystemStore(ctx)

	// Find all modules in the system.store namespace
	policies := m.findSystemStorePolicies()
	if len(policies) == 0 {
		return nil
	}

	// Evaluate each policy module
	for _, module := range policies {
		if module == nil {
			continue
		}

		// Evaluate each rule in the module
		for _, rule := range module.Rules {
			if rule == nil || rule.Head == nil {
				continue
			}

			if err := m.evaluateRule(ctx, txn, module, rule); err != nil {
				return fmt.Errorf("failed to evaluate system.store rule %s.%s: %w", module.Package.Path, rule.Head.Name, err)
			}
		}
	}

	return nil
}

// findSystemStorePolicies returns all modules under the system.store namespace
func (m *Manager) findSystemStorePolicies() []*ast.Module {
	// First pass: count system.store modules for exact allocation
	count := 0
	for _, module := range m.compiler.Modules {
		if isSystemStoreModule(module) {
			count++
		}
	}

	if count == 0 {
		return nil
	}

	// Second pass: allocate exactly and populate
	policies := make([]*ast.Module, 0, count)
	for _, module := range m.compiler.Modules {
		if isSystemStoreModule(module) {
			policies = append(policies, module)
		}
	}

	return policies
}

// isSystemStoreModule checks if a module is under the system.store namespace
func isSystemStoreModule(module *ast.Module) bool {
	if module == nil || module.Package == nil {
		return false
	}

	path := module.Package.Path

	// Fast path: minimum length check
	pathLen := len(path)
	if pathLen < 2 {
		return false
	}

	// Check if starts with "data" and skip it
	startIdx := 0
	if pathLen > 0 {
		if firstVal := path[0].Value; firstVal.Compare(ast.DefaultRootDocument.Value) == 0 {
			startIdx = 1
			// After skipping "data", need at least 2 more components
			if pathLen < 3 {
				return false
			}
		}
	}

	// Check for system.store prefix (avoid multiple Compare calls)
	// Use direct comparison when possible
	if startIdx+1 >= pathLen {
		return false
	}

	systemVal := path[startIdx].Value
	storeVal := path[startIdx+1].Value

	// Use interned strings to avoid allocations
	return systemVal.Compare(ast.SystemDocumentKey) == 0 &&
		storeVal.Compare(internedStoreString) == 0
}

// evaluateRule evaluates a single rule in a system.store module and writes results to the store
func (m *Manager) evaluateRule(ctx context.Context, txn storage.Transaction, module *ast.Module, rule *ast.Rule) error {
	if module == nil || module.Package == nil || rule == nil || rule.Head == nil {
		return ErrInvalidModuleOrRule
	}

	if rule.Head.Name == "" {
		return ErrRuleEmptyName
	}

	// Derive the storage path from the module package and rule name
	// e.g., package system.store + rule developers -> /system/store/developers
	storePath, err := deriveStoragePathForRule(module, rule)
	if err != nil {
		return err
	}

	// Build the query path - module.Package.Path already starts with "data"
	// so we append the rule name to it
	// Use pooled Ref to avoid allocation
	queryPathPtr := getRefFromPool()
	defer putRefToPool(queryPathPtr)

	// Build queryPath from pooled slice
	*queryPathPtr = append(*queryPathPtr, module.Package.Path...)
	// Convert rule name to term - must use String type for query paths
	*queryPathPtr = append(*queryPathPtr, ast.StringTerm(rule.Head.Name.String()))
	queryPath := *queryPathPtr

	// Use pre-allocated itemVar constant (defined at package level)

	// Create query to enumerate all items in the rule result
	// For sets, we query data.system.store.rulename[__item]
	// For complete rules, we query __item = data.system.store.rulename
	var query ast.Body

	if rule.Head.Key != nil {
		// Partial set/object rule: query data.system.store.rulename[__item]
		// Use pooled Ref for refWithVar
		refWithVarPtr := getRefFromPool()
		defer putRefToPool(refWithVarPtr)

		*refWithVarPtr = append(*refWithVarPtr, queryPath...)
		*refWithVarPtr = append(*refWithVarPtr, ast.NewTerm(itemVar))
		query = ast.NewBody(ast.NewExpr(ast.NewTerm(*refWithVarPtr)))
	} else {
		// Complete rule: query __item = data.system.store.rulename
		query = ast.NewBody(
			ast.Equality.Expr(ast.NewTerm(itemVar), ast.NewTerm(queryPath)),
		)
	}

	// Create and configure query
	q := topdown.NewQuery(query).
		WithCompiler(m.compiler).
		WithStore(m.store).
		WithTransaction(txn)

	// Execute query and collect all results
	rs, err := q.Run(ctx)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}

	// Extract results
	if len(rs) == 0 {
		// No results means undefined, we can skip writing
		return nil
	}

	// Convert results to native Go types for storage
	var storageValue any

	if rule.Head.Key != nil {
		// Partial rule: Build native slice directly, bypassing ast.Set
		// This avoids: Set hash map allocation + Set iteration during JSON conversion
		// Pre-allocate with exact capacity to avoid growth
		nativeSlice := make([]any, 0, len(rs))
		for i := range rs {
			item, ok := rs[i][itemVar]
			if !ok || item == nil {
				continue
			}

			// Convert each item directly to native type
			// Using JSONWithOpt with optimized flags
			itemNative, err := ast.JSONWithOpt(item.Value, ast.JSONOpt{
				SortSets: false, // Don't sort - faster
				CopyMaps: false, // For lazyObj, return native directly (zero-copy)
			})
			if err != nil {
				return fmt.Errorf("failed to convert item to native type: %w", err)
			}
			nativeSlice = append(nativeSlice, itemNative)
		}
		storageValue = nativeSlice
	} else {
		// Complete rule: convert single value
		item, ok := rs[0][itemVar]
		if !ok || item == nil {
			return nil
		}

		// Use JSONWithOpt with optimized flags
		nativeValue, err := ast.JSONWithOpt(item.Value, ast.JSONOpt{
			SortSets: false, // Don't sort - faster
			CopyMaps: false, // For lazyObj, return native directly (zero-copy)
		})
		if err != nil {
			return fmt.Errorf("failed to convert value to native type: %w", err)
		}
		storageValue = nativeValue
	}

	// Ensure parent paths exist
	// For path /system/store/developers, we need to ensure /system and /system/store exist
	// Optimize: reuse shared empty map to avoid allocations
	for i := range storePath {
		parentPath := storePath[:i]
		if _, err := m.store.Read(ctx, txn, parentPath); err != nil {
			// Parent doesn't exist, create it as an empty object
			// Use shared empty map - safe because storage makes a copy
			if err := m.store.Write(ctx, txn, storage.AddOp, parentPath, sharedEmptyMap); err != nil {
				return fmt.Errorf("failed to create parent path %v: %w", parentPath, err)
			}
		}
	}

	// Write the materialized result to the store
	if err := m.store.Write(ctx, txn, storage.AddOp, storePath, storageValue); err != nil {
		return fmt.Errorf("failed to write materialized view to %v: %w", storePath, err)
	}

	return nil
}

// deriveStoragePath converts a module package path to a storage path
// e.g., package system.store.my_view -> /system/store/my_view
func deriveStoragePath(module *ast.Module) (storage.Path, error) {
	if module.Package == nil {
		return nil, ErrModuleNoPackage
	}

	if len(module.Package.Path) < 2 {
		return nil, fmt.Errorf("invalid system.store package path: %v", module.Package.Path)
	}

	// Convert ast.Ref to storage.Path
	// Skip "data" prefix if present
	// Direct allocation - no pool since result is returned (copy would be needed anyway)
	storagePath := make(storage.Path, 0, len(module.Package.Path))

	for i, term := range module.Package.Path {
		if i == 0 && term.Value.Compare(ast.DefaultRootDocument.Value) == 0 {
			// Skip "data" prefix
			continue
		}

		str, ok := term.Value.(ast.String)
		if !ok {
			return nil, fmt.Errorf("non-string component in package path: %v", term)
		}

		storagePath = append(storagePath, string(str))
	}

	return storagePath, nil
}

// deriveStoragePathForRule converts a module package path and rule name to a storage path
// e.g., package system.store + rule developers -> /system/store/developers
func deriveStoragePathForRule(module *ast.Module, rule *ast.Rule) (storage.Path, error) {
	basePath, err := deriveStoragePath(module)
	if err != nil {
		return nil, err
	}

	// Append the rule name to the base path
	// Optimize: pre-allocate with exact size to avoid reallocation
	resultPath := make(storage.Path, len(basePath), len(basePath)+1)
	copy(resultPath, basePath)
	return append(resultPath, rule.Head.Name.String()), nil
}

// isEvaluatingSystemStore checks if we're currently evaluating system.store policies
func isEvaluatingSystemStore(ctx context.Context) bool {
	return ctx.Value(evaluatingSystemStoreKey) != nil
}

// markEvaluatingSystemStore marks the context as currently evaluating system.store policies
func markEvaluatingSystemStore(ctx context.Context) context.Context {
	return context.WithValue(ctx, evaluatingSystemStoreKey, true)
}
