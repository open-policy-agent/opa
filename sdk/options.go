// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"

	"github.com/open-policy-agent/opa/hooks"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/plugins"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
)

// Options contains parameters to setup and configure OPA.
type Options struct {

	// Config provides the OPA configuration for this instance. The config can
	// be supplied as a YAML or JSON byte stream. See
	// https://www.openpolicyagent.org/docs/latest/configuration/ for detailed
	// description of the supported configuration.
	Config io.Reader

	// Logger sets the logging implementation to use for standard logs emitted
	// by OPA. By default, standard logging is disabled.
	Logger logging.Logger

	// ConsoleLogger sets the logging implementation to use for emitting Status
	// and Decision Logs to the console. By default, console logging is enabled.
	ConsoleLogger logging.Logger

	// Ready sets a channel to notify when the OPA instance is ready. If this
	// field is not set, the New() function will block until ready. The channel
	// is closed to signal readiness.
	Ready chan struct{}

	// Plugins provides a set of plugins.Factory instances that will be
	// registered with the OPA SDK instance.
	Plugins map[string]plugins.Factory

	// ID provides an option to set a static ID for the OPA system, avoiding
	// the need to generate a random one at initialization. Setting a static ID
	// is recommended, as it makes it easier to track the system over time.
	ID string

	// Store sets the store to be used by the SDK instance. If nil, it'll use OPA's
	// inmem store.
	Store storage.Store

	// Hooks allows hooking into the internals of SDK operations (TODO(sr): find better words)
	Hooks hooks.Hooks

	config []byte
	block  bool
}

func (o *Options) init() error {

	if o.Ready == nil {
		o.Ready = make(chan struct{})
		o.block = true
	}

	if o.Logger == nil {
		o.Logger = logging.NewNoOpLogger()
	}

	if o.ConsoleLogger == nil {
		l := logging.New()
		l.SetFormatter(&logrus.JSONFormatter{})
		o.ConsoleLogger = l
	}

	if o.Config == nil {
		o.config = []byte("{}")
	} else {
		bs, err := io.ReadAll(o.Config)
		if err != nil {
			return err
		}
		o.config = bs
	}

	if o.Store == nil {
		o.Store = inmem.New()
	}

	if err := o.Hooks.Validate(); err != nil {
		return fmt.Errorf("hooks: %w", err)
	}

	return nil
}

// ConfigOptions contains parameters to (re-)configure OPA.
type ConfigOptions struct {

	// Config provides the OPA configuration for this instance. The config can
	// be supplied as a YAML or JSON byte stream. See
	// https://www.openpolicyagent.org/docs/latest/configuration/ for detailed
	// description of the supported configuration.
	Config io.Reader

	// Ready sets a channel to notify when the OPA instance is ready. If this
	// field is not set, the Configure() function will block until ready. The
	// channel is closed to signal readiness.
	Ready chan struct{}

	config []byte
	block  bool
}

func (o *ConfigOptions) init() error {

	if o.Ready == nil {
		o.Ready = make(chan struct{})
		o.block = true
	}

	bs, err := io.ReadAll(o.Config)
	if err != nil {
		return err
	}

	o.config = bs
	return nil
}
