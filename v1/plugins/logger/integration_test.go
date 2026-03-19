package logger_test

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/logger"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

type testLogEntry struct {
	level   logging.Level
	message string
	fields  map[string]any
	time    time.Time
}

type testLogger struct {
	mu      sync.RWMutex
	entries []testLogEntry
	level   logging.Level
}

func newTestLogger() *testLogger {
	return &testLogger{
		entries: make([]testLogEntry, 0),
		level:   logging.Info,
	}
}

func (t *testLogger) Debug(format string, args ...any) {
	t.log(logging.Debug, format, args, nil)
}

func (t *testLogger) Info(format string, args ...any) {
	t.log(logging.Info, format, args, nil)
}

func (t *testLogger) Warn(format string, args ...any) {
	t.log(logging.Warn, format, args, nil)
}

func (t *testLogger) Error(format string, args ...any) {
	t.log(logging.Error, format, args, nil)
}

func (t *testLogger) WithFields(fields map[string]any) logging.Logger {
	return &testLoggerWithFields{
		parent: t,
		fields: fields,
	}
}

func (t *testLogger) WithContext(context.Context) logging.Logger {
	return t
}

func (t *testLogger) GetLevel() logging.Level {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.level
}

func (t *testLogger) SetLevel(level logging.Level) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.level = level
}

func (t *testLogger) log(level logging.Level, format string, args []any, fields map[string]any) {
	if level > t.GetLevel() {
		return
	}

	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	entry := testLogEntry{
		level:   level,
		message: message,
		fields:  fields,
		time:    time.Now(),
	}

	t.entries = append(t.entries, entry)
}

func (t *testLogger) getEntries() []testLogEntry {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make([]testLogEntry, len(t.entries))
	copy(result, t.entries)
	return result
}

type testLoggerWithFields struct {
	parent *testLogger
	fields map[string]any
}

func (t *testLoggerWithFields) Debug(format string, args ...any) {
	t.parent.log(logging.Debug, format, args, t.fields)
}

func (t *testLoggerWithFields) Info(format string, args ...any) {
	t.parent.log(logging.Info, format, args, t.fields)
}

func (t *testLoggerWithFields) Warn(format string, args ...any) {
	t.parent.log(logging.Warn, format, args, t.fields)
}

func (t *testLoggerWithFields) Error(format string, args ...any) {
	t.parent.log(logging.Error, format, args, t.fields)
}

func (t *testLoggerWithFields) WithFields(fields map[string]any) logging.Logger {
	merged := make(map[string]any, len(t.fields)+len(fields))
	maps.Copy(merged, t.fields)
	maps.Copy(merged, fields)
	return &testLoggerWithFields{
		parent: t.parent,
		fields: merged,
	}
}

func (t *testLoggerWithFields) WithContext(context.Context) logging.Logger {
	return t
}

func (t *testLoggerWithFields) GetLevel() logging.Level {
	return t.parent.GetLevel()
}

func (t *testLoggerWithFields) SetLevel(level logging.Level) {
	t.parent.SetLevel(level)
}

type testLoggerPlugin struct {
	manager *plugins.Manager
	logger  *testLogger
}

const testLoggerName = "test_logger"

func (p *testLoggerPlugin) Start(context.Context) error {
	p.manager.UpdatePluginStatus(testLoggerName, &plugins.Status{State: plugins.StateOK})
	return nil
}

func (p *testLoggerPlugin) Stop(context.Context) {
	p.manager.UpdatePluginStatus(testLoggerName, &plugins.Status{State: plugins.StateNotReady})
}

func (*testLoggerPlugin) Reconfigure(context.Context, any) {}

func (p *testLoggerPlugin) Logger() logging.Logger {
	return p.logger
}

type testLoggerFactory struct {
	logger *testLogger
}

func (_ *testLoggerFactory) Validate(manager *plugins.Manager, config []byte) (any, error) {
	return nil, nil
}

