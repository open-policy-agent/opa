// Copyright 2024 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package benchmark provides performance benchmarking utilities for OPA WASM.
package benchmark

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

// Result represents a single benchmark result
type Result struct {
	Name       string
	Iterations int
	TotalTime  time.Duration
	AvgTime    time.Duration
	MinTime    time.Duration
	MaxTime    time.Duration
	MemAllocs  uint64
	MemBytes   uint64
}

// Suite represents a collection of benchmark tests
type Suite struct {
	tests []Test
	mu    sync.Mutex
}

// Test represents a single benchmark test
type Test struct {
	Name    string
	Policy  string
	Input   interface{}
	Data    interface{}
	Modules []string
}

// NewSuite creates a new benchmark suite
func NewSuite() *Suite {
	return &Suite{
		tests: make([]Test, 0),
	}
}

// Add adds a test to the suite
func (s *Suite) Add(test Test) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tests = append(s.tests, test)
}

// Run executes all benchmark tests in the suite
func (s *Suite) Run(ctx context.Context, iterations int, target string) ([]Result, error) {
	results := make([]Result, 0, len(s.tests))

	for _, test := range s.tests {
		result, err := runBenchmark(ctx, test, iterations, target)
		if err != nil {
			return nil, fmt.Errorf("benchmark %s failed: %w", test.Name, err)
		}
		results = append(results, result)
	}

	return results, nil
}

// runBenchmark executes a single benchmark test
func runBenchmark(ctx context.Context, test Test, iterations int, target string) (Result, error) {
	result := Result{
		Name:       test.Name,
		Iterations: iterations,
		MinTime:    time.Duration(1<<63 - 1),
	}

	// Prepare the policy
	modules := make([]*ast.Module, 0, len(test.Modules)+1)

	// Add main policy
	m, err := ast.ParseModule(test.Name, test.Policy)
	if err != nil {
		return result, fmt.Errorf("parse policy: %w", err)
	}
	modules = append(modules, m)

	// Add additional modules
	for i, mod := range test.Modules {
		m, err := ast.ParseModule(fmt.Sprintf("module%d", i), mod)
		if err != nil {
			return result, fmt.Errorf("parse module %d: %w", i, err)
		}
		modules = append(modules, m)
	}

	// Compile once to ensure policy is valid
	compiler := ast.NewCompiler()
	moduleMap := make(map[string]*ast.Module)
	for _, m := range modules {
		moduleMap[m.Package.Path.String()] = m
	}
	compiler.Compile(moduleMap)
	if compiler.Failed() {
		return result, fmt.Errorf("compile failed: %v", compiler.Errors)
	}

	// Run benchmark iterations
	var totalDuration time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()

		// Create new Rego instance for each iteration
		opts := []func(*rego.Rego){
			rego.Module(test.Name, test.Policy),
			rego.Input(test.Input),
			rego.Query("data." + getPackageName(test.Policy)),
		}

		if test.Data != nil {
			dataMap, ok := test.Data.(map[string]interface{})
			if !ok {
				return result, fmt.Errorf("data must be a map[string]interface{}")
			}
			store := inmem.NewFromObject(dataMap)
			opts = append(opts, rego.Store(store))
		}

		if target == "wasm" {
			opts = append(opts, rego.Target("wasm"))
		}

		r := rego.New(opts...)

		// Prepare query
		pq, err := r.PrepareForEval(ctx)
		if err != nil {
			return result, fmt.Errorf("prepare query: %w", err)
		}

		// Execute query
		rs, err := pq.Eval(ctx)
		if err != nil {
			return result, fmt.Errorf("eval: %w", err)
		}

		// Ensure result is used to prevent optimization
		_ = rs

		duration := time.Since(start)
		totalDuration += duration

		if duration < result.MinTime {
			result.MinTime = duration
		}
		if duration > result.MaxTime {
			result.MaxTime = duration
		}
	}

	result.TotalTime = totalDuration
	result.AvgTime = totalDuration / time.Duration(iterations)

	return result, nil
}

// Compare compares two benchmark results
func Compare(baseline, current []Result) string {
	baselineMap := make(map[string]Result)
	for _, r := range baseline {
		baselineMap[r.Name] = r
	}

	var report string
	report += "Benchmark Comparison Report\n"
	report += "===========================\n\n"

	for _, curr := range current {
		if base, ok := baselineMap[curr.Name]; ok {
			diff := float64(curr.AvgTime-base.AvgTime) / float64(base.AvgTime) * 100
			status := "✓"
			if diff > 5 {
				status = "✗"
			}

			report += fmt.Sprintf("%s %s:\n", status, curr.Name)
			report += fmt.Sprintf("  Baseline: %v\n", base.AvgTime)
			report += fmt.Sprintf("  Current:  %v\n", curr.AvgTime)
			report += fmt.Sprintf("  Change:   %+.1f%%\n\n", diff)
		}
	}

	return report
}

// Hash returns a hash of the test configuration
func (t *Test) Hash() string {
	h := sha256.New()
	h.Write([]byte(t.Name))
	h.Write([]byte(t.Policy))
	h.Write([]byte(fmt.Sprintf("%v", t.Input)))
	h.Write([]byte(fmt.Sprintf("%v", t.Data)))
	for _, m := range t.Modules {
		h.Write([]byte(m))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// getPackageName extracts the package name from a policy
func getPackageName(policy string) string {
	m, err := ast.ParseModule("", policy)
	if err != nil {
		return "test"
	}
	return m.Package.Path.String()
}

