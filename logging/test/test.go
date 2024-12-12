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
