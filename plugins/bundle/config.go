// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"fmt"
	"path"
	"strings"

	"github.com/open-policy-agent/opa/bundle"

	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/util"
)

// ParseConfig validates the config and injects default values. This is
// for the legacy single bundle configuration. This will add the bundle
// to the `Bundles` map to provide compatibility with newer clients.
// Deprecated: Use `ParseBundlesConfig` with `bundles` OPA config option instead
func ParseConfig(config []byte, services []string) (*Config, error) {
	if config == nil {
		return nil, nil
	}

	var parsedConfig Config

	if err := util.Unmarshal(config, &parsedConfig); err != nil {
		return nil, err
	}

	if err := parsedConfig.validateAndInjectDefaults(services, nil); err != nil {
		return nil, err
	}

	// For forwards compatibility make a new Source as if the bundle
	// was configured with `bundles` in the newer format.
	parsedConfig.Bundles = map[string]*Source{
		parsedConfig.Name: {
			Config:         parsedConfig.Config,
			Service:        parsedConfig.Service,
			Resource:       parsedConfig.generateLegacyResourcePath(),
			Signing:        nil,
			Persist:        false,
			SizeLimitBytes: bundle.DefaultSizeLimitBytes,
		},
	}

	return &parsedConfig, nil
}

// ParseBundlesConfig validates the config and injects default values for
// the defined `bundles`. This expects a map of bundle names to resource
// configurations.
func ParseBundlesConfig(config []byte, services []string) (*Config, error) {
	return NewConfigBuilder().WithBytes(config).WithServices(services).Parse()
}

// NewConfigBuilder returns a new ConfigBuilder to build and parse the bundle config
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{}
}

// WithBytes sets the raw bundle config
func (b *ConfigBuilder) WithBytes(config []byte) *ConfigBuilder {
	b.raw = config
	return b
}

// WithServices sets the services that implement control plane APIs
func (b *ConfigBuilder) WithServices(services []string) *ConfigBuilder {
	b.services = services
	return b
}

// WithKeyConfigs sets the public keys to verify a signed bundle
func (b *ConfigBuilder) WithKeyConfigs(keys map[string]*bundle.KeyConfig) *ConfigBuilder {
	b.keys = keys
	return b
}

// Parse validates the config and injects default values for the defined `bundles`.
func (b *ConfigBuilder) Parse() (*Config, error) {
	if b.raw == nil {
		return nil, nil
	}

	var bundleConfigs map[string]*Source

	if err := util.Unmarshal(b.raw, &bundleConfigs); err != nil {
		return nil, err
	}

	// Build a `Config` out of the parsed map
	c := Config{Bundles: map[string]*Source{}}
	for name, source := range bundleConfigs {
		if source != nil {
			c.Bundles[name] = source
		}
	}

	err := c.validateAndInjectDefaults(b.services, b.keys)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// ConfigBuilder assists in the construction of the plugin configuration.
type ConfigBuilder struct {
	raw      []byte
	services []string
	keys     map[string]*bundle.KeyConfig
}

// Config represents the configuration of the plugin.
// The Config can define a single bundle source or a map of
// `Source` objects defining where/how to download bundles. The
// older single bundle configuration is deprecated and will be
// removed in the future in favor of the `Bundles` map.
type Config struct {
	download.Config // Deprecated: Use `Bundles` map instead

	Bundles map[string]*Source

	Name    string  `json:"name"`    // Deprecated: Use `Bundles` map instead
	Service string  `json:"service"` // Deprecated: Use `Bundles` map instead
	Prefix  *string `json:"prefix"`  // Deprecated: Use `Bundles` map instead
}

// Source is a configured bundle source to download bundles from
type Source struct {
	download.Config

	Service        string                     `json:"service"`
	Resource       string                     `json:"resource"`
	Signing        *bundle.VerificationConfig `json:"signing"`
	Persist        bool                       `json:"persist"`
	SizeLimitBytes int64                      `json:"size_limit_bytes"`
}

// IsMultiBundle returns whether or not the config is the newer multi-bundle
// style config that uses `bundles` instead of top level bundle information.
// If/when we drop support for the older style config we can remove this too.
func (c *Config) IsMultiBundle() bool {
	// If a `Name` was set then the config is in "legacy" single plugin mode
	return c.Name == ""
}

func (c *Config) validateAndInjectDefaults(services []string, keys map[string]*bundle.KeyConfig) error {
	if c.Bundles == nil {
		return c.validateAndInjectDefaultsLegacy(services)
	}

	for name, source := range c.Bundles {
		if source.Resource == "" {
			source.Resource = path.Join(defaultBundlePathPrefix, name)
		}

		var err error

		if source.Signing != nil {
			err = source.Signing.ValidateAndInjectDefaults(keys)
			if err != nil {
				return fmt.Errorf("invalid configuration for bundle %q: %s", name, err.Error())
			}
		} else {
			if len(keys) > 0 {
				source.Signing = bundle.NewVerificationConfig(keys, "", "", nil)
			}
		}

		source.Service, err = c.getServiceFromList(source.Service, services)
		if err == nil {
			err = source.Config.ValidateAndInjectDefaults()
		}
		if err != nil {
			return fmt.Errorf("invalid configuration for bundle %q: %s", name, err.Error())
		}

		if source.SizeLimitBytes <= 0 {
			source.SizeLimitBytes = bundle.DefaultSizeLimitBytes
		}
	}

	return nil
}

func (c *Config) validateAndInjectDefaultsLegacy(services []string) error {
	if c.Name == "" {
		return fmt.Errorf("invalid bundle name %q", c.Name)
	}

	if c.Prefix == nil {
		s := defaultBundlePathPrefix
		c.Prefix = &s
	}

	var err error
	c.Service, err = c.getServiceFromList(c.Service, services)
	if err == nil {
		err = c.Config.ValidateAndInjectDefaults()
	}

	if err != nil {
		return fmt.Errorf("invalid configuration for bundle %q: %s", c.Name, err.Error())
	}

	return nil
}

func (c *Config) getServiceFromList(service string, services []string) (string, error) {
	if service == "" && len(services) != 0 {
		return services[0], nil
	}
	for _, svc := range services {
		if svc == service {
			return service, nil
		}
	}
	return service, fmt.Errorf("service name %q not found", service)
}

// generateLegacyDownloadPath will return the Resource path
// from the older style prefix+name configuration.
func (c *Config) generateLegacyResourcePath() string {
	joined := path.Join(*c.Prefix, c.Name)
	return strings.TrimPrefix(joined, "/")
}

const (
	defaultBundlePathPrefix = "bundles"
)
