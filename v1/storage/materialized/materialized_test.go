// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package materialized

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func TestBasicMaterialization(t *testing.T) {
	ctx := context.Background()

	// Setup store with test data
	store := inmem.New()
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}
	defer store.Abort(ctx, txn)

	// Write test data: users with roles
	users := []map[string]any{
		{"name": "alice", "roles": []any{"developer", "admin"}},
		{"name": "bob", "roles": []any{"user"}},
		{"name": "charlie", "roles": []any{"developer"}},
	}

	if err := store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users"), users); err != nil {
		t.Fatalf("failed to write users data: %v", err)
	}

	// Create system.store policy to filter developers
	module := `
		package system.store

		developers contains user if {
			some user in data.users
			"developer" in user.roles
		}
	`

	compiler := ast.MustCompileModules(map[string]string{"test.rego": module})

	// Evaluate system.store policies
	mgr := NewManager(compiler, store)
	if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
		t.Fatalf("failed to evaluate system.store: %v", err)
	}

	// Verify materialized view was created
	result, err := store.Read(ctx, txn, storage.MustParsePath("/system/store/developers"))
	if err != nil {
		t.Fatalf("failed to read materialized view: %v", err)
	}

	// Storage returns native Go types, so we expect a slice
	resultSlice, ok := result.([]any)
	if !ok {
		t.Fatalf("expected slice, got %T", result)
	}

	// Should contain alice and charlie (both developers)
	expectedCount := 2
	if len(resultSlice) != expectedCount {
		t.Errorf("expected %d developers, got %d", expectedCount, len(resultSlice))
	}

	// Verify alice and charlie are in the result
	foundAlice := false
	foundCharlie := false

	for _, item := range resultSlice {
		obj, ok := item.(map[string]any)
		if !ok {
			t.Errorf("expected map[string]any, got %T", item)
			continue
		}

		name, ok := obj["name"].(string)
		if !ok {
			continue
		}

		if name == "alice" {
			foundAlice = true
		}
		if name == "charlie" {
			foundCharlie = true
		}
	}

	if !foundAlice {
		t.Error("alice should be in developers set")
	}
	if !foundCharlie {
		t.Error("charlie should be in developers set")
	}
}

func TestMultiplePolicies(t *testing.T) {
	ctx := context.Background()

	store := inmem.New()
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}
	defer store.Abort(ctx, txn)

	// Write test data
	users := []map[string]any{
		{"name": "alice", "age": 30, "active": true},
		{"name": "bob", "age": 25, "active": false},
		{"name": "charlie", "age": 35, "active": true},
	}

	if err := store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users"), users); err != nil {
		t.Fatalf("failed to write users: %v", err)
	}

	// Multiple system.store policies
	module := `
		package system.store

		# Active users
		active_users contains user if {
			some user in data.users
			user.active == true
		}

		# Senior users (age > 30)
		senior_users contains user if {
			some user in data.users
			user.age > 30
		}
	`

	compiler := ast.MustCompileModules(map[string]string{"test.rego": module})

	mgr := NewManager(compiler, store)
	if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
		t.Fatalf("failed to evaluate system.store: %v", err)
	}

	// Verify active_users
	activeUsers, err := store.Read(ctx, txn, storage.MustParsePath("/system/store/active_users"))
	if err != nil {
		t.Fatalf("failed to read active_users: %v", err)
	}

	activeSlice := activeUsers.([]any)
	if len(activeSlice) != 2 {
		t.Errorf("expected 2 active users, got %d", len(activeSlice))
	}

	// Verify senior_users
	seniorUsers, err := store.Read(ctx, txn, storage.MustParsePath("/system/store/senior_users"))
	if err != nil {
		t.Fatalf("failed to read senior_users: %v", err)
	}

	seniorSlice := seniorUsers.([]any)
	if len(seniorSlice) != 1 {
		t.Errorf("expected 1 senior user, got %d", len(seniorSlice))
	}
}

