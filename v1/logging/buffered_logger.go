package logging

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"sync/atomic"
	"time"
)

type logEntry struct {
	level   Level
	message string
	fields  map[string]any
	time    time.Time
}

// BufferedLogger captures log entries in memory until a target logger is set,
// at which point it replays all buffered entries to the target.
type BufferedLogger struct {
	mu           sync.Mutex
	buffer       []logEntry
	maxEntries   int
	target       atomic.Pointer[Logger]
	setOnce      sync.Once
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
	if target := b.target.Load(); target != nil {
		return level <= (*target).GetLevel()
	}
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

func (_ *BufferedLogger) logToTarget(target Logger, entry logEntry) {
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

	if target := b.target.Load(); target != nil {
		(*target).Debug(format, args...)
	} else {
		b.addToBuffer(Debug, format, args, nil)
	}
}

func (b *BufferedLogger) Info(format string, args ...any) {
	if !b.shouldLog(Info) {
		return
	}

	if target := b.target.Load(); target != nil {
		(*target).Info(format, args...)
	} else {
		b.addToBuffer(Info, format, args, nil)
	}
}

func (b *BufferedLogger) Warn(format string, args ...any) {
	if !b.shouldLog(Warn) {
		return
	}

	if target := b.target.Load(); target != nil {
		(*target).Warn(format, args...)
	} else {
		b.addToBuffer(Warn, format, args, nil)
	}
}

func (b *BufferedLogger) Error(format string, args ...any) {
	if !b.shouldLog(Error) {
		return
	}

	if target := b.target.Load(); target != nil {
		(*target).Error(format, args...)
	} else {
		b.addToBuffer(Error, format, args, nil)
	}
}

// WithFields returns a new logger with additional fields.
func (b *BufferedLogger) WithFields(fields map[string]any) Logger {
	if target := b.target.Load(); target != nil {
		return (*target).WithFields(fields)
	}

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

	if target := b.target.Load(); target != nil {
		return (*target).WithContext(ctx)
	}

	return b
}

// GetLevel returns the current log level.
func (b *BufferedLogger) GetLevel() Level {
	if target := b.target.Load(); target != nil {
		return (*target).GetLevel()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentLevel
}

// SetLevel sets the log level.
func (b *BufferedLogger) SetLevel(level Level) {
	if target := b.target.Load(); target != nil {
		(*target).SetLevel(level)
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.currentLevel = level
}

// SetTarget sets the target logger and flushes all buffered entries to it.
// This method should only be called once.
func (b *BufferedLogger) SetTarget(targetLogger Logger) {
	if targetLogger == nil {
		return
	}

	b.setOnce.Do(func() {
		b.mu.Lock()
		targetLogger.SetLevel(b.currentLevel)
		entries := b.buffer
		b.buffer = nil
		b.mu.Unlock()

		b.target.Store(&targetLogger)

		for _, entry := range entries {
			b.logToTarget(targetLogger, entry)
		}
	})
}

type bufferedLoggerWithFields struct {
	parent *BufferedLogger
	fields map[string]any
}

func (b *bufferedLoggerWithFields) Debug(format string, args ...any) {
	if !b.parent.shouldLog(Debug) {
		return
	}

	if target := b.parent.target.Load(); target != nil {
		(*target).WithFields(b.fields).Debug(format, args...)
	} else {
		b.parent.addToBuffer(Debug, format, args, b.fields)
	}
}

func (b *bufferedLoggerWithFields) Info(format string, args ...any) {
	if !b.parent.shouldLog(Info) {
		return
	}

	if target := b.parent.target.Load(); target != nil {
		(*target).WithFields(b.fields).Info(format, args...)
	} else {
		b.parent.addToBuffer(Info, format, args, b.fields)
	}
}

func (b *bufferedLoggerWithFields) Warn(format string, args ...any) {
	if !b.parent.shouldLog(Warn) {
		return
	}

	if target := b.parent.target.Load(); target != nil {
		(*target).WithFields(b.fields).Warn(format, args...)
	} else {
		b.parent.addToBuffer(Warn, format, args, b.fields)
	}
}

func (b *bufferedLoggerWithFields) Error(format string, args ...any) {
	if !b.parent.shouldLog(Error) {
		return
	}

	if target := b.parent.target.Load(); target != nil {
		(*target).WithFields(b.fields).Error(format, args...)
	} else {
		b.parent.addToBuffer(Error, format, args, b.fields)
	}
}

func (b *bufferedLoggerWithFields) WithFields(fields map[string]any) Logger {
	merged := make(map[string]any, len(b.fields)+len(fields))
	for k, v := range b.fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
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
