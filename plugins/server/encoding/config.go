package encoding

import (
	"compress/gzip"
	"fmt"

	"github.com/open-policy-agent/opa/util"
)

var defaultGzipMinLength = 1024
var defaultGzipCompressionLevel = gzip.BestCompression

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
	if b.raw == nil {
		defaultConfig := &Config{
			Gzip: &Gzip{
				MinLength:        &defaultGzipMinLength,
				CompressionLevel: &defaultGzipCompressionLevel,
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

func (c *Config) validateAndInjectDefaults() error {
	if c.Gzip == nil {
		c.Gzip = &Gzip{
			MinLength:        &defaultGzipMinLength,
			CompressionLevel: &defaultGzipCompressionLevel,
		}
	}
	if c.Gzip.MinLength == nil {
		c.Gzip.MinLength = &defaultGzipMinLength
	}

	if c.Gzip.CompressionLevel == nil {
		c.Gzip.CompressionLevel = &defaultGzipCompressionLevel
	}

	if *c.Gzip.MinLength <= 0 {
		return fmt.Errorf("invalid value for server.encoding.gzip.min_length field, should be a positive number")
	}

	acceptedCompressionLevels := map[int]bool{
		gzip.NoCompression:   true,
		gzip.BestSpeed:       true,
		gzip.BestCompression: true,
	}
	_, compressionLevelAccepted := acceptedCompressionLevels[*c.Gzip.CompressionLevel]
	if !compressionLevelAccepted {
		return fmt.Errorf("invalid value for server.encoding.gzip.compression_level field, accepted values are 0, 1 or 9")
	}

	return nil
}
