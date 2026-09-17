// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package test

import (
	v1 "github.com/open-policy-agent/opa/v1/logging/test"
)

// LogEntry represents a log message.
type LogEntry = v1.LogEntry

// Logger implementation that buffers messages for test purposes.
type Logger = v1.Logger

// New instantiates new Logger.
func New() *Logger {
	return v1.New()
}
