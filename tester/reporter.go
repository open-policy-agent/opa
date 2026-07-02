// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package tester

import (
	v1 "github.com/open-policy-agent/opa/v1/tester"
)

// Reporter defines the interface for reporting test results.
type Reporter = v1.Reporter

// PrettyReporter reports test results in a simple human readable format.
type PrettyReporter = v1.PrettyReporter

// JSONReporter reports test results as array of JSON objects.
type JSONReporter = v1.JSONReporter

// JSONCoverageReporter reports coverage as a JSON structure.
type JSONCoverageReporter = v1.JSONCoverageReporter