func TestRecursionPrevention(t *testing.T) {
	ctx := context.Background()

	store := inmem.New()
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}
	defer store.Abort(ctx, txn)

	// Create a policy that would cause recursion
	module := `
		package system.store

		# This tries to read from system.store itself
		test_value := data.system.store.other_value
	`

	compiler := ast.MustCompileModules(map[string]string{"test.rego": module})

	mgr := NewManager(compiler, store)

	// Mark context as already evaluating
	ctx = markEvaluatingSystemStore(ctx)

	// Should return immediately without error due to recursion prevention
	if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
		t.Fatalf("recursion prevention failed: %v", err)
	}

	// Verify no materialized view was created
	_, err = store.Read(ctx, txn, storage.MustParsePath("/system/store/test_value"))
	if err == nil {
		t.Error("expected error reading non-existent materialized view")
	}
}

func TestNoSystemStorePolicies(t *testing.T) {
	ctx := context.Background()

	store := inmem.New()
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}
	defer store.Abort(ctx, txn)

	// Module NOT in system.store namespace
	module := `
		package test

		allow := true
	`

	compiler := ast.MustCompileModules(map[string]string{"test.rego": module})

	mgr := NewManager(compiler, store)

	// Should succeed without doing anything
	if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeriveStoragePath(t *testing.T) {
	tests := []struct {
		name        string
		packagePath ast.Ref
		expected    storage.Path
		expectError bool
	}{
		{
			name: "simple system.store path",
			packagePath: ast.Ref{
				ast.StringTerm("system"),
				ast.StringTerm("store"),
				ast.StringTerm("my_view"),
			},
			expected:    storage.MustParsePath("/system/store/my_view"),
			expectError: false,
		},
		{
			name: "nested system.store path",
			packagePath: ast.Ref{
				ast.StringTerm("system"),
				ast.StringTerm("store"),
				ast.StringTerm("users"),
				ast.StringTerm("active"),
			},
			expected:    storage.MustParsePath("/system/store/users/active"),
			expectError: false,
		},
		{
			name: "with data prefix",
			packagePath: ast.Ref{
				ast.DefaultRootDocument,
				ast.StringTerm("system"),
				ast.StringTerm("store"),
				ast.StringTerm("test"),
			},
			expected:    storage.MustParsePath("/system/store/test"),
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			module := &ast.Module{
				Package: &ast.Package{
					Path: tc.packagePath,
				},
			}

			result, err := deriveStoragePath(module)

			if tc.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !result.Equal(tc.expected) {
				t.Errorf("expected path %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestCompleteRule(t *testing.T) {
	ctx := context.Background()

	store := inmem.New()
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}
	defer store.Abort(ctx, txn)

	// Write test data
	if err := store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/config"),
		map[string]any{"max_users": 100, "enabled": true}); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Complete rule (not a set) - single value
	module := `
		package system.store

		# Complete rule returning a single value
		max_allowed_users := data.config.max_users
	`

	compiler := ast.MustCompileModules(map[string]string{"test.rego": module})

	mgr := NewManager(compiler, store)
	if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
		t.Fatalf("failed to evaluate system.store: %v", err)
	}

	// Verify materialized complete rule
	result, err := store.Read(ctx, txn, storage.MustParsePath("/system/store/max_allowed_users"))
	if err != nil {
		t.Fatalf("failed to read materialized view: %v", err)
	}

	// Storage returns native types - verify the value represents 100
	// Could be json.Number, int, or float64 depending on storage backend
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	// Convert to string for comparison
	resultStr := fmt.Sprintf("%v", result)
	if resultStr != "100" {
		t.Errorf("expected 100, got %s", resultStr)
	}
}

func TestEmptyResult(t *testing.T) {
	ctx := context.Background()

	store := inmem.New()
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}
	defer store.Abort(ctx, txn)

	// Write test data with no developers
	users := []map[string]any{
		{"name": "alice", "roles": []any{"admin"}},
		{"name": "bob", "roles": []any{"user"}},
	}

	if err := store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users"), users); err != nil {
		t.Fatalf("failed to write users: %v", err)
	}

	// Policy that matches nothing
	module := `
		package system.store

		developers contains user if {
			some user in data.users
			"developer" in user.roles
		}
	`

	compiler := ast.MustCompileModules(map[string]string{"test.rego": module})

	mgr := NewManager(compiler, store)
	if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
		t.Fatalf("failed to evaluate system.store: %v", err)
	}

	// Empty result should not create a path
	_, err = store.Read(ctx, txn, storage.MustParsePath("/system/store/developers"))
	if err == nil {
		t.Error("expected error reading non-existent path, got nil")
	}
}

