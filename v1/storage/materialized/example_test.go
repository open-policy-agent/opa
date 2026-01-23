// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package materialized_test

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/storage/materialized"
)

// Example demonstrates basic usage of materialized views
func Example() {
	ctx := context.Background()

	// Create store with sample data
	store := inmem.New()
	txn, _ := store.NewTransaction(ctx, storage.WriteParams)
	defer store.Abort(ctx, txn)

	// Write user data
	users := []map[string]any{
		{"name": "alice", "role": "admin", "active": true},
		{"name": "bob", "role": "user", "active": true},
		{"name": "charlie", "role": "admin", "active": false},
	}
	_ = store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users"), users)

	// Define system.store policy to filter active admins
	policy := `
		package system.store

		active_admins contains user if {
			some user in data.users
			user.role == "admin"
			user.active == true
		}
	`

	// Compile policy
	compiler := ast.MustCompileModules(map[string]string{"policy.rego": policy})

	// Evaluate system.store policies to create materialized views
	mgr := materialized.NewManager(compiler, store)
	_ = mgr.EvaluateSystemStore(ctx, txn)

	// Read the materialized view
	result, _ := store.Read(ctx, txn, storage.MustParsePath("/system/store/active_admins"))
	admins := result.([]any)

	fmt.Printf("Active admins: %d\n", len(admins))
	// Output: Active admins: 1
}

// Example_completeRule demonstrates a complete rule materialized view
func Example_completeRule() {
	ctx := context.Background()

	store := inmem.New()
	txn, _ := store.NewTransaction(ctx, storage.WriteParams)
	defer store.Abort(ctx, txn)

	// Write configuration
	config := map[string]any{
		"limits": map[string]any{
			"max_requests_per_minute": 1000,
			"max_concurrent_users":    500,
		},
	}
	_ = store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/config"), config)

	// Complete rule that extracts a single value
	policy := `
		package system.store

		# Extract the rate limit for easy access
		rate_limit := data.config.limits.max_requests_per_minute
	`

	compiler := ast.MustCompileModules(map[string]string{"policy.rego": policy})

	mgr := materialized.NewManager(compiler, store)
	_ = mgr.EvaluateSystemStore(ctx, txn)

	// Read the materialized value
	result, _ := store.Read(ctx, txn, storage.MustParsePath("/system/store/rate_limit"))

	fmt.Printf("Rate limit: %v\n", result)
	// Output: Rate limit: 1000
}

// Example_multipleViews demonstrates multiple materialized views in one package
func Example_multipleViews() {
	ctx := context.Background()

	store := inmem.New()
	txn, _ := store.NewTransaction(ctx, storage.WriteParams)
	defer store.Abort(ctx, txn)

	// Write employee data
	employees := []map[string]any{
		{"name": "alice", "department": "engineering", "level": "senior"},
		{"name": "bob", "department": "sales", "level": "junior"},
		{"name": "charlie", "department": "engineering", "level": "junior"},
		{"name": "diana", "department": "sales", "level": "senior"},
	}
	_ = store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/employees"), employees)

	// Multiple views in same package
	policy := `
		package system.store

		# View 1: All engineers
		engineers contains emp if {
			some emp in data.employees
			emp.department == "engineering"
		}

		# View 2: All senior employees
		senior_staff contains emp if {
			some emp in data.employees
			emp.level == "senior"
		}

		# View 3: Count by department
		department_counts[dept] := count(emps) if {
			dept := ["engineering", "sales"][_]
			emps := [e | e := data.employees[_]; e.department == dept]
		}
	`

	compiler := ast.MustCompileModules(map[string]string{"views.rego": policy})

	mgr := materialized.NewManager(compiler, store)
	_ = mgr.EvaluateSystemStore(ctx, txn)

	// Read all views
	engineers, _ := store.Read(ctx, txn, storage.MustParsePath("/system/store/engineers"))
	seniorStaff, _ := store.Read(ctx, txn, storage.MustParsePath("/system/store/senior_staff"))

	fmt.Printf("Engineers: %d\n", len(engineers.([]any)))
	fmt.Printf("Senior staff: %d\n", len(seniorStaff.([]any)))
	// Output:
	// Engineers: 2
	// Senior staff: 2
}
