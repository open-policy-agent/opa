package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/plugins"
)

// Config holds the configuration for the file logger plugin.
type Config struct {
	Path       string `json:"path"`
	MaxSize    int    `json:"max_size_mb"`
	MaxAge     int    `json:"max_age_days"`
	MaxBackups int    `json:"max_backups"`
	Compress   bool   `json:"compress"`
	Level      string `json:"level"`
}

// Plugin implements the logger plugin interface for file-based logging.
type Plugin struct {
	manager *plugins.Manager
	config  Config
	logger  *fileLogger
	mtx     sync.Mutex
}

type fileLogger struct {
	writer io.Writer
	level  logging.Level
	mtx    sync.RWMutex
}

// Factory creates file logger plugin instances.
type Factory struct{}

// Validate validates the configuration for the file logger plugin.
func (_ *Factory) Validate(manager *plugins.Manager, config []byte) (any, error) {
	var parsedConfig Config
	if err := json.Unmarshal(config, &parsedConfig); err != nil {
		return nil, fmt.Errorf("failed to parse file logger config: %w", err)
	}

	if parsedConfig.Path == "" {
		return nil, errors.New("file logger requires 'path' to be specified")
	}

	if parsedConfig.MaxSize <= 0 {
		parsedConfig.MaxSize = 100
	}
	if parsedConfig.MaxAge <= 0 {
		parsedConfig.MaxAge = 28
	}
	if parsedConfig.MaxBackups == 0 {
		parsedConfig.MaxBackups = 3
	}

	if parsedConfig.Level == "" {
		parsedConfig.Level = "info"
	}

	return parsedConfig, nil
}

// New creates a new file logger plugin instance.
func (_ *Factory) New(manager *plugins.Manager, config any) plugins.Plugin {
	parsedConfig := config.(Config)

	return &Plugin{
		manager: manager,
		config:  parsedConfig,
	}
}

// Start initializes the file logger and starts writing logs to the configured file.
func (p *Plugin) Start(context.Context) error {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	lumber := &lumberjack.Logger{
		Filename:   p.config.Path,
		MaxSize:    p.config.MaxSize,
		MaxAge:     p.config.MaxAge,
		MaxBackups: p.config.MaxBackups,
		Compress:   p.config.Compress,
	}

	level := parseLevel(p.config.Level)

	p.logger = &fileLogger{
		writer: lumber,
		level:  level,
	}

	p.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateOK})
	return nil
}

// Stop closes the file logger.
func (p *Plugin) Stop(context.Context) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if p.logger != nil {
		if closer, ok := p.logger.writer.(io.Closer); ok {
			closer.Close()
		}
		p.logger = nil
	}

	p.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateNotReady})
}

// Reconfigure updates the file logger configuration.
func (p *Plugin) Reconfigure(_ context.Context, config any) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	newConfig := config.(Config)
	p.config = newConfig

	if p.logger != nil {
		p.logger.SetLevel(parseLevel(newConfig.Level))
	}
}

func (p *Plugin) Logger() logging.Logger {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.logger
}

func (l *fileLogger) Debug(format string, args ...any) {
	l.log(logging.Debug, format, args, nil)
}

func (l *fileLogger) Info(format string, args ...any) {
	l.log(logging.Info, format, args, nil)
}

func (l *fileLogger) Warn(format string, args ...any) {
	l.log(logging.Warn, format, args, nil)
}

func (l *fileLogger) Error(format string, args ...any) {
	l.log(logging.Error, format, args, nil)
}

func (l *fileLogger) WithFields(fields map[string]any) logging.Logger {
	return &fileLoggerWithFields{
		parent: l,
		fields: fields,
	}
}

func (l *fileLogger) WithContext(context.Context) logging.Logger {
	return l
}

func (l *fileLogger) GetLevel() logging.Level {
	l.mtx.RLock()
	defer l.mtx.RUnlock()
	return l.level
}

func (l *fileLogger) SetLevel(level logging.Level) {
	l.mtx.Lock()
	defer l.mtx.Unlock()
	l.level = level
}

func (l *fileLogger) log(level logging.Level, format string, args []any, fields map[string]any) {
	l.mtx.RLock()
	currentLevel := l.level
	writer := l.writer
	l.mtx.RUnlock()

	if level > currentLevel {
		return
	}

	message := format
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	}

	logEntry := make(map[string]any, 2+len(fields))
	logEntry["level"] = levelToString(level)
	logEntry["message"] = message
	maps.Copy(logEntry, fields)

	jsonBytes, err := json.Marshal(logEntry)
	if err != nil {
		jsonBytes = fmt.Appendf(nil, `{"level":"%s","message":"marshaling error: %v"}`, levelToString(level), err)
	}

	writer.Write(append(jsonBytes, '\n'))
}

type fileLoggerWithFields struct {
	parent *fileLogger
	fields map[string]any
}

func (f *fileLoggerWithFields) Debug(format string, args ...any) {
	f.parent.log(logging.Debug, format, args, f.fields)
}

func (f *fileLoggerWithFields) Info(format string, args ...any) {
	f.parent.log(logging.Info, format, args, f.fields)
}

func (f *fileLoggerWithFields) Warn(format string, args ...any) {
	f.parent.log(logging.Warn, format, args, f.fields)
}

func (f *fileLoggerWithFields) Error(format string, args ...any) {
	f.parent.log(logging.Error, format, args, f.fields)
}

func (f *fileLoggerWithFields) WithFields(fields map[string]any) logging.Logger {
	merged := make(map[string]any, len(f.fields)+len(fields))
	maps.Copy(merged, f.fields)
	maps.Copy(merged, fields)
	return &fileLoggerWithFields{
		parent: f.parent,
		fields: merged,
	}
}

func (f *fileLoggerWithFields) WithContext(context.Context) logging.Logger {
	return f
}

func (f *fileLoggerWithFields) GetLevel() logging.Level {
	return f.parent.GetLevel()
}

func (f *fileLoggerWithFields) SetLevel(level logging.Level) {
	f.parent.SetLevel(level)
}

func levelToString(level logging.Level) string {
	switch level {
	case logging.Debug:
		return "DEBUG"
	case logging.Info:
		return "INFO"
	case logging.Warn:
		return "WARN"
	case logging.Error:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func parseLevel(levelStr string) logging.Level {
	switch levelStr {
	case "debug":
		return logging.Debug
	case "info":
		return logging.Info
	case "warn":
		return logging.Warn
	case "error":
		return logging.Error
	default:
		return logging.Info
	}
}
