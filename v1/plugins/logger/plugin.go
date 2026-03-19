package logger

import (
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/plugins"
)

// LoggerPlugin defines the interface for logging plugins.
type LoggerPlugin interface {
	plugins.Plugin

	// Logger returns the logger implementation provided by this plugin.
	Logger() logging.Logger
}

// Lookup returns the logger from the logger plugin registered with the manager based on configuration.
func Lookup(manager *plugins.Manager) logging.Logger {
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
