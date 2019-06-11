// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package discovery

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/download"
	"github.com/open-policy-agent/opa/util"
)

// Config represents the configuration for the discovery feature.
type Config struct {
	download.Config         // bundle downloader configuration
	Name            *string `json:"name"`     // name of the discovery bundle
	Prefix          *string `json:"prefix"`   // path prefix for downloader
	Decision        *string `json:"decision"` // the name of the query to run on the bundle to get the config

	service string
	path    string
	query   string
}

// ParseConfig returns a valid Config object with defaults injected.
func ParseConfig(bs []byte, services []string) (*Config, error) {

	if bs == nil {
		return nil, nil
	}

	var result Config

	if err := util.Unmarshal(bs, &result); err != nil {
		return nil, err
	}

	return &result, result.validateAndInjectDefaults(services)
}

func (c *Config) validateAndInjectDefaults(services []string) error {

	if c.Name == nil {
		return fmt.Errorf("missing required discovery.name field")
	}

	if c.Prefix == nil {
		s := defaultDiscoveryPathPrefix
		c.Prefix = &s
	}

	if len(services) != 1 {
		return fmt.Errorf("discovery requires exactly one service")
	}

	decision := c.Decision

	if decision == nil {
		decision = c.Name
	}

	c.service = services[0]
	c.path = fmt.Sprintf("%v/%v", strings.Trim(*c.Prefix, "/"), strings.Trim(*c.Name, "/"))
	c.query = fmt.Sprintf("%v.%v", ast.DefaultRootDocument, strings.Replace(strings.Trim(*decision, "/"), "/", ".", -1))

	return c.Config.ValidateAndInjectDefaults()
}

const (
	defaultDiscoveryPathPrefix  = "bundles"
	defaultDiscoveryQueryPrefix = "data"
)
