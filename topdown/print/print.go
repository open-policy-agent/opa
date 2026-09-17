// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package print

import (
	v1 "github.com/open-policy-agent/opa/v1/topdown/print"
)

// Context provides the Hook implementation context about the print() call.
type Context = v1.Context

// Hook defines the interface that callers can implement to receive print
// statement outputs. If the hook returns an error, it will be surfaced if
// strict builtin error checking is enabled (otherwise, it will not halt
// execution.)
type Hook = v1.Hook
