// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package debug
// EXPERIMENTAL: This package is under active development and is subject to change.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package debug

import (
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/rego"
	v1 "github.com/open-policy-agent/opa/v1/debug"
)

type BreakpointID = v1.BreakpointID

type Breakpoint = v1.Breakpoint

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

// SetMaxVariableLength sets the maximum length of variable values displayed in the debugger.
// Values longer than this limit will be truncated. A value of 0 disables truncation.
// If not set, the default is 100 characters.
func SetMaxVariableLength(maxVariableLength int) DebuggerOption {
	return v1.SetMaxVariableLength(maxVariableLength)
}

type LaunchEvalProperties = v1.LaunchEvalProperties

type LaunchTestProperties = v1.LaunchTestProperties

type LaunchProperties = v1.LaunchProperties

// StackTraceMode determines how the events of a trace are grouped into the frames
// of a StackTrace.
type StackTraceMode = v1.StackTraceMode

const (
	// StackTraceModeDefault is the zero value of StackTraceMode, and is equivalent to
	// StackTraceModeQuery.
	StackTraceModeDefault = v1.StackTraceModeDefault

	// StackTraceModeQuery frames the stack trace by query, where each frame represents
	// a query scope. This is the default.
	StackTraceModeQuery = v1.StackTraceModeQuery

	// StackTraceModeEvent frames the stack trace by trace event, where each frame
	// represents a single event consumed by the debugger.
	StackTraceModeEvent = v1.StackTraceModeEvent
)

type LaunchOption = v1.LaunchOption

// RegoOption adds a rego option to the internal Rego instance.
// Options may be overridden by the debugger, and it is recommended to
// use LaunchEvalProperties for commonly used options.
func RegoOption(opt func(*rego.Rego)) LaunchOption {
	return v1.RegoOption(opt)
}

type EventType = v1.EventType

const (
	ExceptionEventType  = v1.ExceptionEventType
	StdoutEventType     = v1.StdoutEventType
	StoppedEventType    = v1.StoppedEventType
	TerminatedEventType = v1.TerminatedEventType
	ThreadEventType     = v1.ThreadEventType
)

type Event = v1.Event

type EventHandler = v1.EventHandler

type FrameID = v1.FrameID

type StackFrame = v1.StackFrame

// StackTrace represents a StackFrame stack.
type StackTrace = v1.StackTrace

type ThreadID = v1.ThreadID

// Thread represents a single thread of execution.
type Thread = v1.Thread

// Scope represents the variable state of a StackFrame.
type Scope = v1.Scope

type Variable = v1.Variable

type VarRef = v1.VarRef
