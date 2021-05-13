package test

import (
	"fmt"
	"sync"

	"github.com/open-policy-agent/opa/logging"
)

// LogEntry represents a log message.
type LogEntry struct {
	Level   logging.Level
	Fields  map[string]interface{}
	Message string
}

// Logger implementation that buffers messages for test purposes.
type Logger struct {
	level   logging.Level
	fields  map[string]interface{}
	entries *[]LogEntry
	mtx     sync.Mutex
}

// New instantiates new Logger.
func New() *Logger {
	return &Logger{
		level:   logging.Info,
		entries: &[]LogEntry{},
	}
}

// WithFields provides additional fields to include in log output.
// Implemented here primarily to be able to switch between implementations without loss of data.
func (l *Logger) WithFields(fields map[string]interface{}) logging.Logger {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	cp := Logger{
		level:   l.level,
		entries: l.entries,
		fields:  l.fields,
	}
	flds := make(map[string]interface{})
	for k, v := range cp.fields {
		flds[k] = v
	}
	for k, v := range fields {
		flds[k] = v
	}
	cp.fields = flds
	return &cp
}

// GetFields returns additional fields of this logger
// Implemented here primarily to be able to switch between implementations without loss of data.
func (l *Logger) GetFields() map[string]interface{} {
	return l.fields
}

// Debug buffers a log message.
func (l *Logger) Debug(f string, a ...interface{}) {
	l.append(logging.Debug, f, a...)
}

// Info buffers a log message.
func (l *Logger) Info(f string, a ...interface{}) {
	l.append(logging.Info, f, a...)
}

// Error buffers a log message.
func (l *Logger) Error(f string, a ...interface{}) {
	l.append(logging.Error, f, a...)
}

// Warn buffers a log message.
func (l *Logger) Warn(f string, a ...interface{}) {
	l.append(logging.Warn, f, a...)
}

// SetLevel set log level.
func (l *Logger) SetLevel(level logging.Level) {
	l.level = level
}

// GetLevel get log level.
func (l *Logger) GetLevel() logging.Level {
	return l.level
}

// Entries returns buffered log entries.
func (l *Logger) Entries() []LogEntry {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	return *l.entries
}

func (l *Logger) append(lvl logging.Level, f string, a ...interface{}) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	*l.entries = append(*l.entries, LogEntry{
		Level:   lvl,
		Fields:  l.fields,
		Message: fmt.Sprintf(f, a...),
	})
}
