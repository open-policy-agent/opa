package logging

import (
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
// After Flush, subsequent log calls are forwarded to the flush target.
// This ensures that code holding a cached reference to the BufferedLogger
// (or a WithFields derivative) continues to work correctly.
type BufferedLogger struct {
	mu           sync.Mutex
	buffer       []logEntry
	maxEntries   int
	currentLevel Level
	target       Logger
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

func (b *BufferedLogger) addToBuffer(level Level, format string, args []any, fields map[string]any) {
	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}

	b.mu.Lock()
	if b.target != nil {
		target := b.target
		b.mu.Unlock()

		logger := target
		if len(fields) > 0 {
			logger = target.WithFields(fields)
		}
		switch level {
		case Debug:
			logger.Debug("%s", message)
		case Info:
			logger.Info("%s", message)
		case Warn:
			logger.Warn("%s", message)
		case Error:
			logger.Error("%s", message)
		}
		return
	}

	if len(b.buffer) >= b.maxEntries {
		b.buffer = b.buffer[1:]
	}
	b.buffer = append(b.buffer, logEntry{
		level:   level,
		message: message,
		fields:  fields,
		time:    time.Now(),
	})
	b.mu.Unlock()
}

func (*BufferedLogger) logToTarget(target Logger, entry logEntry) {
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
	b.addToBuffer(Debug, format, args, nil)
}

func (b *BufferedLogger) Info(format string, args ...any) {
	b.addToBuffer(Info, format, args, nil)
}

func (b *BufferedLogger) Warn(format string, args ...any) {
	b.addToBuffer(Warn, format, args, nil)
}

func (b *BufferedLogger) Error(format string, args ...any) {
	b.addToBuffer(Error, format, args, nil)
}

// WithFields returns a new logger with additional fields.
func (b *BufferedLogger) WithFields(fields map[string]any) Logger {
	return &bufferedLoggerWithFields{
		parent: b,
		fields: fields,
	}
}

// GetLevel returns the current log level.
func (b *BufferedLogger) GetLevel() Level {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.target != nil {
		return b.target.GetLevel()
	}
	return b.currentLevel
}

// SetLevel sets the log level.
func (b *BufferedLogger) SetLevel(level Level) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.currentLevel = level
	if b.target != nil {
		b.target.SetLevel(level)
	}
}

// Close discards all buffered entries without flushing them.
// After calling Close, the BufferedLogger should not be used anymore.
func (b *BufferedLogger) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buffer = nil
}

// Flush replays all buffered entries to the target logger.
// After Flush, subsequent log calls on this BufferedLogger (and any
// previously obtained WithFields loggers) are forwarded to the target.
func (b *BufferedLogger) Flush(targetLogger Logger) {
	if targetLogger == nil {
		return
	}

	b.mu.Lock()
	b.target = targetLogger
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
	b.parent.addToBuffer(Debug, format, args, b.fields)
}

func (b *bufferedLoggerWithFields) Info(format string, args ...any) {
	b.parent.addToBuffer(Info, format, args, b.fields)
}

func (b *bufferedLoggerWithFields) Warn(format string, args ...any) {
	b.parent.addToBuffer(Warn, format, args, b.fields)
}

func (b *bufferedLoggerWithFields) Error(format string, args ...any) {
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

func (b *bufferedLoggerWithFields) GetLevel() Level {
	return b.parent.GetLevel()
}

func (b *bufferedLoggerWithFields) SetLevel(level Level) {
	b.parent.SetLevel(level)
}
