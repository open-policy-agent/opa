// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package json

import (
	v1 "github.com/open-policy-agent/opa/v1/ast/json"
)

// Options defines the options for JSON operations,
// currently only marshaling can be configured
type Options = v1.Options

// MarshalOptions defines the options for JSON marshaling,
// currently only toggling the marshaling of location information is supported
type MarshalOptions = v1.MarshalOptions

// NodeToggle is a generic struct to allow the toggling of
// settings for different ast node types
type NodeToggle = v1.NodeToggle
