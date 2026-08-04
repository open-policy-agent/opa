// Package decoding implements the configuration side of the upgraded gzip
// decompression framework. The original work only enabled gzip decoding for
// a few endpoints-- here we enable if for all of OPA. Additionally, we provide
// some new defensive configuration options: max_length, and gzip.max_length.
// These allow rejecting requests that indicate their contents are larger than
// the size limits.
//
// The request handling pipeline now looks roughly like this:
//
// Request -> MaxBytesReader(Config.MaxLength) -> ir.CopyN(dest, req, Gzip.MaxLength)
//
// The intent behind this design is to improve how OPA handles large and/or
// malicious requests, compressed or otherwise. The benefit of being a little
// more strict in what we allow is that we can now use "riskier", but
// dramatically more performant techniques, like preallocating content buffers
// for gzipped data. This also should help OPAs in limited memory situations.
package decoding

import (
	"context"
	_ "embed"

	"github.com/open-policy-agent/opa/internal/configpolicy"
	"github.com/open-policy-agent/opa/v1/config"
)

//go:embed validate.rego
var validationModule string

var validationPolicy = configpolicy.New(
	"opa/config/server/decoding/validate.rego",
	validationModule,
	"data.opa.config.server.decoding = x",
)

func init() {
	config.RegisterConfigSpec(config.SpecsFromStruct[Config]("server", "decoding")...)
}

// Config represents the configuration for the Server.Decoding settings
type Config struct {
	MaxLength *int64 `json:"max_length,omitempty"` // maximum request size that will be read, regardless of compression.
	Gzip      *Gzip  `json:"gzip,omitempty"`
}

// Gzip represents the configuration for the Server.Decoding.Gzip settings
type Gzip struct {
	MaxLength *int64 `json:"max_length,omitempty"` // Max number of bytes allowed to be read from the decompressor.
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
