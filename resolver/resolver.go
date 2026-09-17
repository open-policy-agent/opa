// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package resolver

import (
	v1 "github.com/open-policy-agent/opa/v1/resolver"
)

// Resolver defines an external value resolver for OPA evaluations.
type Resolver = v1.Resolver

// Input as provided to a Resolver instance when evaluating.
type Input = v1.Input

// Result of resolving a ref.
type Result = v1.Result
