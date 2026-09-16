// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package logs

import (
	v1 "github.com/open-policy-agent/opa/v1/test/e2e/logs"
)

// GeneratePolicy generates a policy for use in Decision Log e2e tests. The
// `ruleCounts` determine how many total rules to generate, and the `ruleHits`
// are the number of them that will be evaluated. This is keyed off of
// the `input.hit` boolean value.
func GeneratePolicy(ruleCounts int, ruleHits int) string {
	return v1.GeneratePolicy(ruleCounts, ruleHits)
}

// TestLogServer implements the decision log endpoint for e2e testing.
type TestLogServer = v1.TestLogServer