func TestNestedObjects(t *testing.T) {
	ctx := context.Background()

	store := inmem.New()
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}
	defer store.Abort(ctx, txn)

	// Write nested test data
	data := map[string]any{
		"departments": map[string]any{
			"engineering": map[string]any{
				"budget": 1000000,
				"employees": []any{
					map[string]any{"name": "alice", "level": "senior"},
					map[string]any{"name": "bob", "level": "junior"},
				},
			},
			"sales": map[string]any{
				"budget": 500000,
				"employees": []any{
					map[string]any{"name": "charlie", "level": "senior"},
				},
			},
		},
	}

	if err := store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/company"), data); err != nil {
		t.Fatalf("failed to write company data: %v", err)
	}

	// Policy with nested object access
	module := `
		package system.store

		senior_engineers contains emp if {
			dept := data.company.departments.engineering
			some emp in dept.employees
			emp.level == "senior"
		}
	`

	compiler := ast.MustCompileModules(map[string]string{"test.rego": module})

	mgr := NewManager(compiler, store)
	if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
		t.Fatalf("failed to evaluate system.store: %v", err)
	}

	// Verify result
	result, err := store.Read(ctx, txn, storage.MustParsePath("/system/store/senior_engineers"))
	if err != nil {
		t.Fatalf("failed to read materialized view: %v", err)
	}

	resultSlice, ok := result.([]any)
	if !ok {
		t.Fatalf("expected slice, got %T", result)
	}

	if len(resultSlice) != 1 {
		t.Errorf("expected 1 senior engineer, got %d", len(resultSlice))
	}

	// Verify alice is the senior engineer
	if len(resultSlice) > 0 {
		emp := resultSlice[0].(map[string]any)
		if name := emp["name"]; name != "alice" {
			t.Errorf("expected alice, got %v", name)
		}
	}
}

func TestMultipleModulesInPackage(t *testing.T) {
	ctx := context.Background()

	store := inmem.New()
	txn, err := store.NewTransaction(ctx, storage.WriteParams)
	if err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}
	defer store.Abort(ctx, txn)

	users := []map[string]any{
		{"name": "alice", "age": 25, "active": true},
		{"name": "bob", "age": 35, "active": false},
	}

	if err := store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users"), users); err != nil {
		t.Fatalf("failed to write users: %v", err)
	}

	// Multiple modules in same package
	module1 := `
		package system.store

		active_users contains user if {
			some user in data.users
			user.active == true
		}
	`

	module2 := `
		package system.store

		young_users contains user if {
			some user in data.users
			user.age < 30
		}
	`

	compiler := ast.MustCompileModules(map[string]string{
		"module1.rego": module1,
		"module2.rego": module2,
	})

	mgr := NewManager(compiler, store)
	if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
		t.Fatalf("failed to evaluate system.store: %v", err)
	}

	// Verify both views were created
	activeResult, err := store.Read(ctx, txn, storage.MustParsePath("/system/store/active_users"))
	if err != nil {
		t.Fatalf("failed to read active_users: %v", err)
	}
	if len(activeResult.([]any)) != 1 {
		t.Errorf("expected 1 active user, got %d", len(activeResult.([]any)))
	}

	youngResult, err := store.Read(ctx, txn, storage.MustParsePath("/system/store/young_users"))
	if err != nil {
		t.Fatalf("failed to read young_users: %v", err)
	}
	if len(youngResult.([]any)) != 1 {
		t.Errorf("expected 1 young user, got %d", len(youngResult.([]any)))
	}
}

