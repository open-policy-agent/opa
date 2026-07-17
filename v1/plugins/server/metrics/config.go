package metrics

import (
	"context"
	_ "embed"

	"github.com/open-policy-agent/opa/internal/configpolicy"
	"github.com/open-policy-agent/opa/v1/config"
)

var defaultHTTPRequestBuckets = []float64{
	1e-6, // 1 microsecond
	5e-6,
	1e-5,
	5e-5,
	1e-4,
	5e-4,
	1e-3, // 1 millisecond
	0.01,
	0.1,
	1, // 1 second
}

//go:embed validate.rego
var validationModule string

var validationPolicy = configpolicy.New(
	"opa/config/server/metrics/validate.rego",
	validationModule,
	"data.opa.config.server.metrics = x",
)

func init() {
	config.RegisterConfigSpec(config.SpecsFromStruct[Config]("server", "metrics")...)
}

// Config represents the configuration for the Server.Metrics settings
type Config struct {
	Prom *Prom `json:"prom,omitempty"`
}

// Prom represents the configuration for the Server.Metrics.Prom settings
type Prom struct {
	HTTPRequestDurationSeconds *HTTPRequestDurationSeconds `json:"http_request_duration_seconds,omitempty"`
}

// HTTPRequestDurationSeconds represents the configuration for the Server.Metrics.Prom.HTTPRequestDurationSeconds settings
type HTTPRequestDurationSeconds struct {
	Buckets []float64 `json:"buckets,omitempty"` // the float64 array of buckets representing seconds or division of a second
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
	var result Config
	if _, err := configpolicy.EvalConfigInto(context.TODO(), validationPolicy, b.raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
