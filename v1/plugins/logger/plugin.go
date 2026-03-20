package logger

import (
	"log/slog"

	"github.com/open-policy-agent/opa/v1/plugins"
)

// LoggerPlugin defines the interface for logging plugins.
type LoggerPlugin interface {
	plugins.Plugin

	// Logger returns the slog.Handler implementation provided by this plugin.
	Logger() slog.Handler
}

// Lookup returns the slog.Handler from the logger plugin registered with the manager based on configuration.
func Lookup(manager *plugins.Manager) slog.Handler {
	configObj := manager.GetConfig()
	if configObj == nil {
		return nil
	}

	if configObj.Server == nil || configObj.Server.LoggerPlugin == nil {
		return nil
	}

	p := manager.Plugin(*configObj.Server.LoggerPlugin)
	if p == nil {
		return nil
	}

	loggerPlugin, ok := p.(LoggerPlugin)
	if !ok {
		return nil
	}

	return loggerPlugin.Logger()
}
