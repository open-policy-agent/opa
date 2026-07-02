// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package cases contains utilities for evaluation test cases.
package cases

import (
	v1 "github.com/open-policy-agent/opa/v1/test/cases"
)

// Set represents a collection of test cases.
type Set = v1.Set

// TestCase represents a single test case.
type TestCase = v1.TestCase

// Load returns a set of built-in test cases.
func Load(path string) (Set, error) {
	return v1.Load(path)
}

// MustLoad returns a set of built-in test cases or panics if an error occurs.
func MustLoad(path string) Set {
	return v1.MustLoad(path)
}
