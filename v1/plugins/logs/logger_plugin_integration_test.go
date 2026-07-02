package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/server"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/util"
)

type testLoggerPlugin struct {
	manager *plugins.Manager
	handler *mockHandler
}

type mockHandler struct {
	buf *bytes.Buffer
}

func (p *testLoggerPlugin) Start(context.Context) error {
	p.manager.UpdatePluginStatus("test_logger", &plugins.Status{State: plugins.StateOK})
	return nil
}

func (p *testLoggerPlugin) Stop(context.Context) {
	p.manager.UpdatePluginStatus("test_logger", &plugins.Status{State: plugins.StateNotReady})
}

func (*testLoggerPlugin) Reconfigure(context.Context, any) {}

func (p *testLoggerPlugin) Logger() slog.Handler {
	return p.handler
}

func (*mockHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *mockHandler) Handle(_ context.Context, r slog.Record) error {
	entry := make(map[string]any)
	entry["level"] = r.Level.String()
	entry["message"] = r.Message
	r.Attrs(func(a slog.Attr) bool {
		entry[a.Key] = a.Value.Any()
		return true
	})

	data, _ := json.Marshal(entry)
	h.buf.Write(data)
	h.buf.WriteString("\n")
	return nil
}

func (h *mockHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *mockHandler) WithGroup(string) slog.Handler {
	return h
}

func TestDecisionLogsWithLoggerPlugin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		// Arrange
		buf := &bytes.Buffer{}
		testHandler := &mockHandler{
			buf: buf,
		}

		testLoggerPlug := &testLoggerPlugin{
			handler: testHandler,
		}

		mgrCfg := map[string]any{
			"decision_logs": map[string]any{
				"console": false,
				"plugin":  "test_logger",
			},
		}
		managerConfig, err := json.Marshal(mgrCfg)
		if err != nil {
			t.Fatal(err)
		}

		manager, err := plugins.New(managerConfig, "test", inmem.New())
		if err != nil {
			t.Fatal(err)
		}

		testLoggerPlug.manager = manager
		manager.Register("test_logger", testLoggerPlug)

		dlConfig := map[string]any{
			"console": false,
			"plugin":  "test_logger",
		}
		dlConfigBytes, err := json.Marshal(dlConfig)
		if err != nil {
			t.Fatal(err)
		}

		config, err := ParseConfig(dlConfigBytes, nil, []string{"test_logger"})
		if err != nil {
			t.Fatal(err)
		}
		if config == nil {
			t.Fatal("decision logs config is nil")
		}

		decisionLogsPlugin := New(config, manager)
		manager.Register(Name, decisionLogsPlugin)

		if err := testLoggerPlug.Start(ctx); err != nil {
			t.Fatal(err)
		}
		if err := decisionLogsPlugin.Start(ctx); err != nil {
			t.Fatal(err)
		}

		// Act
		inputData := map[string]any{"user": "alice"}
		resultsData := map[string]any{"allow": true}
		info := &server.Info{
			DecisionID: "test-decision-123",
			Path:       "data/test/allow",
			Timestamp:  time.Now(),
			Input:      util.Reference(inputData),
			Results:    util.Reference(resultsData),
			Metrics:    metrics.New(),
		}

		if err := decisionLogsPlugin.Log(ctx, info); err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond)

		// Assert
		logged := buf.String()
		if logged == "" {
			t.Fatal("expected decision log to be written to logger plugin, but buffer is empty")
		}

		if !strings.Contains(logged, "test-decision-123") {
			t.Errorf("expected decision_id in log output, got: %s", logged)
		}
		if !strings.Contains(logged, "data/test/allow") {
			t.Errorf("expected path in log output, got: %s", logged)
		}
		if !strings.Contains(logged, "openpolicyagent.org/decision_logs") {
			t.Errorf("expected type field in log output, got: %s", logged)
		}

		manager.Stop(ctx)
	})
}

