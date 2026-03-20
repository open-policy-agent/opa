package logger_test

import (
	"context"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/logging/test"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/logger"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

type testLoggerPlugin struct {
	manager *plugins.Manager
	logger  *test.Logger
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

func (p *testLoggerPlugin) Logger() slog.Handler {
	return logging.NewSlogHandler(p.logger)
}

type testLoggerFactory struct {
	logger *test.Logger
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

		testLog := test.New()
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

		p := manager.Plugin(testLoggerName)
		if p == nil {
			t.Fatal("Logger plugin not found")
		}

		loggerPlugin, ok := p.(logger.LoggerPlugin)
		if !ok {
			t.Fatal("Plugin does not implement LoggerPlugin interface")
		}

		handler := loggerPlugin.Logger()
		// Wrap the slog.Handler in a Logger adapter
		targetLogger := logging.NewLoggerFromSlogHandler(handler, logging.Debug)
		bufferedLogger.Flush(targetLogger)

		time.Sleep(100 * time.Millisecond)

		entries := testLog.Entries()
		if len(entries) < 3 {
			t.Fatalf("Expected at least 3 buffered entries (Debug filtered out), got %d", len(entries))
		}

		expectedMessages := []string{"early log 1", "early log with fields", "early error log"}
		for i, expected := range expectedMessages {
			if entries[i].Message != expected {
				t.Errorf("Entry %d: expected message %q, got %q", i, expected, entries[i].Message)
			}
		}

		if entries[1].Fields == nil || entries[1].Fields["key"] != "value" {
			t.Errorf("Entry 1: expected fields with key=value, got %v", entries[1].Fields)
		}

		for i, entry := range entries[:3] {
			if entry.Time.Before(startTime) {
				t.Errorf("Entry %d: log time %v is before start time %v", i, entry.Time, startTime)
			}
		}

		beforeDirectLog := len(entries)

		// After flush, use the target logger directly
		targetLogger.Info("direct log after plugin set")

		time.Sleep(50 * time.Millisecond)

		entries = testLog.Entries()
		if len(entries) != beforeDirectLog+1 {
			t.Errorf("Expected %d entries after direct log, got %d", beforeDirectLog+1, len(entries))
		}

		lastEntry := entries[len(entries)-1]
		if lastEntry.Message != "direct log after plugin set" {
			t.Errorf("Last entry: expected 'direct log after plugin set', got %q", lastEntry.Message)
		}
	})
}

func TestBufferedLoggerWithFields(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		bufferedLogger := logging.NewBufferedLogger(1000)
		testLog := test.New()

	logger1 := bufferedLogger.WithFields(map[string]any{"component": "runtime"})
	logger1.Info("buffered message")

		bufferedLogger.Flush(testLog)
		time.Sleep(50 * time.Millisecond)

		// After flush, use the target logger directly
		logger2 := testLog.WithFields(map[string]any{"component": "server"})
		logger2.Info("direct message")
		time.Sleep(50 * time.Millisecond)

		entries := testLog.Entries()
		if len(entries) != 2 {
			t.Fatalf("Expected 2 entries, got %d", len(entries))
		}

		if entries[0].Fields["component"] != "runtime" {
			t.Errorf("First entry: expected component=runtime, got %v", entries[0].Fields)
		}

		if entries[1].Fields["component"] != "server" {
			t.Errorf("Second entry: expected component=server, got %v", entries[1].Fields)
		}
	})
}
