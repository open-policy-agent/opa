// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package status implements status reporting.
package status

import (
	"context"
	"fmt"
	"net/http"

	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/plugins/bundle"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// UpdateRequestV1 represents the status update message that OPA sends to
// remote HTTP endpoints.
type UpdateRequestV1 struct {
	Labels map[string]string `json:"labels"`
	Bundle bundle.Status     `json:"bundle"`
}

// Plugin implements status reporting. Updates can be triggered by the caller.
type Plugin struct {
	manager *plugins.Manager
	config  Config
	update  chan bundle.Status
	stop    chan chan struct{}
}

// Config contains configuration for the plugin.
type Config struct {
	Service       string `json:"service"`
	PartitionName string `json:"partition_name,omitempty"`
}

func (c *Config) validateAndInjectDefaults(services []string) error {

	found := false

	for _, svc := range services {
		if svc == c.Service {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("invalid service name %q in status", c.Service)
	}

	return nil
}

// New returns a new Plugin with the given config.
func New(config []byte, manager *plugins.Manager) (*Plugin, error) {

	var parsedConfig Config

	if err := util.Unmarshal(config, &parsedConfig); err != nil {
		return nil, err
	}

	if err := parsedConfig.validateAndInjectDefaults(manager.Services()); err != nil {
		return nil, err
	}

	plugin := &Plugin{
		manager: manager,
		config:  parsedConfig,
		update:  make(chan bundle.Status),
		stop:    make(chan chan struct{}),
	}

	return plugin, nil
}

// Start starts the plugin.
func (p *Plugin) Start(ctx context.Context) error {
	go p.loop()
	return nil
}

// Stop stops the plugin.
func (p *Plugin) Stop(ctx context.Context) {
	done := make(chan struct{})
	p.stop <- done
	_ = <-done
}

// Update notifies the plugin with a new bundle.Status.
func (p *Plugin) Update(status bundle.Status) {
	p.update <- status
}

func (p *Plugin) loop() {

	ctx, cancel := context.WithCancel(context.Background())

	for {
		select {
		case status := <-p.update:
			err := p.oneShot(ctx, status)
			if err != nil {
				p.logError("%v.", err)
			} else {
				p.logInfo("Status update sent successfully.")
			}

		case done := <-p.stop:
			cancel()
			done <- struct{}{}
			return
		}
	}
}

func (p *Plugin) oneShot(ctx context.Context, status bundle.Status) error {

	req := UpdateRequestV1{
		Labels: p.manager.Labels,
		Bundle: status,
	}

	resp, err := p.manager.Client(p.config.Service).
		WithJSON(req).
		Do(ctx, "POST", fmt.Sprintf("/status/%v", p.config.PartitionName))

	if err != nil {
		return errors.Wrap(err, "Status update failed")
	}

	defer util.Close(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("Status update failed, server replied with not found")
	case http.StatusUnauthorized:
		return fmt.Errorf("Status update failed, server replied with not authorized")
	default:
		return fmt.Errorf("Status update failed, server replied with HTTP %v", resp.StatusCode)
	}
}

func (p *Plugin) logError(fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields()).Errorf(fmt, a...)
}

func (p *Plugin) logInfo(fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields()).Infof(fmt, a...)
}

func (p *Plugin) logDebug(fmt string, a ...interface{}) {
	logrus.WithFields(p.logrusFields()).Debugf(fmt, a...)
}

func (p *Plugin) logrusFields() logrus.Fields {
	return logrus.Fields{
		"plugin": "status",
	}
}
