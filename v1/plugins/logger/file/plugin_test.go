package file

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/v1/plugins"
	opalogger "github.com/open-policy-agent/opa/v1/plugins/logger"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func TestFileLoggerPlugin(t *testing.T) {
	ctx := t.Context()
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "opa.log")

	config := Config{
		Path:       logFile,
		MaxSize:    1,
		MaxAge:     1,
		MaxBackups: 1,
		Compress:   false,
		Level:      "debug",
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	manager, err := plugins.New([]byte("{}"), "test-instance", inmem.New())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	factory := &Factory{}
	validatedConfig, err := factory.Validate(manager, configJSON)
	if err != nil {
		t.Fatalf("Failed to validate config: %v", err)
	}

	plugin := factory.New(manager, validatedConfig)
	manager.Register(Name, plugin)

	if err := plugin.Start(ctx); err != nil {
		t.Fatalf("Failed to start plugin: %v", err)
	}
	defer plugin.Stop(ctx)

	loggerPlugin := plugin.(opalogger.LoggerPlugin)
	handler := loggerPlugin.Logger()
	logger := slog.New(handler)

	logger.Info("test info message")
	logger.Debug("test debug message")
	logger.Warn("test warn message")
	logger.Error("test error message")

	logger.Info("message with fields",
		slog.String("key1", "value1"),
		slog.Int("key2", 123),
	)

	plugin.Stop(ctx)

	f, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var entries []map[string]any
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("Failed to parse JSON line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}

	if len(entries) != 5 {
		t.Fatalf("Expected 5 log entries, got %d", len(entries))
	}

	assertLogEntry(t, entries[0], "INFO", "test info message", nil)
	assertLogEntry(t, entries[1], "DEBUG", "test debug message", nil)
	assertLogEntry(t, entries[2], "WARN", "test warn message", nil)
	assertLogEntry(t, entries[3], "ERROR", "test error message", nil)
	assertLogEntry(t, entries[4], "INFO", "message with fields", map[string]any{
		"key1": "value1",
		"key2": float64(123),
	})
}

func TestFileLoggerLevelFiltering(t *testing.T) {
	ctx := t.Context()
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "opa.log")

	config := Config{
		Path:       logFile,
		MaxSize:    1,
		MaxAge:     1,
		MaxBackups: 1,
		Compress:   false,
		Level:      "warn",
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	manager, err := plugins.New([]byte("{}"), "test-instance", inmem.New())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	factory := &Factory{}
	validatedConfig, err := factory.Validate(manager, configJSON)
	if err != nil {
		t.Fatalf("Failed to validate config: %v", err)
	}

	plugin := factory.New(manager, validatedConfig)

	if err := plugin.Start(ctx); err != nil {
		t.Fatalf("Failed to start plugin: %v", err)
	}
	defer plugin.Stop(ctx)

	loggerPlugin := plugin.(opalogger.LoggerPlugin)
	handler := loggerPlugin.Logger()
	logger := slog.New(handler)

	logger.Debug("should not be logged")
	logger.Info("should not be logged either")
	logger.Warn("should be logged")
	logger.Error("should also be logged")

	plugin.Stop(ctx)

	f, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var entries []map[string]any
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("Failed to parse JSON line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 log entries (debug and info filtered), got %d", len(entries))
	}

	assertLogEntry(t, entries[0], "WARN", "should be logged", nil)
	assertLogEntry(t, entries[1], "ERROR", "should also be logged", nil)
}

func TestFileLoggerConfigValidation(t *testing.T) {
	manager, err := plugins.New([]byte("{}"), "test-instance", inmem.New())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	factory := &Factory{}

	t.Run("missing path", func(t *testing.T) {
		config := []byte(`{}`)
		_, err := factory.Validate(manager, config)
		if err == nil {
			t.Error("Expected validation error for missing path")
		}
	})

	t.Run("valid config with defaults", func(t *testing.T) {
		config := []byte(`{"path": "/tmp/opa.log"}`)
		validatedConfig, err := factory.Validate(manager, config)
		if err != nil {
			t.Errorf("Unexpected validation error: %v", err)
		}

		cfg := validatedConfig.(Config)
		if cfg.MaxSize != 100 {
			t.Errorf("Expected default MaxSize=100, got %d", cfg.MaxSize)
		}
		if cfg.MaxAge != 28 {
			t.Errorf("Expected default MaxAge=28, got %d", cfg.MaxAge)
		}
		if cfg.MaxBackups != 3 {
			t.Errorf("Expected default MaxBackups=3, got %d", cfg.MaxBackups)
		}
		if cfg.Level != "info" {
			t.Errorf("Expected default Level=info, got %s", cfg.Level)
		}
	})
}

func TestFileLoggerWithAttrs(t *testing.T) {
	ctx := t.Context()
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "opa.log")

	config := Config{
		Path:       logFile,
		MaxSize:    1,
		MaxAge:     1,
		MaxBackups: 1,
		Compress:   false,
		Level:      "info",
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	manager, err := plugins.New([]byte("{}"), "test-instance", inmem.New())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	factory := &Factory{}
	validatedConfig, err := factory.Validate(manager, configJSON)
	if err != nil {
		t.Fatalf("Failed to validate config: %v", err)
	}

	plugin := factory.New(manager, validatedConfig)
	if err := plugin.Start(ctx); err != nil {
		t.Fatalf("Failed to start plugin: %v", err)
	}
	defer plugin.Stop(ctx)

	loggerPlugin := plugin.(opalogger.LoggerPlugin)
	handler := loggerPlugin.Logger()

	logger1 := slog.New(handler).With(slog.String("component", "runtime"))
	logger1.Info("message from runtime")

	logger2 := logger1.With(slog.String("subsystem", "storage"))
	logger2.Warn("warning from storage")

	plugin.Stop(ctx)

	f, err := os.Open(logFile)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var entries []map[string]any
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("Failed to parse JSON line %q: %v", line, err)
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 log entries, got %d", len(entries))
	}

	assertLogEntry(t, entries[0], "INFO", "message from runtime", map[string]any{
		"component": "runtime",
	})
	assertLogEntry(t, entries[1], "WARN", "warning from storage", map[string]any{
		"component": "runtime",
		"subsystem": "storage",
	})
}

func assertLogEntry(t *testing.T, entry map[string]any, expectedLevel, expectedMessage string, expectedFields map[string]any) {
	t.Helper()

	level, ok := entry["level"].(string)
	if !ok {
		t.Errorf("Log entry missing 'level' field or not a string: %v", entry)
		return
	}
	if level != expectedLevel {
		t.Errorf("Expected level %q, got %q", expectedLevel, level)
	}

	message, ok := entry["msg"].(string)
	if !ok {
		t.Errorf("Log entry missing 'msg' field or not a string: %v", entry)
		return
	}
	if message != expectedMessage {
		t.Errorf("Expected message %q, got %q", expectedMessage, message)
	}

	for key, expectedValue := range expectedFields {
		actualValue, ok := entry[key]
		if !ok {
			t.Errorf("Expected field %q not found in log entry", key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("Field %q: expected %v, got %v", key, expectedValue, actualValue)
		}
	}
}
