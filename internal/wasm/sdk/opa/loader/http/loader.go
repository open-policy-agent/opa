// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package http

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa/errors"
)

const (
	// MinRetryDelay determines the minimum retry interval in case
	// of an error.
	MinRetryDelay = 100 * time.Millisecond

	// DefaultMinDelay is the default minimum re-downloading
	// interval in case of a previously successful download.
	DefaultMinDelay = 60 * time.Second

	// DefaultMaxDelay is the default maximum re-downloading
	// interval in case of a previously successful download.
	DefaultMaxDelay = 120 * time.Second
)

// Loader downloads a bundle over HTTP. If started, it downloads the
// bundle periodically until closed.
type Loader struct {
	configErr      error // Delayed configuration error, if any.
	initialized    bool
	pd             policyData
	client         *http.Client
	url            string
	tag            string
	minDelay       time.Duration
	maxDelay       time.Duration
	closing        chan struct{} // Signal the request to stop the poller.
	closed         chan struct{} // Signals the successful stopping of the poller.
	logError       func(error)
	prepareRequest func(*http.Request) error
	mutex          sync.Mutex
}

// policyData captures the functions used in setting the policy and data.
type policyData interface {
	SetPolicyData(ctx context.Context, policy []byte, data *interface{}) error
}

// New constructs a new HTTP loader periodically downloading a bundle
// over HTTP.
func New(o *opa.OPA) *Loader {
	return newLoader(o)
}

// newLoader constructs a new HTTP loader. This is for tests.
func newLoader(pd policyData) *Loader {
	return &Loader{
		pd:             pd,
		client:         http.DefaultClient,
		minDelay:       DefaultMinDelay,
		maxDelay:       DefaultMaxDelay,
		logError:       func(error) {},
		prepareRequest: func(*http.Request) error { return nil },
	}
}

// Init initializes the loader after its construction and
// configuration. If invalid config, will return ErrInvalidConfig.
func (l *Loader) Init() (*Loader, error) {
	if l.configErr != nil {
		return nil, l.configErr
	}

	if l.url == "" {
		return nil, errors.New(errors.InvalidConfigErr, "missing url")
	}

	l.initialized = true
	return l, nil
}

// Start starts the periodic downloads, blocking until the first
// successful download.  If cancelled, will return context.Cancelled.
func (l *Loader) Start(ctx context.Context) error {
	if !l.initialized {
		return errors.New(errors.NotReadyErr, "")
	}

	if err := l.download(ctx); err != nil {
		return err
	}

	l.closing = make(chan struct{})
	l.closed = make(chan struct{})

	go l.poller()

	return nil
}

// Close stops the downloading, releasing all resources.
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

// poller periodically downloads the bundle.
func (l *Loader) poller() {
	defer close(l.closed)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-l.closing
		cancel()
	}()

	for {
		if err := l.download(ctx); err != nil {
			break
		}

		select {
		case <-time.After(time.Duration(float64((l.maxDelay-l.minDelay))*rand.Float64()) + l.minDelay):
		case <-ctx.Done():
			return
		}
	}
}

// download blocks until a bundle has been download successfully or
// the context is cancelled. No other error besides context.Canceled
// is ever returned.
func (l *Loader) download(ctx context.Context) error {
	for retry := 0; true; retry++ {
		if err := l.Load(ctx); err == context.Canceled {
			return err
		} else if err != nil {
			l.logError(err)
		} else {
			break
		}

		select {
		case <-time.After(defaultBackoff(float64(MinRetryDelay), float64(l.maxDelay), retry)):
		case <-ctx.Done():
			return context.Canceled
		}
	}

	return nil
}

// Load downloads the bundle from a remote location and installs
// it. The possible returned errors are ErrInvalidBundle (in case of
// an error in downloading or opening the bundle) and the ones
// SetPolicyData of OPA returns.
func (l *Loader) Load(ctx context.Context) error {
	if !l.initialized {
		return errors.New(errors.NotReadyErr, "")
	}

	l.mutex.Lock()
	defer l.mutex.Unlock()

	bundle, err := l.get(ctx, "")
	if err != nil {
		return errors.New(errors.InvalidBundleErr, err.Error())
	}

	if len(bundle.WasmModules) == 0 {
		return errors.New(errors.InvalidBundleErr, "missing wasm")
	}

	var data *interface{}
	if bundle.Data != nil {
		var v interface{} = bundle.Data
		data = &v
	}

	return l.pd.SetPolicyData(ctx, bundle.WasmModules[0].Raw, data)
}

// get executes HTTP GET.
func (l *Loader) get(ctx context.Context, tag string) (*bundle.Bundle, error) {
	req, err := http.NewRequest(http.MethodGet, l.url, nil)
	if err != nil {
		return nil, err
	}

	if tag != "" {
		req.Header.Add("If-None-Match", tag)
	}

	req = req.WithContext(ctx)
	if err := l.prepareRequest(req); err != nil {
		return nil, err
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer l.close(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		// TODO: Cut the dependency to the OPA bundle package.

		b, err := bundle.NewReader(resp.Body).Read()
		if err != nil {
			return nil, err
		}

		l.tag = resp.Header.Get("ETag")
		return &b, nil

	case http.StatusNotModified:
		return nil, nil
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("not authorized (401)")
	case http.StatusForbidden:
		return nil, fmt.Errorf("forbidden (403)")
	case http.StatusNotFound:
		return nil, fmt.Errorf("not found (404)")
	default:
		return nil, fmt.Errorf("unknown HTTP status %v", resp.StatusCode)
	}
}

// close closes the HTTP response gracefully, first draining it, to
// avoid resource leaks.
func (l *Loader) close(resp *http.Response) {
	_, _ = io.Copy(io.Discard, resp.Body) // Ignore errors.
	_ = resp.Body.Close()
}
