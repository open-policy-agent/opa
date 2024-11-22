package encoding

import (
	v1 "github.com/open-policy-agent/opa/v1/plugins/server/encoding"
)

// Config represents the configuration for the Server.Encoding settings
type Config = v1.Config

// Gzip represents the configuration for the Server.Encoding.Gzip settings
type Gzip = v1.Gzip

// ConfigBuilder assists in the construction of the plugin configuration.
type ConfigBuilder = v1.ConfigBuilder

// NewConfigBuilder returns a new ConfigBuilder to build and parse the server config
func NewConfigBuilder() *ConfigBuilder {
	return v1.NewConfigBuilder()
}
