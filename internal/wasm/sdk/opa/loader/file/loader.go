// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package file

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa/errors"

	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
)

const (
	// DefaultInterval for re-loading the bundle file.
	DefaultInterval = time.Minute
)

// Loader loads a bundle from a file. If started, it loads the bundle
// periodically until closed.
type Loader struct {
	configErr   error // Delayed configuration error, if any.
	initialized bool
	pd          policyData
	filename    string
	interval    time.Duration
	closing     chan struct{} // Signal the request to stop the poller.
	closed      chan struct{} // Signals the successful stopping of the poller.
	logError    func(error)
	mutex       sync.Mutex
}

// policyData captures the functions used in setting the policy and data.
type policyData interface {
	SetPolicyData(policy []byte, data *interface{}) error
}

// New constructs a new file loader periodically reloading the bundle
// from a file.
func New(opa *opa.OPA) *Loader {
	return new(opa)
}

// new constucts a new file loader. This is for tests.
func new(pd policyData) *Loader {
	return &Loader{
		pd:       pd,
		interval: DefaultInterval,
		logError: func(error) {},
	}
}

// Init initializes the loader after its construction and
// configuration. If invalid config, will return ErrInvalidConfig.
func (l *Loader) Init() (*Loader, error) {
	if l.configErr != nil {
		return nil, l.configErr
	}

	if l.filename == "" {
		return nil, fmt.Errorf("filename: %w", errors.ErrInvalidConfig)
	}

	l.initialized = true
	return l, nil
}

// Start starts the periodic loading byt calling Load, failing if the
// bundle loading fails.
func (l *Loader) Start(ctx context.Context) error {
	if !l.initialized {
		return errors.ErrNotReady
	}

	if err := l.Load(ctx); err != nil {
		return err
	}

	l.closing = make(chan struct{})
	l.closed = make(chan struct{})

	go l.poller()

	return nil
}

// Close stops the loading, releasing all resources.
func (l *Loader) Close() {
	if !l.initialized {
		return
	}

	if l.closing == nil {
		return
	}

	close(l.closing)
	<-l.closed

	l.closing = nil
	l.closed = nil
}

// Load loads the bundle from a file and installs it. The possible
// returned errors are ErrInvalidBundle (in case of an error in
// loading or opening the bundle) and the ones SetPolicyData of OPA
// returns.
func (l *Loader) Load(ctx context.Context) error {
	if !l.initialized {
		return errors.ErrNotReady
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	f, err := os.Open(l.filename)
	if err != nil {
		return fmt.Errorf("%v: %w", err, errors.ErrInvalidBundle)
	}

	defer f.Close()

	// TODO: Cut the dependency to the OPA bundle package.

	b, err := bundle.NewReader(f).Read()
	if err != nil {
		return fmt.Errorf("%v: %w", err, errors.ErrInvalidBundle)
	}

	if len(b.WasmModules) == 0 {
		return fmt.Errorf("missing wasm: %w", errors.ErrInvalidBundle)
	}

	var data *interface{}
	if b.Data != nil {
		var v interface{} = b.Data
		data = &v
	}

	return l.pd.SetPolicyData(b.WasmModules[0].Raw, data)
}

// poller periodically downloads the bundle.
func (l *Loader) poller() {
	defer close(l.closed)

	for {
		if err := l.Load(context.Background()); err != nil {
			l.logError(err)
		}

		select {
		case <-time.After(l.interval):
		case <-l.closing:
			return
		}
	}
}
