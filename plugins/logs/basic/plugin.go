// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package basic

import (
	"context"
	"encoding/json"

	log "github.com/sirupsen/logrus"

	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/logs"
	"github.com/open-policy-agent/opa/util"
)

type config struct {
}

// DecisionLogger is a local decision logger implementing logs.Logger
// which will use Logrus to output logs.
type DecisionLogger struct {
	config config
}

// Start the plugin
func (p *DecisionLogger) Start(ctx context.Context) error {
	// No-op.
	return nil
}

// Stop the plugin
func (p *DecisionLogger) Stop(ctx context.Context) {
	// No-op.
}

// Reconfigure the plugin
func (p *DecisionLogger) Reconfigure(ctx context.Context, cfg interface{}) {
	p.config = cfg.(config)
}

// Log an event (decision)
func (p *DecisionLogger) Log(ctx context.Context, event logs.EventV1) error {
	eventBuf, err := json.Marshal(&event)
	if err != nil {
		return err
	}
	fields := log.Fields{}
	err = json.Unmarshal(eventBuf, &fields)
	if err != nil {
		return err
	}
	log.WithFields(fields).Info("Decision Log")
	return nil
}

// DecisionLoggerFactory implements the plugins.Factory interface for the basic decision logger
type DecisionLoggerFactory struct{}

// New creates an instance of the basic DecisionLogger
func (f *DecisionLoggerFactory) New(_ *plugins.Manager, cfg interface{}) plugins.Plugin {
	logger := &DecisionLogger{
		config: cfg.(config),
	}

	return logger
}

// Validate parses the config for the plugin
func (f *DecisionLoggerFactory) Validate(_ *plugins.Manager, cfg []byte) (interface{}, error) {
	parsedConfig := config{}
	err := util.Unmarshal(cfg, &parsedConfig)
	if err != nil {
		return nil, err
	}

	return parsedConfig, err
}
