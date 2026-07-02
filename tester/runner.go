// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package tester contains utilities for executing Rego tests.
package tester

import (
	"context"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/storage"
	v1 "github.com/open-policy-agent/opa/v1/tester"
)

// TestPrefix declares the prefix for all test rules.
const TestPrefix = v1.TestPrefix

// SkipTestPrefix declares the prefix for tests that should be skipped.
const SkipTestPrefix = v1.SkipTestPrefix

// Run executes all test cases found under files in path.
func Run(ctx context.Context, paths ...string) ([]*Result, error) {
	return v1.Run(ctx, paths...)
}

// RunWithFilter executes all test cases found under files in path. The filter
// will be applied to exclude files that should not be included.
func RunWithFilter(ctx context.Context, _ loader.Filter, paths ...string) ([]*Result, error) {
	return v1.Run(ctx, paths...)
}

// Result represents a single test case result.
type Result = v1.Result

// BenchmarkOptions defines options specific to benchmarking tests
type BenchmarkOptions = v1.BenchmarkOptions

// Runner implements simple test discovery and execution.
type Runner = v1.Runner

// NewRunner returns a new runner.
func NewRunner() *Runner {
	return v1.NewRunner().SetDefaultRegoVersion(ast.DefaultRegoVersion)
}

type Builtin = v1.Builtin

// Load returns modules and an in-memory store for running tests.
func Load(args []string, filter loader.Filter) (map[string]*ast.Module, storage.Store, error) {
	return LoadWithRegoVersion(args, filter, ast.DefaultRegoVersion)
}

// LoadWithRegoVersion returns modules and an in-memory store for running tests.
// Modules are parsed in accordance with the given RegoVersion.
func LoadWithRegoVersion(args []string, filter loader.Filter, regoVersion ast.RegoVersion) (map[string]*ast.Module, storage.Store, error) {
	return v1.LoadWithRegoVersion(args, filter, regoVersion)
}

// LoadBundles will load the given args as bundles, either tarball or directory is OK.
func LoadBundles(args []string, filter loader.Filter) (map[string]*bundle.Bundle, error) {
	return LoadBundlesWithRegoVersion(args, filter, ast.DefaultRegoVersion)
}

// LoadBundlesWithRegoVersion will load the given args as bundles, either tarball or directory is OK.
// Bundles are parsed in accordance with the given RegoVersion.
func LoadBundlesWithRegoVersion(args []string, filter loader.Filter, regoVersion ast.RegoVersion) (map[string]*bundle.Bundle, error) {
	return v1.LoadBundlesWithRegoVersion(args, filter, regoVersion)
}
