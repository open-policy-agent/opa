// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"runtime"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

// Benchmark rule evaluation with varying body sizes
func BenchmarkRuleBindings(b *testing.B) {
	testCases := []struct {
		name   string
		module string
		query  string
	}{
		{
			name: "Rule_1Expr",
			module: `package test
import rego.v1

allow if {
	input.user == "admin"
}`,
			query: "data.test.allow",
		},
		{
			name: "Rule_2Exprs",
			module: `package test
import rego.v1

allow if {
	input.user == "admin"
	input.role == "superuser"
}`,
			query: "data.test.allow",
		},
		{
			name: "Rule_3Exprs",
			module: `package test
import rego.v1

allow if {
	input.user == "admin"
	input.role == "superuser"
	input.department == "engineering"
}`,
			query: "data.test.allow",
		},
		{
			name: "Rule_5Exprs",
			module: `package test
import rego.v1

allow if {
	user := input.user
	role := input.role
	user == "admin"
	role == "superuser"
	input.active == true
}`,
			query: "data.test.allow",
		},
		{
			name: "Rule_10Exprs",
			module: `package test
import rego.v1

allow if {
	user := input.user
	role := input.role
	dept := input.department
	status := input.status
	level := input.level
	user == "admin"
	role == "superuser"
	dept == "engineering"
	status == "active"
	level > 5
}`,
			query: "data.test.allow",
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			mod := ast.MustParseModule(tc.module)
			compiler := ast.NewCompiler()
			compiler.Compile(map[string]*ast.Module{"test": mod})
			if compiler.Failed() {
				b.Fatalf("Compilation failed: %v", compiler.Errors)
			}

			store := inmem.New()
			ctx := context.Background()
			input := map[string]any{
				"user":       "admin",
				"role":       "superuser",
				"department": "engineering",
				"status":     "active",
				"level":      10,
				"active":     true,
			}

			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()

			for b.Loop() {
				q := NewQuery(ast.MustParseBody(tc.query)).
					WithCompiler(compiler).
					WithStore(store).
					WithInput(ast.NewTerm(ast.MustInterfaceToValue(input)))

				_, err := q.Run(ctx)
				if err != nil {
					b.Fatalf("Query failed: %v", err)
				}
			}

			b.StopTimer()

			runtime.GC()
			runtime.ReadMemStats(&m2)

			allocPerOp := (m2.TotalAlloc - m1.TotalAlloc) / uint64(b.N)
			mallocsPerOp := (m2.Mallocs - m1.Mallocs) / uint64(b.N)

			b.ReportMetric(float64(allocPerOp), "B/op")
			b.ReportMetric(float64(mallocsPerOp), "allocs/op")
		})
	}
}

// Benchmark rules with comprehensions in body
func BenchmarkRuleWithComprehensions(b *testing.B) {
	testCases := []struct {
		name   string
		module string
		query  string
	}{
		{
			name: "Rule_WithArrayComp",
			module: `package test
import rego.v1

filtered_users := [u |
	some u in input.users
	u.active == true
]

result if {
	count(filtered_users) > 0
}`,
			query: "data.test.result",
		},
		{
			name: "Rule_WithSetComp",
			module: `package test
import rego.v1

admin_names := {u.name |
	some u in input.users
	u.role == "admin"
}

result if {
	count(admin_names) > 0
}`,
			query: "data.test.result",
		},
		{
			name: "Rule_WithMultipleComps",
			module: `package test
import rego.v1

active_users := [u | some u in input.users; u.active]
admin_users := [u | some u in active_users; u.role == "admin"]

result if {
	count(admin_users) > 0
}`,
			query: "data.test.result",
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			mod := ast.MustParseModule(tc.module)
			compiler := ast.NewCompiler()
			compiler.Compile(map[string]*ast.Module{"test": mod})
			if compiler.Failed() {
				b.Fatalf("Compilation failed: %v", compiler.Errors)
			}

			store := inmem.New()
			ctx := context.Background()
			input := map[string]any{
				"users": []map[string]any{
					{"name": "alice", "role": "admin", "active": true},
					{"name": "bob", "role": "user", "active": true},
					{"name": "charlie", "role": "admin", "active": false},
				},
			}

			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()

			for b.Loop() {
				q := NewQuery(ast.MustParseBody(tc.query)).
					WithCompiler(compiler).
					WithStore(store).
					WithInput(ast.NewTerm(ast.MustInterfaceToValue(input)))

				_, err := q.Run(ctx)
				if err != nil {
					b.Fatalf("Query failed: %v", err)
				}
			}

			b.StopTimer()

			runtime.GC()
			runtime.ReadMemStats(&m2)

			allocPerOp := (m2.TotalAlloc - m1.TotalAlloc) / uint64(b.N)
			mallocsPerOp := (m2.Mallocs - m1.Mallocs) / uint64(b.N)

			b.ReportMetric(float64(allocPerOp), "B/op")
			b.ReportMetric(float64(mallocsPerOp), "allocs/op")
		})
	}
}

