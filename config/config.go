// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package config implements OPA configuration file parsing and validation.
package config

import (
	v1 "github.com/open-policy-agent/opa/v1/config"
)

// Config represents the configuration file that OPA can be started with.
type Config = v1.Config

// ParseConfig returns a valid Config object with defaults injected. The id
// and version parameters will be set in the labels map.
func ParseConfig(raw []byte, id string) (*Config, error) {
	return v1.ParseConfig(raw, id)
}
