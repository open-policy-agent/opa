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
	Name            *string `json:"name"`               // name of the discovery bundle
	Prefix          *string `json:"prefix,omitempty"`   // Deprecated: use `Resource` instead.
	Decision        *string `json:"decision"`           // the name of the query to run on the bundle to get the config
	Service         string  `json:"service"`            // the name of the service used to download discovery bundle from
	Resource        *string `json:"resource,omitempty"` // the resource path which will be downloaded from the service

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

	if c.Resource != nil {
		c.path = *c.Resource
	} else {
		if c.Prefix == nil {
			s := defaultDiscoveryPathPrefix
			c.Prefix = &s
		}

		c.path = fmt.Sprintf("%v/%v", strings.Trim(*c.Prefix, "/"), strings.Trim(*c.Name, "/"))
	}

	service, err := c.getServiceFromList(c.Service, services)
	if err != nil {
		return fmt.Errorf("invalid configuration for decision service: %s", err.Error())
	}

	c.service = service

	decision := c.Decision

	if decision == nil {
		decision = c.Name
	}

	c.query = fmt.Sprintf("%v.%v", ast.DefaultRootDocument, strings.Replace(strings.Trim(*decision, "/"), "/", ".", -1))

	return c.Config.ValidateAndInjectDefaults()
}

func (c *Config) getServiceFromList(service string, services []string) (string, error) {
	if service == "" {
		if len(services) != 1 {
			return "", fmt.Errorf("more than one service is defined")
		}
		return services[0], nil
	}
	for _, svc := range services {
		if svc == service {
			return service, nil
		}
	}
	return service, fmt.Errorf("service name %q not found", service)
}

const (
	defaultDiscoveryPathPrefix = "bundles"
)
