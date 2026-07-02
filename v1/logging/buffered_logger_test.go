package logging

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewBufferedLogger(t *testing.T) {
	logger := NewBufferedLogger(100)
	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}

	if logger.maxEntries != 100 {
		t.Errorf("Expected maxEntries=100, got %d", logger.maxEntries)
	}

	if logger.currentLevel != Info {
		t.Errorf("Expected default level=Info, got %v", logger.currentLevel)
	}

	defaultLogger := NewBufferedLogger(0)
	if defaultLogger.maxEntries != 1000 {
		t.Errorf("Expected default maxEntries=1000, got %d", defaultLogger.maxEntries)
	}
}

func TestBufferedLoggerBuffering(t *testing.T) {
	logger := NewBufferedLogger(100)

	// BufferedLogger records all messages regardless of level; the target
	// logger decides what to discard when Flush is called.
	logger.Info("test message 1")
	logger.Error("test message 2")
	logger.Debug("test message 3")

	logger.mu.Lock()
	count := len(logger.buffer)
	logger.mu.Unlock()

	if count != 3 {
		t.Errorf("Expected 3 buffered messages, got %d", count)
	}

	logger.SetLevel(Debug)
	if logger.GetLevel() != Debug {
		t.Errorf("Expected level=Debug, got %v", logger.GetLevel())
	}

	logger.Debug("debug message")
	logger.mu.Lock()
	count = len(logger.buffer)
	logger.mu.Unlock()

	if count != 4 {
		t.Errorf("Expected 4 buffered messages after level change, got %d", count)
	}
}

func TestBufferedLoggerBufferOverflow(t *testing.T) {
	maxEntries := 5
	logger := NewBufferedLogger(maxEntries)

	for i := range 10 {
		logger.Info("message %d", i)
	}

	logger.mu.Lock()
	count := len(logger.buffer)
	firstMessage := logger.buffer[0].message
	logger.mu.Unlock()

	if count != maxEntries {
		t.Errorf("Expected %d buffered messages, got %d", maxEntries, count)
	}

	if firstMessage != "message 5" {
		t.Errorf("Expected first message to be 'message 5', got %q", firstMessage)
	}
}

func TestBufferedLoggerFlush(t *testing.T) {
	buffered := NewBufferedLogger(100)

	// Cache a WithFields reference before flush (simulates what plugins do)
	cached := buffered.WithFields(map[string]any{"plugin": "bundle"})

	buffered.Info("buffered 1")
	buffered.Warn("buffered 2")
	cached.Error("buffered 3")

	testLog := &captureLogger{entries: make([]string, 0)}
	buffered.Flush(testLog)

	if len(testLog.entries) != 3 {
		t.Errorf("Expected 3 flushed entries, got %d", len(testLog.entries))
	}

	expectedMessages := []string{"buffered 1", "buffered 2", "buffered 3"}
	for i, expected := range expectedMessages {
		if testLog.entries[i] != expected {
			t.Errorf("Entry %d: expected %q, got %q", i, expected, testLog.entries[i])
		}
	}

	// After flush, log via cached reference — should forward to target
	cached.Info("forwarded via cached ref")
	buffered.Info("forwarded directly")

	if len(testLog.entries) != 5 {
		t.Fatalf("Expected 5 entries after forwarding, got %d", len(testLog.entries))
	}
	if testLog.entries[3] != "forwarded via cached ref" {
		t.Errorf("Entry 3: expected 'forwarded via cached ref', got %q", testLog.entries[3])
	}
	if testLog.entries[4] != "forwarded directly" {
		t.Errorf("Entry 4: expected 'forwarded directly', got %q", testLog.entries[4])
	}

	// SetLevel/GetLevel should forward to target after flush
	buffered.SetLevel(Error)
	if got := testLog.GetLevel(); got != Error {
		t.Errorf("Expected target level Error after SetLevel, got %v", got)
	}
	if got := buffered.GetLevel(); got != Error {
		t.Errorf("Expected buffered GetLevel to return Error, got %v", got)
	}
}

func TestBufferedLoggerConcurrentWrites(t *testing.T) {
	buffered := NewBufferedLogger(1000)

	done := make(chan bool)
	for i := range 10 {
		go func(id int) {
			for range 100 {
				buffered.Info("message from goroutine %d", id)
			}
			done <- true
		}(i)
	}

	for range 10 {
		<-done
	}

	buffered.mu.Lock()
	count := len(buffered.buffer)
	buffered.mu.Unlock()

	if count != 1000 {
		t.Errorf("Expected 1000 buffered messages, got %d", count)
	}

	// Flush and verify concurrent forwarding works
	testLog := &captureLogger{entries: make([]string, 0)}
	buffered.Flush(testLog)

	cachedLoggers := make([]Logger, 10)
	for i := range cachedLoggers {
		cachedLoggers[i] = buffered.WithFields(map[string]any{"goroutine": i})
	}

	var wg sync.WaitGroup
	for i, logger := range cachedLoggers {
		wg.Add(1)
		go func(id int, l Logger) {
			defer wg.Done()
			for j := range 100 {
				l.Info("fwd %d-%d", id, j)
			}
		}(i, logger)
	}
	wg.Wait()

	// 1000 flushed + 1000 forwarded
	if len(testLog.entries) != 2000 {
		t.Errorf("Expected 2000 total entries, got %d", len(testLog.entries))
	}
}

type captureLogger struct {
	mu      sync.Mutex
	entries []string
	level   Level
}

func (c *captureLogger) Debug(format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	c.mu.Lock()
	c.entries = append(c.entries, msg)
	c.mu.Unlock()
}

func (c *captureLogger) Info(format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	c.mu.Lock()
	c.entries = append(c.entries, msg)
	c.mu.Unlock()
}

func (c *captureLogger) Warn(format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	c.mu.Lock()
	c.entries = append(c.entries, msg)
	c.mu.Unlock()
}

func (c *captureLogger) Error(format string, args ...any) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	c.mu.Lock()
	c.entries = append(c.entries, msg)
	c.mu.Unlock()
}

func (c *captureLogger) WithFields(fields map[string]any) Logger {
	return c
}

func (c *captureLogger) GetLevel() Level {
	return c.level
}

func (c *captureLogger) SetLevel(level Level) {
	c.level = level
}
