// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk

import (
	"fmt"
	"io"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/sirupsen/logrus"

	"github.com/open-policy-agent/opa/v1/hooks"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
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

	// V0Compatible enables v0 compatibility mode when set to true.
	// This is an opt-in to OPA features and behaviors that were enabled by default in OPA v0.x.
	// Takes precedence over V1Compatible.
	V0Compatible bool

	// V1Compatible enables v1 compatibility mode when set to true.
	// This is an opt-in to OPA features and behaviors that will be enabled by default in OPA v1.0 and later.
	// See https://www.openpolicyagent.org/docs/latest/opa-1/ for more information.
	// If V0Compatible is set to true, this field is ignored.
	V1Compatible bool

	// RegoVersion sets the version of the Rego language to use.
	// If V0Compatible or V1Compatible is set to true, this field is ignored.
	RegoVersion ast.RegoVersion

	// ManagerOpts allows customization of the plugin manager.
	// The given options get appended to the list of options already provided by the SDK and eventually
	// overriding them.
	ManagerOpts []func(manager *plugins.Manager)

	config []byte
	block  bool
}

func (o *Options) regoVersion() ast.RegoVersion {
	// v0 takes precedence over v1
	if o.V0Compatible {
		return ast.RegoV0
	}
	if o.V1Compatible {
		return ast.RegoV1
	}
	return o.RegoVersion
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
