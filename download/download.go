// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package download implements low-level OPA bundle downloading.
package download

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"path"
	"time"

	"github.com/open-policy-agent/opa/sdk"

	"github.com/pkg/errors"

	"github.com/open-policy-agent/opa/metrics"

	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/plugins/rest"
	"github.com/open-policy-agent/opa/util"
)

const (
	minRetryDelay = time.Millisecond * 100
)

// Update contains the result of a download. If an error occurred, the Error
// field will be non-nil. If a new bundle is available, the Bundle field will
// be non-nil.
type Update struct {
	ETag    string
	Bundle  *bundle.Bundle
	Error   error
	Metrics metrics.Metrics
}

// Downloader implements low-level OPA bundle downloading. Downloader can be
// started and stopped. After starting, the downloader will request bundle
// updates from the remote HTTP endpoint that the client is configured to
// connect to.
type Downloader struct {
	config         Config                        // downloader configuration for tuning polling and other downloader behaviour
	client         rest.Client                   // HTTP client to use for bundle downloading
	path           string                        // path to use in bundle download request
	stop           chan chan struct{}            // used to signal plugin to stop running
	f              func(context.Context, Update) // callback function invoked when download updates occur
	etag           string                        // HTTP Etag for caching purposes
	sizeLimitBytes *int64                        // max bundle file size in bytes (passed to reader)
	bvc            *bundle.VerificationConfig
	logger         sdk.Logger
}

// New returns a new Downloader that can be started.
func New(config Config, client rest.Client, path string) *Downloader {
	return &Downloader{
		config: config,
		client: client,
		path:   path,
		stop:   make(chan chan struct{}),
		logger: client.Logger(),
	}
}

// WithCallback registers a function f to be called when download updates occur.
func (d *Downloader) WithCallback(f func(context.Context, Update)) *Downloader {
	d.f = f
	return d
}

// WithLogAttrs sets an optional set of key/value pair attributes to include in
// log messages emitted by the downloader.
func (d *Downloader) WithLogAttrs(attrs map[string]interface{}) *Downloader {
	d.logger = d.logger.WithFields(attrs)
	return d
}

// WithBundleVerificationConfig sets the key configuration used to verify a signed bundle
func (d *Downloader) WithBundleVerificationConfig(config *bundle.VerificationConfig) *Downloader {
	d.bvc = config
	return d
}

// WithSizeLimitBytes sets the file size limit for bundles read by this downloader.
func (d *Downloader) WithSizeLimitBytes(n int64) *Downloader {
	d.sizeLimitBytes = &n
	return d
}

// ClearCache resets the etag value on the downloader
func (d *Downloader) ClearCache() {
	d.etag = ""
}

// Start tells the Downloader to begin downloading bundles.
func (d *Downloader) Start(ctx context.Context) {
	go d.loop()
}

// Stop tells the Downloader to stop begin downloading bundles.
func (d *Downloader) Stop(ctx context.Context) {
	done := make(chan struct{})
	d.stop <- done
	_ = <-done
}

func (d *Downloader) loop() {

	ctx, cancel := context.WithCancel(context.Background())

	var retry int

	for {
		err := d.oneShot(ctx)
		var delay time.Duration

		if err == nil {
			min := float64(*d.config.Polling.MinDelaySeconds)
			max := float64(*d.config.Polling.MaxDelaySeconds)
			delay = time.Duration(((max - min) * rand.Float64()) + min)
		} else {
			delay = util.DefaultBackoff(float64(minRetryDelay), float64(*d.config.Polling.MaxDelaySeconds), retry)
		}

		d.logger.Debug("Waiting %v before next download/retry.", delay)
		timer := time.NewTimer(delay)

		select {
		case <-timer.C:
			if err != nil {
				retry++
			} else {
				retry = 0
			}
		case done := <-d.stop:
			cancel()
			done <- struct{}{}
			return
		}
	}
}

func (d *Downloader) oneShot(ctx context.Context) error {
	m := metrics.New()
	b, etag, err := d.download(ctx, m)

	d.etag = etag

	if d.f != nil {
		d.f(ctx, Update{ETag: etag, Bundle: b, Error: err, Metrics: m})
	}

	return err
}

func (d *Downloader) download(ctx context.Context, m metrics.Metrics) (*bundle.Bundle, string, error) {
	d.logger.Debug("Download starting.")

	resp, err := d.client.WithHeader("If-None-Match", d.etag).Do(ctx, "GET", d.path)
	if err != nil {
		return nil, "", errors.Wrap(err, "request failed")
	}

	defer util.Close(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		if resp.Body != nil {
			d.logger.Debug("Download in progress.")
			m.Timer(metrics.RegoLoadBundles).Start()
			defer m.Timer(metrics.RegoLoadBundles).Stop()
			baseURL := path.Join(d.client.Config().URL, d.path)
			loader := bundle.NewTarballLoaderWithBaseURL(resp.Body, baseURL)
			reader := bundle.NewCustomReader(loader).WithMetrics(m).WithBundleVerificationConfig(d.bvc)
			if d.sizeLimitBytes != nil {
				reader = reader.WithSizeLimitBytes(*d.sizeLimitBytes)
			}
			b, err := reader.Read()
			if err != nil {
				return nil, "", err
			}
			return &b, resp.Header.Get("ETag"), nil
		}

		d.logger.Debug("Server replied with empty body.")
		return nil, "", nil

	case http.StatusNotModified:
		etag := resp.Header.Get("ETag")
		if etag == "" {
			etag = d.etag
		}
		return nil, etag, nil
	case http.StatusNotFound:
		return nil, "", fmt.Errorf("server replied with not found")
	case http.StatusUnauthorized:
		return nil, "", fmt.Errorf("server replied with not authorized")
	default:
		return nil, "", fmt.Errorf("server replied with HTTP %v", resp.StatusCode)
	}
}
