// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package authz contains unit and benchmark tests for authz use-cases
// The public (non-test) APIs are meant to be used as helpers for
// other tests to build off of.
package authz

import (
	v1 "github.com/open-policy-agent/opa/v1/test/authz"
)

// Policy is a test rego policy for a token based authz system
const Policy = v1.Policy

// AllowQuery is the test query that goes with the Policy
// defined in this package
const AllowQuery = v1.AllowQuery

// DataSetProfile defines how the test data should be generated
type DataSetProfile = v1.DataSetProfile

// InputMode defines what type of inputs to generate for testings
type InputMode = v1.InputMode

// InputMode types supported by GenerateInput
const (
	ForbidIdentity = v1.ForbidIdentity
	ForbidPath     = v1.ForbidPath
	ForbidMethod   = v1.ForbidMethod
	Allow          = v1.Allow
)

// GenerateInput will use a dataset profile and desired InputMode to generate inputs for testing
func GenerateInput(profile DataSetProfile, mode InputMode) (interface{}, interface{}) {
	return v1.GenerateInput(profile, mode)
}

// GenerateDataset will generate a dataset for the given DatasetProfile
func GenerateDataset(profile DataSetProfile) map[string]interface{} {
	return v1.GenerateDataset(profile)
}
