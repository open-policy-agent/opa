// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package debug
// EXPERIMENTAL: This package is under active development and is subject to change.
package debug

import (
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/rego"
	v1 "github.com/open-policy-agent/opa/v1/debug"
)

// Debugger is the interface for launching OPA debugger Session(s).
// This implementation is similar in structure to the Debug Adapter Protocol (DAP)
// to make such integrations easier, but is not intended to be a direct implementation.
// See: https://microsoft.github.io/debug-adapter-protocol/specification
//
// EXPERIMENTAL: These interfaces are under active development and is subject to change.
type Debugger = v1.Debugger

type Session = v1.Session

type DebuggerOption = v1.DebuggerOption

func NewDebugger(options ...DebuggerOption) Debugger {
	return v1.NewDebugger(options...)
}

func SetLogger(logger logging.Logger) DebuggerOption {
	return v1.SetLogger(logger)
}

func SetEventHandler(handler EventHandler) DebuggerOption {
	return v1.SetEventHandler(handler)
}

type LaunchEvalProperties = v1.LaunchEvalProperties

type LaunchTestProperties = v1.LaunchTestProperties

type LaunchProperties = v1.LaunchProperties

type LaunchOption = v1.LaunchOption

// RegoOption adds a rego option to the internal Rego instance.
// Options may be overridden by the debugger, and it is recommended to
// use LaunchEvalProperties for commonly used options.
func RegoOption(opt func(*rego.Rego)) LaunchOption {
	return v1.RegoOption(opt)
}
