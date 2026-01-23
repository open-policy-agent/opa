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
	"github.com/open-policy-agent/opa/v1/topdown"
)

// BenchmarkMaterializeSmallDataset benchmarks materialization with 100 items
func BenchmarkMaterializeSmallDataset(b *testing.B) {
	benchmarkMaterialize(b, 100)
}

// BenchmarkMaterializeMediumDataset benchmarks materialization with 1000 items
func BenchmarkMaterializeMediumDataset(b *testing.B) {
	benchmarkMaterialize(b, 1000)
}

// BenchmarkMaterializeLargeDataset benchmarks materialization with 10000 items
func BenchmarkMaterializeLargeDataset(b *testing.B) {
	benchmarkMaterialize(b, 10000)
}

func benchmarkMaterialize(b *testing.B, numItems int) {
	ctx := context.Background()

	// Generate test data
	users := make([]map[string]any, numItems)
	for i := range numItems {
		users[i] = map[string]any{
			"id":       fmt.Sprintf("user_%d", i),
			"name":     fmt.Sprintf("User %d", i),
			"age":      20 + (i % 60),
			"active":   i%3 != 0,
			"verified": i%2 == 0,
			"role":     []string{"user", "admin", "moderator"}[i%3],
			"profile": map[string]any{
				"bio":   fmt.Sprintf("Bio for user %d", i),
				"email": fmt.Sprintf("user%d@example.com", i),
			},
		}
	}

	// Define system.store policy
	module := `
		package system.store

		# Filter active users
		active_users contains user if {
			some user in data.users
			user.active == true
		}

		# Filter verified admins
		verified_admins contains user if {
			some user in data.users
			user.verified == true
			user.role == "admin"
		}

		# Count by role
		user_count_by_role[role] := count(users) if {
			role := ["user", "admin", "moderator"][_]
			users := [u | u := data.users[_]; u.role == role]
		}

		# Average age
		average_age := sum([user.age | user := data.users[_]]) / count(data.users)
	`

	compiler := ast.MustCompileModules(map[string]string{"bench.rego": module})

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		store := inmem.New()
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)

		// Write data
		_ = store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users"), users)

		// Evaluate system.store policies
		mgr := NewManager(compiler, store)
		if err := mgr.EvaluateSystemStore(ctx, txn); err != nil {
			b.Fatal(err)
		}

		store.Abort(ctx, txn)
	}
}

// BenchmarkWithoutMaterialization benchmarks query without materialized views
func BenchmarkWithoutMaterialization(b *testing.B) {
	ctx := context.Background()

	// Generate 1000 users
	users := make([]map[string]any, 1000)
	for i := range 1000 {
		users[i] = map[string]any{
			"id":       fmt.Sprintf("user_%d", i),
			"name":     fmt.Sprintf("User %d", i),
			"active":   i%3 != 0,
			"verified": i%2 == 0,
			"role":     []string{"user", "admin", "moderator"}[i%3],
		}
	}

	store := inmem.New()
	txn, _ := store.NewTransaction(ctx, storage.WriteParams)
	defer store.Abort(ctx, txn)
	_ = store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users"), users)

	// Regular policy (no materialization)
	module := `
		package test

		active_users := [user | user := data.users[_]; user.active == true]
	`

	compiler := ast.MustCompileModules(map[string]string{"test.rego": module})
	query := ast.MustParseBody("data.test.active_users")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		q := topdown.NewQuery(query).
			WithCompiler(compiler).
			WithStore(store).
			WithTransaction(txn)

		_, err := q.Run(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkWithMaterialization benchmarks query using materialized view
func BenchmarkWithMaterialization(b *testing.B) {
	ctx := context.Background()

	// Generate 1000 users
	users := make([]map[string]any, 1000)
	for i := range 1000 {
		users[i] = map[string]any{
			"id":       fmt.Sprintf("user_%d", i),
			"name":     fmt.Sprintf("User %d", i),
			"active":   i%3 != 0,
			"verified": i%2 == 0,
			"role":     []string{"user", "admin", "moderator"}[i%3],
		}
	}

	store := inmem.New()
	txn, _ := store.NewTransaction(ctx, storage.WriteParams)
	defer store.Abort(ctx, txn)
	_ = store.Write(ctx, txn, storage.AddOp, storage.MustParsePath("/users"), users)

	// Materialize view
	materializeModule := `
		package system.store

		active_users contains user if {
			some user in data.users
			user.active == true
		}
	`

	compiler := ast.MustCompileModules(map[string]string{"materialize.rego": materializeModule})
	mgr := NewManager(compiler, store)
	_ = mgr.EvaluateSystemStore(ctx, txn)

	// Query using materialized view
	queryModule := `
		package test

		active_users := data.system.store.active_users
	`

	queryCompiler := ast.MustCompileModules(map[string]string{"query.rego": queryModule})
	query := ast.MustParseBody("data.test.active_users")

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		q := topdown.NewQuery(query).
			WithCompiler(queryCompiler).
			WithStore(store).
			WithTransaction(txn)

		_, err := q.Run(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParentPathCreation benchmarks parent path creation overhead
func BenchmarkParentPathCreation(b *testing.B) {
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		store := inmem.New()
		txn, _ := store.NewTransaction(ctx, storage.WriteParams)

		// Simulate creating nested path /system/store/nested/deep/path
		storePath := storage.MustParsePath("/system/store/nested/deep/path")

		// Create parent paths
		for i := 1; i < len(storePath); i++ {
			parentPath := storePath[:i]
			if _, err := store.Read(ctx, txn, parentPath); err != nil {
				_ = store.Write(ctx, txn, storage.AddOp, parentPath, map[string]any{})
			}
		}

		// Write final value
		_ = store.Write(ctx, txn, storage.AddOp, storePath, "test_value")

		store.Abort(ctx, txn)
	}
}

// BenchmarkASTToNativeConversion benchmarks AST to native type conversion
func BenchmarkASTToNativeConversion(b *testing.B) {
	// Create a set with 100 items
	set := ast.NewSet()
	for i := range 100 {
		obj := ast.NewObject()
		obj.Insert(ast.StringTerm("id"), ast.IntNumberTerm(i))
		obj.Insert(ast.StringTerm("name"), ast.StringTerm(fmt.Sprintf("user_%d", i)))
		set.Add(ast.NewTerm(obj))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for b.Loop() {
		_, err := ast.JSON(set)
		if err != nil {
			b.Fatal(err)
		}
	}
}
