// Copyright 2024 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package benchmark

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBenchmarkSuite(t *testing.T) {
	suite := NewSuite()

	// Add simple test
	suite.Add(Test{
		Name: "simple_allow",
		Policy: `package test
			import rego.v1
			default allow = false
			allow if {
				input.user == "admin"
			}`,
		Input: map[string]interface{}{
			"user": "admin",
		},
	})

	// Add complex test with data
	suite.Add(Test{
		Name: "complex_authorization",
		Policy: `package authz
			import rego.v1
			default allow = false
			
			allow if {
				input.user.role == "admin"
			}
			
			allow if {
				input.user.id == input.resource.owner
				input.action in data.permissions[input.user.role]
			}`,
		Input: map[string]interface{}{
			"user": map[string]interface{}{
				"id":   "user123",
				"role": "editor",
			},
			"resource": map[string]interface{}{
				"owner": "user123",
			},
			"action": "read",
		},
		Data: map[string]interface{}{
			"permissions": map[string]interface{}{
				"editor": []string{"read", "write"},
				"viewer": []string{"read"},
			},
		},
	})

	ctx := context.Background()

	// Test regular target
	t.Run("regular", func(t *testing.T) {
		results, err := suite.Run(ctx, 5, "rego")
		if err != nil {
			t.Fatalf("benchmark failed: %v", err)
		}

		for _, r := range results {
			if r.AvgTime <= 0 {
				t.Errorf("invalid average time for %s: %v", r.Name, r.AvgTime)
			}
			if r.MinTime > r.MaxTime {
				t.Errorf("min time > max time for %s", r.Name)
			}
		}
	})

	// Test WASM target
	t.Run("wasm", func(t *testing.T) {
		results, err := suite.Run(ctx, 5, "wasm")
		if err != nil {
			// Skip if WASM engine not available
			if strings.Contains(err.Error(), "engine not found") {
				t.Skip("WASM engine not available, skipping test")
			}
			t.Fatalf("benchmark failed: %v", err)
		}

		for _, r := range results {
			if r.AvgTime <= 0 {
				t.Errorf("invalid average time for %s: %v", r.Name, r.AvgTime)
			}
		}
	})
}

func TestCompare(t *testing.T) {
	baseline := []Result{
		{
			Name:    "test1",
			AvgTime: 100 * time.Millisecond,
		},
		{
			Name:    "test2",
			AvgTime: 200 * time.Millisecond,
		},
	}

	current := []Result{
		{
			Name:    "test1",
			AvgTime: 105 * time.Millisecond, // 5% regression
		},
		{
			Name:    "test2",
			AvgTime: 180 * time.Millisecond, // 10% improvement
		},
	}

	report := Compare(baseline, current)

	if report == "" {
		t.Error("expected non-empty comparison report")
	}

	// Verify report contains expected elements
	expectedStrings := []string{
		"Benchmark Comparison Report",
		"test1",
		"test2",
		"+5.0%",
		"-10.0%",
	}

	for _, expected := range expectedStrings {
		if !contains(report, expected) {
			t.Errorf("report missing expected string: %s", expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}

// Benchmark functions for standard Go benchmarking
func BenchmarkSimplePolicy(b *testing.B) {
	ctx := context.Background()
	test := Test{
		Name: "simple",
		Policy: `package test
			import rego.v1
			allow if { input.x == 1 }`,
		Input: map[string]interface{}{"x": 1},
	}

	b.Run("rego", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := runBenchmark(ctx, test, 1, "rego")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("wasm", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := runBenchmark(ctx, test, 1, "wasm")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

