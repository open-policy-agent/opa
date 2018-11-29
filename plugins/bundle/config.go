// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"fmt"

	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/util"
)

// ParseConfig validates the config and injects default values.
func ParseConfig(config []byte, services []string) (*Config, error) {

	if config == nil {
		return nil, nil
	}

	var parsedConfig Config

	if err := util.Unmarshal(config, &parsedConfig); err != nil {
		return nil, err
	}

	if err := parsedConfig.validateAndInjectDefaults(services); err != nil {
		return nil, err
	}

	return &parsedConfig, nil
}

// Config represents configuration the plguin.
type Config struct {
	download.Config

	Name    string `json:"name"`
	Service string `json:"service"`
}

func (c *Config) validateAndInjectDefaults(services []string) error {

	if c.Name == "" {
		return fmt.Errorf("invalid bundle name %q", c.Name)
	}

	if c.Service == "" && len(services) != 0 {
		c.Service = services[0]
	} else {
		found := false

		for _, svc := range services {
			if svc == c.Service {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("invalid service name %q in bundle %q", c.Service, c.Name)
		}
	}

	return c.ValidateAndInjectDefaults()
}