func (f *testLoggerFactory) New(manager *plugins.Manager, config any) plugins.Plugin {
	return &testLoggerPlugin{
		manager: manager,
		logger:  f.logger,
	}
}

func TestBufferedLoggerIntegration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		bufferedLogger := logging.NewBufferedLogger(1000)

		startTime := time.Now()

		bufferedLogger.Info("early log 1")
		time.Sleep(10 * time.Millisecond)
		bufferedLogger.Debug("early log 2")
		bufferedLogger.WithFields(map[string]any{"key": "value"}).Warn("early log with fields")
		bufferedLogger.Error("early error log")

		testLog := newTestLogger()
		testLog.SetLevel(logging.Debug)
		factory := &testLoggerFactory{logger: testLog}

		config := []byte(`{"logger": {"plugin": "test_logger"}}`)

		manager, err := plugins.New(config, "test-instance", inmem.New(),
			plugins.Logger(bufferedLogger))
		if err != nil {
			t.Fatalf("Failed to create manager: %v", err)
		}

		manager.Register(testLoggerName, factory.New(manager, nil))

		if err := manager.Init(ctx); err != nil {
			t.Fatalf("Failed to initialize manager: %v", err)
		}

		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Failed to start manager: %v", err)
		}
		defer manager.Stop(ctx)

		loggerPlugin := logger.Lookup(manager)
		if loggerPlugin == nil {
			t.Fatal("Logger plugin not found")
		}

		bufferedLogger.SetTarget(loggerPlugin.Logger())

		time.Sleep(100 * time.Millisecond)

		entries := testLog.getEntries()
		if len(entries) < 3 {
			t.Fatalf("Expected at least 3 buffered entries (Debug filtered out), got %d", len(entries))
		}

		expectedMessages := []string{"early log 1", "early log with fields", "early error log"}
		for i, expected := range expectedMessages {
			if entries[i].message != expected {
				t.Errorf("Entry %d: expected message %q, got %q", i, expected, entries[i].message)
			}
		}

		if entries[1].fields == nil || entries[1].fields["key"] != "value" {
			t.Errorf("Entry 1: expected fields with key=value, got %v", entries[1].fields)
		}

		for i, entry := range entries[:3] {
			if entry.time.Before(startTime) {
				t.Errorf("Entry %d: log time %v is before start time %v", i, entry.time, startTime)
			}
		}

		beforeDirectLog := len(entries)

		bufferedLogger.Info("direct log after plugin set")

		time.Sleep(50 * time.Millisecond)

		entries = testLog.getEntries()
		if len(entries) != beforeDirectLog+1 {
			t.Errorf("Expected %d entries after direct log, got %d", beforeDirectLog+1, len(entries))
		}

		lastEntry := entries[len(entries)-1]
		if lastEntry.message != "direct log after plugin set" {
			t.Errorf("Last entry: expected 'direct log after plugin set', got %q", lastEntry.message)
		}
	})
}

func TestBufferedLoggerWithFields(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bufferedLogger := logging.NewBufferedLogger(1000)
		testLog := newTestLogger()

	logger1 := bufferedLogger.WithFields(map[string]any{"component": "runtime"})
	logger1.Info("buffered message")

		bufferedLogger.SetTarget(testLog)
		time.Sleep(50 * time.Millisecond)

		logger2 := bufferedLogger.WithFields(map[string]any{"component": "server"})
		logger2.Info("direct message")
		time.Sleep(50 * time.Millisecond)

		entries := testLog.getEntries()
		if len(entries) != 2 {
			t.Fatalf("Expected 2 entries, got %d", len(entries))
		}

		if entries[0].fields["component"] != "runtime" {
			t.Errorf("First entry: expected component=runtime, got %v", entries[0].fields)
		}

		if entries[1].fields["component"] != "server" {
			t.Errorf("Second entry: expected component=server, got %v", entries[1].fields)
		}
	})
}
