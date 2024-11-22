package metrics

import (
	v1 "github.com/open-policy-agent/opa/v1/plugins/server/metrics"
)

// Config represents the configuration for the Server.Metrics settings
type Config = v1.Config

// Prom represents the configuration for the Server.Metrics.Prom settings
type Prom = v1.Prom

// HTTPRequestDurationSeconds represents the configuration for the Server.Metrics.Prom.HTTPRequestDurationSeconds settings
type HTTPRequestDurationSeconds = v1.HTTPRequestDurationSeconds

// ConfigBuilder assists in the construction of the plugin configuration.
type ConfigBuilder = v1.ConfigBuilder

// NewConfigBuilder returns a new ConfigBuilder to build and parse the server config
func NewConfigBuilder() *ConfigBuilder {
	return v1.NewConfigBuilder()
}
