// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

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
