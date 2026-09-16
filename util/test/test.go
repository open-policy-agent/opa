// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package test contains utilities used in the policy engine's test suite.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package test

import (
	"io/fs"
	"testing"
	"time"

	v1 "github.com/open-policy-agent/opa/v1/util/test"
)

// This file collects some helpers for generating data used in
// benchmarks,
// - topdown/topdown_bench_test.go

// PartialObjectBenchmarkCrossModule returns a module with n "bench_test_" prefixed rules
// that each refer to another "cond_bench_" prefixed rule
func PartialObjectBenchmarkCrossModule(n int) []string {
	return v1.PartialObjectBenchmarkCrossModule(n)
}

// ArrayIterationBenchmarkModule returns a module that iterates an array
// with `n` elements
func ArrayIterationBenchmarkModule(n int) string {
	return v1.ArrayIterationBenchmarkModule(n)
}

// SetIterationBenchmarkModule returns a module that iterates a set
// with `n` elements
func SetIterationBenchmarkModule(n int) string {
	return v1.SetIterationBenchmarkModule(n)
}

// ObjectIterationBenchmarkModule returns a module that iterates an object
// with `n` key/val pairs
func ObjectIterationBenchmarkModule(n int) string {
	return v1.ObjectIterationBenchmarkModule(n)
}

// GenerateLargeJSONBenchmarkData returns a map of 100 keys and 100.000 key/value
// pairs.
func GenerateLargeJSONBenchmarkData() map[string]any {
	return v1.GenerateLargeJSONBenchmarkData()
}

// GenerateJSONBenchmarkData returns a map of `k` keys and `v` key/value pairs.
func GenerateJSONBenchmarkData(k, v int) map[string]any {
	return v1.GenerateJSONBenchmarkData(k, v)
}

// GenerateConcurrencyBenchmarkData returns a module and data; the module
// checks some input parameters against that data in a simple API authz
// scheme.
func GenerateConcurrencyBenchmarkData() (string, map[string]any) {
	return v1.GenerateConcurrencyBenchmarkData()
}

// GenerateVirtualDocsBenchmarkData generates a module and input; the
// numTotalRules and numHitRules create as many rules in the module to
// match/miss the returned input.
func GenerateVirtualDocsBenchmarkData(numTotalRules, numHitRules int) (string, map[string]any) {
	return v1.GenerateVirtualDocsBenchmarkData(numTotalRules, numHitRules)
}

// WithTempFS creates a temporary directory structure and invokes f with the
// root directory path.
func WithTempFS(files map[string]string, f func(string)) {
	v1.WithTempFS(files, f)
}

// MakeTempFS creates a temporary directory structure for test purposes rooted at root.
// If root is empty, the dir is created in the default system temp location.
// If the creation fails, cleanup is nil and the caller does not have to invoke it. If
// creation succeeds, the caller should invoke cleanup when they are done.
func MakeTempFS(root, prefix string, files map[string]string) (rootDir string, cleanup func(), err error) {
	return v1.MakeTempFS(root, prefix, files)
}

// WithTestFS creates a temporary file system of `files` in memory
// if `inMemoryFS` is true and invokes `f“ with that filesystem
func WithTestFS(files map[string]string, inMemoryFS bool, f func(string, fs.FS)) {
	v1.WithTestFS(files, inMemoryFS, f)
}

func Eventually(t *testing.T, timeout time.Duration, f func() bool) bool {
	t.Helper()
	return v1.Eventually(t, timeout, f)
}

func EventuallyOrFatal(t *testing.T, timeout time.Duration, f func() bool) {
	t.Helper()
	v1.EventuallyOrFatal(t, timeout, f)
}

type BlockingWriter = v1.BlockingWriter
