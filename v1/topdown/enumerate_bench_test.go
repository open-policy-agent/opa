// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

// BenchmarkEnumerateComprehensions benchmarks policy evaluation with
// comprehensions over large datasets. This specifically targets the
// enumerate optimization that eliminates closure allocations.
func BenchmarkEnumerateComprehensions(b *testing.B) {
	sizes := []int{1000, 5000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size_%d", size), func(b *testing.B) {
			ctx := context.Background()

			// Generate mock dataset with nested objects
			data := generateNestedDataset(size)
			store := inmem.NewFromObject(data)

			// Policy with multiple comprehensions that exercise enumerate
			module := `package test

import rego.v1

# Set comprehension over users
active_users contains user.id if {
	some user in data.users
	user.profile.active == true
}

# Array comprehension with nested access
premium_users := [user |
	some user in data.users
	user.profile.settings.subscription.tier == "premium"
]

# Object comprehension with filtering
users_by_age contains age_group if {
	age_group := "20-30"
	some u in data.users
	u.profile.age >= 20
	u.profile.age < 30
}

users_by_age contains age_group if {
	age_group := "30-40"
	some u in data.users
	u.profile.age >= 30
	u.profile.age < 40
}

# Nested comprehension
high_value_users contains user.email if {
	some user in data.users
	user.profile.active == true
	count([p | some p in user.permissions; p.level > 5]) > 0
}

# Random access pattern
user_lookup[id] := user if {
	some user in data.users
	id := user.id
}
			`

			compiler := ast.MustCompileModules(map[string]string{
				"test.rego": module,
			})

			// Query that exercises all comprehensions
			query := ast.MustParseBody(`
				data.test.active_users
				data.test.premium_users
				data.test.users_by_age
				data.test.high_value_users
			`)

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
					q := NewQuery(query).
						WithCompiler(compiler).
						WithStore(store).
						WithTransaction(txn)

					_, err := q.Run(ctx)
					return err
				})

				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// generateNestedDataset creates a dataset with nested objects of varying depth
func generateNestedDataset(size int) map[string]any {
	users := make([]any, size)
	rng := rand.New(rand.NewSource(42)) // Fixed seed for reproducibility

	tiers := []string{"free", "basic", "premium", "enterprise"}
	departments := []string{"engineering", "sales", "marketing", "support", "hr"}

	for i := range size {
		// Random nested object with 3-5 levels of nesting
		permissions := make([]any, rng.Intn(10)+1)
		for j := 0; j < len(permissions); j++ {
			permissions[j] = map[string]any{
				"name":  fmt.Sprintf("perm_%d", j),
				"level": rng.Intn(10),
				"scope": map[string]any{
					"resource": fmt.Sprintf("res_%d", rng.Intn(100)),
					"actions":  []string{"read", "write", "delete"}[rng.Intn(3)],
				},
			}
		}

		users[i] = map[string]any{
			"id":    fmt.Sprintf("user_%d", i),
			"name":  fmt.Sprintf("User %d", i),
			"email": fmt.Sprintf("user%d@example.com", i),
			"profile": map[string]any{
				"active": rng.Float64() > 0.3, // 70% active
				"age":    20 + rng.Intn(40),   // Age 20-59
				"settings": map[string]any{
					"subscription": map[string]any{
						"tier":       tiers[rng.Intn(len(tiers))],
						"start_date": "2024-01-01",
						"features": map[string]any{
							"api_access":    rng.Float64() > 0.5,
							"custom_domain": rng.Float64() > 0.7,
							"priority_support": map[string]any{
								"enabled": rng.Float64() > 0.8,
								"level":   rng.Intn(5) + 1,
							},
						},
					},
					"notifications": map[string]any{
						"email": rng.Float64() > 0.4,
						"sms":   rng.Float64() > 0.8,
					},
				},
				"department": departments[rng.Intn(len(departments))],
			},
			"permissions": permissions,
			"metadata": map[string]any{
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-15T00:00:00Z",
				"tags": map[string]any{
					"region":      []string{"us-west", "us-east", "eu-central"}[rng.Intn(3)],
					"environment": []string{"prod", "staging", "dev"}[rng.Intn(3)],
					"cost_center": fmt.Sprintf("CC%04d", rng.Intn(1000)),
				},
			},
		}
	}

	return map[string]any{
		"users": users,
	}
}

// BenchmarkEnumerateRandomAccess benchmarks random access patterns
// that exercise virtual document enumeration
func BenchmarkEnumerateRandomAccess(b *testing.B) {
	ctx := context.Background()

	data := generateNestedDataset(10000)
	store := inmem.NewFromObject(data)

	module := `package test

import rego.v1

# Virtual document with random access
user_by_id[id] := user if {
	some user in data.users
	id := user.id
}

# Nested virtual document access
premium_by_dept[dept] := users if {
	dept := data.users[_].profile.department
	users := [u |
		some u in data.users
		u.profile.department == dept
		u.profile.settings.subscription.tier == "premium"
	]
}
	`

	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": module,
	})

	// Access random users
	query := ast.MustParseBody(`
		data.test.user_by_id["user_1234"]
		data.test.user_by_id["user_5678"]
		data.test.premium_by_dept.engineering
	`)

	b.ReportAllocs()

	for b.Loop() {
		err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
			q := NewQuery(query).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn)

			_, err := q.Run(ctx)
			return err
		})

		if err != nil {
			b.Fatal(err)
		}
	}
}
