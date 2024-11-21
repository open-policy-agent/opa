// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/v1/topdown"
)

type EventType string

const (
	ExceptionEventType  = "exception"
	StdoutEventType     = "stdout"
	StoppedEventType    = "stopped"
	TerminatedEventType = "terminated"
	ThreadEventType     = "thread"
)

type Event struct {
	Type       EventType
	Thread     ThreadID
	Message    string
	stackIndex int
	stackEvent *topdown.Event
}

func (d Event) String() string {
	buf := new(strings.Builder)

	buf.WriteString(fmt.Sprintf("%s{", d.Type))
	buf.WriteString(fmt.Sprintf("thread=%d", d.Thread))

	if d.Message != "" {
		buf.WriteString(fmt.Sprintf(", message=%q", d.Message))
	}

	if d.stackEvent != nil {
		buf.WriteString(fmt.Sprintf(", stackIndex=%d", d.stackIndex))
	}

	buf.WriteString("}")

	return buf.String()
}

type EventHandler func(Event)

func newNopEventHandler() EventHandler {
	return func(_ Event) {}
}
