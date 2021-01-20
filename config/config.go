// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package config implements OPA configuration file parsing and validation.
package config

import (
	"encoding/json"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/ref"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
)

// Config represents the configuration file that OPA can be started with.
type Config struct {
	Services                     json.RawMessage            `json:"services"`
	Labels                       map[string]string          `json:"labels"`
	Discovery                    json.RawMessage            `json:"discovery"`
	Bundle                       json.RawMessage            `json:"bundle"` // Deprecated: Use `bundles` instead
	Bundles                      json.RawMessage            `json:"bundles"`
	DecisionLogs                 json.RawMessage            `json:"decision_logs"`
	Status                       json.RawMessage            `json:"status"`
	Plugins                      map[string]json.RawMessage `json:"plugins"`
	Keys                         json.RawMessage            `json:"keys"`
	DefaultDecision              *string                    `json:"default_decision"`
	DefaultAuthorizationDecision *string                    `json:"default_authorization_decision"`
	Caching                      json.RawMessage            `json:"caching"`
}

// ParseConfig returns a valid Config object with defaults injected. The id
// and version parameters will be set in the labels map.
func ParseConfig(raw []byte, id string) (*Config, error) {
	var result Config
	if err := util.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, result.validateAndInjectDefaults(id)
}

// PluginsEnabled returns true if one or more plugin features are enabled.
func (c Config) PluginsEnabled() bool {
	return c.Bundle != nil || c.Bundles != nil || c.DecisionLogs != nil || c.Status != nil || len(c.Plugins) > 0
}

// DefaultDecisionRef returns the default decision as a reference.
func (c Config) DefaultDecisionRef() ast.Ref {
	r, _ := ref.ParseDataPath(*c.DefaultDecision)
	return r
}

// DefaultAuthorizationDecisionRef returns the default authorization decision
// as a reference.
func (c Config) DefaultAuthorizationDecisionRef() ast.Ref {
	r, _ := ref.ParseDataPath(*c.DefaultAuthorizationDecision)
	return r
}

func (c *Config) validateAndInjectDefaults(id string) error {

	if c.DefaultDecision == nil {
		s := defaultDecisionPath
		c.DefaultDecision = &s
	}

	_, err := ref.ParseDataPath(*c.DefaultDecision)
	if err != nil {
		return err
	}

	if c.DefaultAuthorizationDecision == nil {
		s := defaultAuthorizationDecisionPath
		c.DefaultAuthorizationDecision = &s
	}

	_, err = ref.ParseDataPath(*c.DefaultAuthorizationDecision)
	if err != nil {
		return err
	}

	if c.Labels == nil {
		c.Labels = map[string]string{}
	}

	c.Labels["id"] = id
	c.Labels["version"] = version.Version

	return nil
}

const (
	defaultDecisionPath              = "/system/main"
	defaultAuthorizationDecisionPath = "/system/authz/allow"
)
