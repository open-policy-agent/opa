// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package sdk

import (
	"fmt"
	"io"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/sirupsen/logrus"

	"github.com/open-policy-agent/opa/v1/hooks"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

// DefaultOptions allows providing default `Options` to be used in sdk.New().
var defaultOptions Options
var defaultOptsMtx sync.Mutex

// SetDefaultOptions allows providing default `Options` to be used in sdk.New().
// Note that due to the way booleans work, V1Compatible and V0Compatible is ignored in default options,
// use RegoVersion instead.
func SetDefaultOptions(o Options) {
	defaultOptsMtx.Lock()
	defaultOptions = o
	defaultOptsMtx.Unlock()
}

// Options contains parameters to setup and configure OPA.
type Options struct {
	Store         storage.Store
	Logger        logging.Logger
	ConsoleLogger logging.Logger
	Config        io.Reader
	Ready         chan struct{}
	Plugins       map[string]plugins.Factory
	Hooks         hooks.Hooks
	ID            string
	ManagerOpts   []func(manager *plugins.Manager)
	config        []byte
	RegoVersion   ast.RegoVersion
	V0Compatible  bool
	V1Compatible  bool
	block         bool
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
