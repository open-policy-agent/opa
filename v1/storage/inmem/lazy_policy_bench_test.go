// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package inmem

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/storage"
)

// generatePolicyData generates realistic Rego policy data of specified size
func generatePolicyData(size int) []byte {
	// Generate realistic Rego policy with repeated patterns (compresses well)
	var sb strings.Builder
	sb.WriteString("package test.policy\n\nimport rego.v1\n\n")

	// Add rules until we reach desired size
	ruleTemplate := `
allow if {
	input.user.role == "admin"
	input.action == "read"
	input.resource.type == "document"
}

deny if {
	not allow
	input.user.authenticated == false
}

`
	for sb.Len() < size {
		sb.WriteString(ruleTemplate)
	}

	result := sb.String()
	if len(result) > size {
		result = result[:size]
	}
	return []byte(result)
}

// Benchmark UpsertPolicy operation (compression overhead on write)
func BenchmarkUpsertPolicy(b *testing.B) {
	sizes := []int{512, 2048, 8192, 32768}

	for _, size := range sizes {
		policyData := generatePolicyData(size)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			st := New()
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := b.N; b.Loop(); i++ {
				txn, err := st.NewTransaction(ctx, storage.WriteParams)
				if err != nil {
					b.Fatal(err)
				}

				id := fmt.Sprintf("policy-%d", i%100)
				if err := st.UpsertPolicy(ctx, txn, id, policyData); err != nil {
					b.Fatal(err)
				}

				if err := st.Commit(ctx, txn); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Benchmark GetPolicy operation (decompression overhead on read)
func BenchmarkGetPolicy(b *testing.B) {
	sizes := []int{512, 2048, 8192, 32768}

	for _, size := range sizes {
		policyData := generatePolicyData(size)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			st := New()
			ctx := context.Background()

			// Setup: insert policies
			txn, _ := st.NewTransaction(ctx, storage.WriteParams)
			for i := range 10 {
				id := fmt.Sprintf("policy-%d", i)
				if err := st.UpsertPolicy(ctx, txn, id, policyData); err != nil {
					b.Fatal(err)
				}
			}
			if err := st.Commit(ctx, txn); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := b.N; b.Loop(); i++ {
				txn, err := st.NewTransaction(ctx)
				if err != nil {
					b.Fatal(err)
				}

				id := fmt.Sprintf("policy-%d", i%10)
				_, err = st.GetPolicy(ctx, txn, id)
				if err != nil {
					b.Fatal(err)
				}

				st.Abort(ctx, txn)
			}
		})
	}
}

// Benchmark GetPolicy with cache hits (no decompression after first read)
func BenchmarkGetPolicyCached(b *testing.B) {
	sizes := []int{512, 2048, 8192, 32768}

	for _, size := range sizes {
		policyData := generatePolicyData(size)

		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			st := New()
			ctx := context.Background()

			// Setup: insert and warm cache
			txn, _ := st.NewTransaction(ctx, storage.WriteParams)
			if err := st.UpsertPolicy(ctx, txn, "policy-1", policyData); err != nil {
				b.Fatal(err)
			}
			if err := st.Commit(ctx, txn); err != nil {
				b.Fatal(err)
			}

			// Warm cache
			txn, _ = st.NewTransaction(ctx)
			if _, err := st.GetPolicy(ctx, txn, "policy-1"); err != nil {
				b.Fatal(err)
			}
			st.Abort(ctx, txn)

			b.ResetTimer()
			b.ReportAllocs()

			for range b.N {
				txn, err := st.NewTransaction(ctx)
				if err != nil {
					b.Fatal(err)
				}

				_, err = st.GetPolicy(ctx, txn, "policy-1")
				if err != nil {
					b.Fatal(err)
				}

				st.Abort(ctx, txn)
			}
		})
	}
}

// Benchmark ListPolicies operation
func BenchmarkListPolicies(b *testing.B) {
	policyCounts := []int{10, 100, 1000}
	policySize := 2048

	for _, count := range policyCounts {
		policyData := generatePolicyData(policySize)

		b.Run(fmt.Sprintf("count=%d", count), func(b *testing.B) {
			st := New()
			ctx := context.Background()

			// Setup: insert policies
			txn, _ := st.NewTransaction(ctx, storage.WriteParams)
			for i := range count {
				id := fmt.Sprintf("policy-%d", i)
				if err := st.UpsertPolicy(ctx, txn, id, policyData); err != nil {
					b.Fatal(err)
				}
			}
			if err := st.Commit(ctx, txn); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				txn, err := st.NewTransaction(ctx)
				if err != nil {
					b.Fatal(err)
				}

				_, err = st.ListPolicies(ctx, txn)
				if err != nil {
					b.Fatal(err)
				}

				st.Abort(ctx, txn)
			}
		})
	}
}

// Benchmark concurrent access patterns
func BenchmarkConcurrentGetPolicy(b *testing.B) {
	policyData := generatePolicyData(2048)
	st := New()
	ctx := context.Background()

	// Setup: insert policies
	txn, _ := st.NewTransaction(ctx, storage.WriteParams)
	for i := range 100 {
		id := fmt.Sprintf("policy-%d", i)
		if err := st.UpsertPolicy(ctx, txn, id, policyData); err != nil {
			b.Fatal(err)
		}
	}
	if err := st.Commit(ctx, txn); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			txn, err := st.NewTransaction(ctx)
			if err != nil {
				b.Fatal(err)
			}

			id := fmt.Sprintf("policy-%d", i%100)
			_, err = st.GetPolicy(ctx, txn, id)
			if err != nil {
				b.Fatal(err)
			}

			st.Abort(ctx, txn)
			i++
		}
	})
}

// Benchmark mixed read/write workload (80% reads, 20% writes)
func BenchmarkMixedWorkload(b *testing.B) {
	policyData := generatePolicyData(2048)
	st := New()
	ctx := context.Background()

	// Pre-populate
	txn, _ := st.NewTransaction(ctx, storage.WriteParams)
	for i := range 50 {
		id := fmt.Sprintf("policy-%d", i)
		if err := st.UpsertPolicy(ctx, txn, id, policyData); err != nil {
			b.Fatal(err)
		}
	}
	if err := st.Commit(ctx, txn); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		if i%5 == 0 {
			// Write (20%)
			txn, _ := st.NewTransaction(ctx, storage.WriteParams)
			id := fmt.Sprintf("policy-%d", i%100)
			if err := st.UpsertPolicy(ctx, txn, id, policyData); err != nil {
				b.Fatal(err)
			}
			if err := st.Commit(ctx, txn); err != nil {
				b.Fatal(err)
			}
		} else {
			// Read (80%)
			txn, _ := st.NewTransaction(ctx)
			id := fmt.Sprintf("policy-%d", i%100)
			if _, err := st.GetPolicy(ctx, txn, id); err != nil {
				b.Fatal(err)
			}
			st.Abort(ctx, txn)
		}
	}
}
