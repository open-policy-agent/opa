// Copyright 2026 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

// BenchmarkLazyObjRealWorkload - realistic benchmark for lazyObj optimization
// Tests actual policy evaluation with large nested datasets
func BenchmarkLazyObjRealWorkload(b *testing.B) {
	ctx := context.Background()

	// Generate realistic large dataset: 10K users with nested structure
	// This simulates a real API gateway or authorization service scenario
	users := make(map[string]any, 10000)
	for i := range 10000 {
		userID := fmt.Sprintf("user_%d", i)
		users[userID] = map[string]any{
			"id":       userID,
			"name":     fmt.Sprintf("User %d", i),
			"email":    fmt.Sprintf("user%d@example.com", i),
			"age":      20 + (i % 60),
			"country":  []string{"US", "UK", "DE", "FR", "JP"}[i%5],
			"active":   i%3 != 0,
			"verified": i%2 == 0,
			"profile": map[string]any{
				"bio":    fmt.Sprintf("Bio for user %d", i),
				"avatar": fmt.Sprintf("https://cdn.example.com/avatars/%d.jpg", i),
				"social": map[string]any{
					"twitter":  fmt.Sprintf("@user%d", i),
					"github":   fmt.Sprintf("user%d", i),
					"linkedin": fmt.Sprintf("user-%d", i),
				},
				"preferences": map[string]any{
					"theme":    []string{"dark", "light", "auto"}[i%3],
					"language": []string{"en", "es", "de", "fr", "ja"}[i%5],
					"timezone": "UTC",
					"notifications": map[string]any{
						"email": i%2 == 0,
						"push":  i%3 == 0,
						"sms":   i%4 == 0,
					},
				},
			},
			"subscription": map[string]any{
				"tier":       []string{"free", "basic", "premium", "enterprise"}[i%4],
				"started_at": "2024-01-01",
				"expires_at": "2027-01-01",
				"features": []any{
					"feature1", "feature2", "feature3",
				},
				"limits": map[string]any{
					"api_calls":  1000 * (i%10 + 1),
					"storage_gb": 10 * (i%5 + 1),
					"bandwidth":  100 * (i%3 + 1),
				},
			},
			"permissions": []any{
				"read:own", "write:own", "delete:own",
			},
			"roles": []any{
				[]string{"user", "admin", "moderator", "guest"}[i%4],
			},
			"metadata": map[string]any{
				"created_at":    "2024-01-01T00:00:00Z",
				"updated_at":    "2026-01-20T00:00:00Z",
				"last_login":    "2026-01-22T12:00:00Z",
				"login_count":   i * 10,
				"failed_logins": i % 10,
				"ip_address":    fmt.Sprintf("192.168.%d.%d", i/256, i%256),
				"user_agent":    "Mozilla/5.0",
				"session_id":    fmt.Sprintf("sess_%d", i),
				"fingerprint":   fmt.Sprintf("fp_%d", i),
			},
		}
	}

	data := map[string]any{
		"users": users,
		"config": map[string]any{
			"max_users":        10000,
			"rate_limit":       1000,
			"allow_signup":     true,
			"require_verified": true,
		},
	}

	store := inmem.NewFromObject(data)

	// Real-world Rego policy that exercises lazyObj hot paths:
	// - Keys() iteration (comprehensions)
	// - Hash() computation (map lookups)
	// - Get() access (nested field access)
	// - Find() traversal (path resolution)
	module := `
	package authz

	# Rule 1: Filter active users (Keys iteration + Get)
	active_users := [user |
		user := data.users[_]
		user.active == true
	]

	# Rule 2: Count premium subscribers (comprehension)
	premium_count := count([1 |
		user := data.users[_]
		user.subscription.tier == "premium"
	])

	# Rule 3: Find users by country (nested access)
	users_by_country[country] := users if {
		country := ["US", "UK", "DE", "FR", "JP"][_]
		users := [user |
			user := data.users[_]
			user.country == country
		]
	}

	# Rule 4: Check user permissions (deep nesting)
	user_has_permission(user_id, permission) if {
		user := data.users[user_id]
		permission == user.permissions[_]
	}

	# Rule 5: Aggregate subscription limits (complex access)
	total_api_calls := sum([limits |
		user := data.users[_]
		limits := user.subscription.limits.api_calls
	])

	# Rule 6: Find verified admins (multiple conditions)
	verified_admins := [user |
		user := data.users[_]
		user.verified == true
		user.roles[_] == "admin"
	]

	# Rule 7: Profile completeness check (nested field access)
	complete_profiles := [user_id |
		user := data.users[user_id]
		user.profile.bio != ""
		user.profile.social.twitter != ""
		user.profile.preferences.theme != ""
	]
	`

	query := ast.MustParseBody("data.authz")
	compiler := ast.MustCompileModules(map[string]string{
		"authz.rego": module,
	})

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
