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
	"fmt"

	"github.com/open-policy-agent/opa/util"
)

var (
	defaultMaxRequestLength     = int64(268435456) // 256 MB
	defaultGzipMaxContentLength = int64(536870912) // 512 MB
)

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
	if b.raw == nil {
		defaultConfig := &Config{
			MaxLength: &defaultMaxRequestLength,
			Gzip: &Gzip{
				MaxLength: &defaultGzipMaxContentLength,
			},
		}
		return defaultConfig, nil
	}

	var result Config

	if err := util.Unmarshal(b.raw, &result); err != nil {
		return nil, err
	}

	return &result, result.validateAndInjectDefaults()
}

// validateAndInjectDefaults populates defaults if the fields are nil, then
// validates the config values.
func (c *Config) validateAndInjectDefaults() error {
	if c.MaxLength == nil {
		c.MaxLength = &defaultMaxRequestLength
	}

	if c.Gzip == nil {
		c.Gzip = &Gzip{
			MaxLength: &defaultGzipMaxContentLength,
		}
	}
	if c.Gzip.MaxLength == nil {
		c.Gzip.MaxLength = &defaultGzipMaxContentLength
	}

	if *c.MaxLength <= 0 {
		return fmt.Errorf("invalid value for server.decoding.max_length field, should be a positive number")
	}
	if *c.Gzip.MaxLength <= 0 {
		return fmt.Errorf("invalid value for server.decoding.gzip.max_length field, should be a positive number")
	}

	return nil
}
