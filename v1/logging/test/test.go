package test

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/v1/logging"
)

// LogEntry represents a log message.
type LogEntry struct {
	Level   logging.Level
	Fields  map[string]any
	Message string
	Time    time.Time
}

// Logger implementation that buffers messages for test purposes.
type Logger struct {
	level   logging.Level
	fields  map[string]any
	entries *[]LogEntry
	mtx     *sync.RWMutex
}

// New instantiates new Logger.
func New() *Logger {
	return &Logger{
		level:   logging.Info,
		entries: &[]LogEntry{},
		mtx:     &sync.RWMutex{},
	}
}

// WithFields provides additional fields to include in log output.
// Implemented here primarily to be able to switch between implementations without loss of data.
func (l *Logger) WithFields(fields map[string]any) logging.Logger {
	l.mtx.RLock()
	defer l.mtx.RUnlock()
	cp := Logger{
		level:   l.level,
		entries: l.entries,
		fields:  l.fields,
		mtx:     l.mtx,
	}
	flds := make(map[string]any)
	maps.Copy(flds, cp.fields)
	maps.Copy(flds, fields)
	cp.fields = flds
	return &cp
}

// WithContext returns the logger unchanged (test logger doesn't use context).
func (l *Logger) WithContext(context.Context) logging.Logger {
	return l
}

// Debug buffers a log message.
func (l *Logger) Debug(f string, a ...any) {
	l.append(logging.Debug, f, a...)
}

// Info buffers a log message.
func (l *Logger) Info(f string, a ...any) {
	l.append(logging.Info, f, a...)
}

// Error buffers a log message.
func (l *Logger) Error(f string, a ...any) {
	l.append(logging.Error, f, a...)
}

// Warn buffers a log message.
func (l *Logger) Warn(f string, a ...any) {
	l.append(logging.Warn, f, a...)
}

// SetLevel set log level.
func (l *Logger) SetLevel(level logging.Level) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.level = level
}

// GetLevel get log level.
func (l *Logger) GetLevel() logging.Level {
	l.mtx.RLock()
	defer l.mtx.RUnlock()
	return l.level
}

// Entries returns buffered log entries.
func (l *Logger) Entries() []LogEntry {
	l.mtx.RLock()
	defer l.mtx.RUnlock()
	result := make([]LogEntry, len(*l.entries))
	copy(result, *l.entries)
	return result
}

func (l *Logger) append(lvl logging.Level, f string, a ...any) {
	l.mtx.RLock()
	currentLevel := l.level
	l.mtx.RUnlock()

	if lvl > currentLevel {
		return
	}

	l.mtx.Lock()
	defer l.mtx.Unlock()

	// Extract time from fields if present, otherwise use now
	var timestamp time.Time
	if l.fields != nil {
		if t, ok := l.fields["time"]; ok {
			if tt, ok := t.(time.Time); ok {
				timestamp = tt
			}
		}
	}

	*l.entries = append(*l.entries, LogEntry{
		Level:   lvl,
		Fields:  l.fields,
		Message: fmt.Sprintf(f, a...),
		Time:    timestamp,
	})
}
