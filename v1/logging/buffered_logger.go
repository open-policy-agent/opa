package logging

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"
)

type logEntry struct {
	level   Level
	message string
	fields  map[string]any
	time    time.Time
}

// BufferedLogger captures log entries in memory until Flush is called,
// at which point it replays all buffered entries to the target logger.
// After Flush() is called, the BufferedLogger should not be used anymore.
type BufferedLogger struct {
	mu           sync.Mutex
	buffer       []logEntry
	maxEntries   int
	currentLevel Level
}

// NewBufferedLogger creates a new buffered logger that will buffer up to maxEntries.
func NewBufferedLogger(maxEntries int) *BufferedLogger {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &BufferedLogger{
		buffer:       make([]logEntry, 0, maxEntries),
		maxEntries:   maxEntries,
		currentLevel: Info,
	}
}

func (b *BufferedLogger) shouldLog(level Level) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return level <= b.currentLevel
}

func (b *BufferedLogger) addToBuffer(level Level, format string, args []any, fields map[string]any) {
	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}

	entry := logEntry{
		level:   level,
		message: message,
		fields:  fields,
		time:    time.Now(),
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.buffer) >= b.maxEntries {
		b.buffer = b.buffer[1:]
	}
	b.buffer = append(b.buffer, entry)
}

func (b *BufferedLogger) logToTarget(target Logger, entry logEntry) {
	fields := make(map[string]any, len(entry.fields))
	maps.Copy(fields, entry.fields)
	fields["time"] = entry.time

	logger := target.WithFields(fields)

	switch entry.level {
	case Debug:
		logger.Debug("%s", entry.message)
	case Info:
		logger.Info("%s", entry.message)
	case Warn:
		logger.Warn("%s", entry.message)
	case Error:
		logger.Error("%s", entry.message)
	}
}

func (b *BufferedLogger) Debug(format string, args ...any) {
	if !b.shouldLog(Debug) {
		return
	}
	b.addToBuffer(Debug, format, args, nil)
}

func (b *BufferedLogger) Info(format string, args ...any) {
	if !b.shouldLog(Info) {
		return
	}
	b.addToBuffer(Info, format, args, nil)
}

func (b *BufferedLogger) Warn(format string, args ...any) {
	if !b.shouldLog(Warn) {
		return
	}
	b.addToBuffer(Warn, format, args, nil)
}

func (b *BufferedLogger) Error(format string, args ...any) {
	if !b.shouldLog(Error) {
		return
	}
	b.addToBuffer(Error, format, args, nil)
}

// WithFields returns a new logger with additional fields.
func (b *BufferedLogger) WithFields(fields map[string]any) Logger {
	return &bufferedLoggerWithFields{
		parent: b,
		fields: fields,
	}
}

// WithContext returns a new logger with context.
func (b *BufferedLogger) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return b
	}
	return b
}

// GetLevel returns the current log level.
func (b *BufferedLogger) GetLevel() Level {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentLevel
}

// SetLevel sets the log level.
func (b *BufferedLogger) SetLevel(level Level) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.currentLevel = level
}

// Flush replays all buffered entries to the target logger.
// After calling Flush, the BufferedLogger should not be used anymore.
// The caller should switch to using the target logger directly.
func (b *BufferedLogger) Flush(targetLogger Logger) {
	if targetLogger == nil {
		return
	}

	b.mu.Lock()
	targetLogger.SetLevel(b.currentLevel)
	entries := b.buffer
	b.buffer = nil
	b.mu.Unlock()

	for _, entry := range entries {
		b.logToTarget(targetLogger, entry)
	}
}

type bufferedLoggerWithFields struct {
	parent *BufferedLogger
	fields map[string]any
}

func (b *bufferedLoggerWithFields) Debug(format string, args ...any) {
	if !b.parent.shouldLog(Debug) {
		return
	}
	b.parent.addToBuffer(Debug, format, args, b.fields)
}

func (b *bufferedLoggerWithFields) Info(format string, args ...any) {
	if !b.parent.shouldLog(Info) {
		return
	}
	b.parent.addToBuffer(Info, format, args, b.fields)
}

func (b *bufferedLoggerWithFields) Warn(format string, args ...any) {
	if !b.parent.shouldLog(Warn) {
		return
	}
	b.parent.addToBuffer(Warn, format, args, b.fields)
}

func (b *bufferedLoggerWithFields) Error(format string, args ...any) {
	if !b.parent.shouldLog(Error) {
		return
	}
	b.parent.addToBuffer(Error, format, args, b.fields)
}

func (b *bufferedLoggerWithFields) WithFields(fields map[string]any) Logger {
	merged := make(map[string]any, len(b.fields)+len(fields))
	maps.Copy(merged, b.fields)
	maps.Copy(merged, fields)
	return &bufferedLoggerWithFields{
		parent: b.parent,
		fields: merged,
	}
}

func (b *bufferedLoggerWithFields) WithContext(ctx context.Context) Logger {
	if ctx == nil {
		return b
	}
	return b
}

func (b *bufferedLoggerWithFields) GetLevel() Level {
	return b.parent.GetLevel()
}

func (b *bufferedLoggerWithFields) SetLevel(level Level) {
	b.parent.SetLevel(level)
}
