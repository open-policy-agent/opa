// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	v1 "github.com/open-policy-agent/opa/v1/debug"
)

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