func TestDecisionLogsWithBothConsoleAndLoggerPlugin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx := t.Context()

		// Arrange
		buf := &bytes.Buffer{}
		testHandler := &mockHandler{
			buf: buf,
		}

		consoleBuf := &bytes.Buffer{}
		consoleLog := &mockLogger{
			buf:    consoleBuf,
			level:  logging.Info,
			fields: make(map[string]any),
		}

		testLoggerPlug := &testLoggerPlugin{
			handler: testHandler,
		}

		mgrCfg := map[string]any{
			"decision_logs": map[string]any{
				"console": true,
				"plugin":  "test_logger",
			},
		}
		managerConfig, err := json.Marshal(mgrCfg)
		if err != nil {
			t.Fatal(err)
		}

		manager, err := plugins.New(
			managerConfig,
			"test",
			inmem.New(),
			plugins.ConsoleLogger(consoleLog),
		)
		if err != nil {
			t.Fatal(err)
		}

		testLoggerPlug.manager = manager
		manager.Register("test_logger", testLoggerPlug)

		dlConfig := map[string]any{
			"console": true,
			"plugin":  "test_logger",
		}
		dlConfigBytes, err := json.Marshal(dlConfig)
		if err != nil {
			t.Fatal(err)
		}

		config, err := ParseConfig(dlConfigBytes, nil, []string{"test_logger"})
		if err != nil {
			t.Fatal(err)
		}
		if config == nil {
			t.Fatal("decision logs config is nil")
		}

		decisionLogsPlugin := New(config, manager)
		manager.Register(Name, decisionLogsPlugin)

		if err := testLoggerPlug.Start(ctx); err != nil {
			t.Fatal(err)
		}
		if err := decisionLogsPlugin.Start(ctx); err != nil {
			t.Fatal(err)
		}

		// Act
		inputData := map[string]any{"user": "bob"}
		resultsData := map[string]any{"allow": false}
		info := &server.Info{
			DecisionID: "test-decision-456",
			Path:       "data/test/deny",
			Timestamp:  time.Now(),
			Input:      util.Reference(inputData),
			Results:    util.Reference(resultsData),
			Metrics:    metrics.New(),
		}

		if err := decisionLogsPlugin.Log(ctx, info); err != nil {
			t.Fatal(err)
		}

		time.Sleep(100 * time.Millisecond)

		// Assert
		pluginLogged := buf.String()
		if pluginLogged == "" {
			t.Fatal("expected decision log to be written to logger plugin, but buffer is empty")
		}

		consoleLogged := consoleBuf.String()
		if consoleLogged == "" {
			t.Fatal("expected decision log to be written to console logger, but buffer is empty")
		}

		if !strings.Contains(pluginLogged, "test-decision-456") {
			t.Errorf("expected decision_id in logger plugin output")
		}
		if !strings.Contains(consoleLogged, "test-decision-456") {
			t.Errorf("expected decision_id in console logger output")
		}

		manager.Stop(ctx)
	})
}

func TestDecisionLogsWithMissingLoggerPlugin(t *testing.T) {
	dlConfig := map[string]any{
		"console": false,
		"plugin":  "nonexistent_logger",
	}
	dlConfigBytes, err := json.Marshal(dlConfig)
	if err != nil {
		t.Fatal(err)
	}

	config, err := ParseConfig(dlConfigBytes, nil, []string{})
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
	if config != nil {
		t.Fatal("expected config to be nil for invalid plugin")
	}
}

type mockLogger struct {
	buf    *bytes.Buffer
	level  logging.Level
	fields map[string]any
}

func (l *mockLogger) Debug(format string, args ...any) {
	l.log("DEBUG", format, args)
}

func (l *mockLogger) Info(format string, args ...any) {
	l.log("INFO", format, args)
}

func (l *mockLogger) Warn(format string, args ...any) {
	l.log("WARN", format, args)
}

func (l *mockLogger) Error(format string, args ...any) {
	l.log("ERROR", format, args)
}

func (l *mockLogger) WithFields(fields map[string]any) logging.Logger {
	newFields := make(map[string]any, len(l.fields)+len(fields))
	maps.Copy(newFields, l.fields)
	maps.Copy(newFields, fields)
	return &mockLogger{
		buf:    l.buf,
		level:  l.level,
		fields: newFields,
	}
}

func (l *mockLogger) GetLevel() logging.Level {
	return l.level
}

func (l *mockLogger) SetLevel(level logging.Level) {
	l.level = level
}

func (l *mockLogger) log(level string, format string, args []any) {
	entry := make(map[string]any, 2+len(l.fields))
	entry["level"] = level
	maps.Copy(entry, l.fields)
	entry["message"] = format

	data, _ := json.Marshal(entry)
	l.buf.Write(data)
	l.buf.WriteString("\n")
}

func TestDecisionLogsLoggerPluginLookup(t *testing.T) {
	buf := &bytes.Buffer{}
	testHandler := &mockHandler{
		buf: buf,
	}

	testLoggerPlug := &testLoggerPlugin{
		handler: testHandler,
	}

	mgrCfg := map[string]any{}
	managerConfig, err := json.Marshal(mgrCfg)
	if err != nil {
		t.Fatal(err)
	}

	manager, err := plugins.New(managerConfig, "test", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	testLoggerPlug.manager = manager
	manager.Register("test_logger", testLoggerPlug)

	directLookup := manager.Plugin("test_logger")
	if directLookup == nil {
		t.Fatal("expected to find plugin via direct lookup")
	}

	if directLookup != testLoggerPlug {
		t.Fatal("expected direct lookup to return registered logger plugin")
	}

	loggerPlug, ok := directLookup.(plugins.LoggerPlugin)
	if !ok {
		t.Fatal("expected plugin to implement LoggerPlugin interface")
	}

	logger := loggerPlug.Logger()
	if logger == nil {
		t.Fatal("expected Logger() to return non-nil logger")
	}
}
