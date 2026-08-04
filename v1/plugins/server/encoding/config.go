package encoding

import (
	"context"
	_ "embed"

	"github.com/open-policy-agent/opa/internal/configpolicy"
	"github.com/open-policy-agent/opa/v1/config"
)

//go:embed validate.rego
var validationModule string

var validationPolicy = configpolicy.New(
	"opa/config/server/encoding/validate.rego",
	validationModule,
	"data.opa.config.server.encoding = x",
)

func init() {
	config.RegisterConfigSpec(config.SpecsFromStruct[Config]("server", "encoding")...)
}

// Config represents the configuration for the Server.Encoding settings
type Config struct {
	Gzip *Gzip `json:"gzip,omitempty"`
}

// Gzip represents the configuration for the Server.Encoding.Gzip settings
type Gzip struct {
	MinLength        *int `json:"min_length,omitempty"`        // the minimum length of a response that will be gzipped
	CompressionLevel *int `json:"compression_level,omitempty"` // the compression level for gzip
}

// ConfigBuilder assists in the construction of the plugin configuration.
type ConfigBuilder struct {
	raw []byte
}

// NewConfigBuilder returns a new ConfigBuilder to build and parse the server config
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{}
}

// WithBytes sets the raw server config
func (b *ConfigBuilder) WithBytes(config []byte) *ConfigBuilder {
	b.raw = config
	return b
}

// Parse returns a valid Config object with defaults injected.
func (b *ConfigBuilder) Parse() (*Config, error) {
	return b.ParseWithContext(context.Background())
}

// ParseWithContext returns a valid Config object with defaults injected, using
// ctx to evaluate the validation policy.
func (b *ConfigBuilder) ParseWithContext(ctx context.Context) (*Config, error) {
	var result Config
	if _, err := configpolicy.EvalConfigInto(ctx, validationPolicy, b.raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
