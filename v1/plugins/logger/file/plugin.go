package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"gopkg.in/natefinch/lumberjack.v2"

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
	manager  *plugins.Manager
	config   Config
	handler  slog.Handler
	lumber   io.Closer
	levelVar *slog.LevelVar
	mtx      sync.Mutex
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

	if p.handler != nil {
		return errors.New("file logger already started")
	}

	lumber := &lumberjack.Logger{
		Filename:   p.config.Path,
		MaxSize:    p.config.MaxSize,
		MaxAge:     p.config.MaxAge,
		MaxBackups: p.config.MaxBackups,
		Compress:   p.config.Compress,
	}

	level := parseLevel(p.config.Level)
	p.levelVar = new(slog.LevelVar)
	p.levelVar.Set(level)

	opts := &slog.HandlerOptions{
		Level: p.levelVar,
	}

	p.handler = slog.NewJSONHandler(lumber, opts)
	p.lumber = lumber

	p.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateOK})
	return nil
}

// Stop closes the file logger.
func (p *Plugin) Stop(context.Context) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	if p.lumber != nil {
		p.lumber.Close()
		p.lumber = nil
	}

	p.handler = nil
	p.levelVar = nil

	p.manager.UpdatePluginStatus(Name, &plugins.Status{State: plugins.StateNotReady})
}

// Reconfigure updates the file logger configuration.
func (p *Plugin) Reconfigure(ctx context.Context, config any) {
	newConfig := config.(Config)

	p.mtx.Lock()
	oldConfig := p.config
	levelVar := p.levelVar
	p.config = newConfig
	p.mtx.Unlock()

	if oldConfig.Path != newConfig.Path ||
		oldConfig.MaxSize != newConfig.MaxSize ||
		oldConfig.MaxAge != newConfig.MaxAge ||
		oldConfig.MaxBackups != newConfig.MaxBackups ||
		oldConfig.Compress != newConfig.Compress {
		p.Stop(ctx)
		_ = p.Start(ctx)
	} else if oldConfig.Level != newConfig.Level && levelVar != nil {
		level := parseLevel(newConfig.Level)
		levelVar.Set(level)
	}
}

func (p *Plugin) Logger() slog.Handler {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.handler
}

func parseLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