func TestIsSystemStoreModule(t *testing.T) {
	tests := []struct {
		name        string
		packagePath ast.Ref
		expected    bool
	}{
		{
			name: "system.store module",
			packagePath: ast.Ref{
				ast.StringTerm("system"),
				ast.StringTerm("store"),
				ast.StringTerm("test"),
			},
			expected: true,
		},
		{
			name: "data.system.store module",
			packagePath: ast.Ref{
				ast.DefaultRootDocument,
				ast.StringTerm("system"),
				ast.StringTerm("store"),
				ast.StringTerm("test"),
			},
			expected: true,
		},
		{
			name: "nested system.store module",
			packagePath: ast.Ref{
				ast.StringTerm("system"),
				ast.StringTerm("store"),
				ast.StringTerm("users"),
				ast.StringTerm("active"),
			},
			expected: true,
		},
		{
			name: "not system.store",
			packagePath: ast.Ref{
				ast.StringTerm("test"),
				ast.StringTerm("policy"),
			},
			expected: false,
		},
		{
			name: "system but not store",
			packagePath: ast.Ref{
				ast.StringTerm("system"),
				ast.StringTerm("other"),
			},
			expected: false,
		},
		{
			name: "just system",
			packagePath: ast.Ref{
				ast.StringTerm("system"),
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			module := &ast.Module{
				Package: &ast.Package{
					Path: tc.packagePath,
				},
			}

			result := isSystemStoreModule(module)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestSentinelErrors verifies that sentinel errors are properly interned and can be used with errors.Is()
func TestSentinelErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("ErrManagerNotInitialized", func(t *testing.T) {
		// Test nil compiler
		mgr := &Manager{
			compiler: nil,
			store:    inmem.New(),
		}
		err := mgr.EvaluateSystemStore(ctx, nil)
		if err != ErrManagerNotInitialized {
			t.Errorf("expected ErrManagerNotInitialized, got %v", err)
		}

		// Test nil store
		mgr = &Manager{
			compiler: &ast.Compiler{},
			store:    nil,
		}
		err = mgr.EvaluateSystemStore(ctx, nil)
		if err != ErrManagerNotInitialized {
			t.Errorf("expected ErrManagerNotInitialized, got %v", err)
		}
	})

	t.Run("ErrInvalidModuleOrRule", func(t *testing.T) {
		mgr := NewManager(&ast.Compiler{}, inmem.New())

		// Test nil module
		err := mgr.evaluateRule(ctx, nil, nil, &ast.Rule{})
		if err != ErrInvalidModuleOrRule {
			t.Errorf("expected ErrInvalidModuleOrRule, got %v", err)
		}

		// Test nil rule
		module := &ast.Module{Package: &ast.Package{}}
		err = mgr.evaluateRule(ctx, nil, module, nil)
		if err != ErrInvalidModuleOrRule {
			t.Errorf("expected ErrInvalidModuleOrRule, got %v", err)
		}
	})

	t.Run("ErrRuleEmptyName", func(t *testing.T) {
		mgr := NewManager(&ast.Compiler{}, inmem.New())

		module := &ast.Module{
			Package: &ast.Package{
				Path: ast.Ref{ast.StringTerm("system"), ast.StringTerm("store")},
			},
		}
		rule := &ast.Rule{
			Head: &ast.Head{
				Name: ast.Var(""), // Empty name
			},
		}

		err := mgr.evaluateRule(ctx, nil, module, rule)
		if err != ErrRuleEmptyName {
			t.Errorf("expected ErrRuleEmptyName, got %v", err)
		}
	})

	t.Run("ErrModuleNoPackage", func(t *testing.T) {
		module := &ast.Module{
			Package: nil, // No package
		}

		_, err := deriveStoragePath(module)
		if err != ErrModuleNoPackage {
			t.Errorf("expected ErrModuleNoPackage, got %v", err)
		}
	})
}