// Benchmark complex rule scenarios
func BenchmarkComplexRules(b *testing.B) {
	module := `package test
import rego.v1

# Multi-expression rule with assignments
process_request if {
	user := input.user
	resource := input.resource
	action := input.action
	user.authenticated
	resource.accessible
	allowed_actions := ["read", "write", "delete"]
	action in allowed_actions
	user.role == "admin"
}

# Rule with nested conditions
check_permissions if {
	user := input.user
	required_role := "admin"
	user.role == required_role
	perms := user.permissions
	"write" in perms
	user.active == true
}

# Rule with multiple comprehensions
analyze_access if {
	users := [u | some u in input.users; u.active]
	admins := {u.name | some u in users; u.role == "admin"}
	count(admins) > 0
}`

	mod := ast.MustParseModule(module)
	compiler := ast.NewCompiler()
	compiler.Compile(map[string]*ast.Module{"test": mod})
	if compiler.Failed() {
		b.Fatalf("Compilation failed: %v", compiler.Errors)
	}

	store := inmem.New()
	ctx := context.Background()

	testCases := []struct {
		name  string
		query string
		input map[string]any
	}{
		{
			name:  "ProcessRequest",
			query: "data.test.process_request",
			input: map[string]any{
				"user": map[string]any{
					"authenticated": true,
					"role":          "admin",
				},
				"resource": map[string]any{
					"accessible": true,
				},
				"action": "read",
			},
		},
		{
			name:  "CheckPermissions",
			query: "data.test.check_permissions",
			input: map[string]any{
				"user": map[string]any{
					"role":        "admin",
					"permissions": []string{"read", "write"},
					"active":      true,
				},
			},
		},
		{
			name:  "AnalyzeAccess",
			query: "data.test.analyze_access",
			input: map[string]any{
				"users": []map[string]any{
					{"name": "alice", "role": "admin", "active": true},
					{"name": "bob", "role": "user", "active": true},
					{"name": "charlie", "role": "admin", "active": false},
				},
			},
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			var m1, m2 runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&m1)

			b.ResetTimer()

			for b.Loop() {
				q := NewQuery(ast.MustParseBody(tc.query)).
					WithCompiler(compiler).
					WithStore(store).
					WithInput(ast.NewTerm(ast.MustInterfaceToValue(tc.input)))

				_, err := q.Run(ctx)
				if err != nil {
					b.Fatalf("Query failed: %v", err)
				}
			}

			b.StopTimer()

			runtime.GC()
			runtime.ReadMemStats(&m2)

			allocPerOp := (m2.TotalAlloc - m1.TotalAlloc) / uint64(b.N)
			mallocsPerOp := (m2.Mallocs - m1.Mallocs) / uint64(b.N)

			b.ReportMetric(float64(allocPerOp), "B/op")
			b.ReportMetric(float64(mallocsPerOp), "allocs/op")
		})
	}
}
