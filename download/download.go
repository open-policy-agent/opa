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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

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
	ETag   string
	Bundle *bundle.Bundle
	Error  error
}

// Downloader implements low-level OPA bundle downloading. Downloader can be
// started and stopped. After starting, the downloader will request bundle
// updates from the remote HTTP endpoint that the client is configured to
// connect to.
type Downloader struct {
	config   Config                        // downloader configuration for tuning polling and other downloader behaviour
	client   rest.Client                   // HTTP client to use for bundle downloading
	path     string                        // path to use in bundle download request
	stop     chan chan struct{}            // used to signal plugin to stop running
	f        func(context.Context, Update) // callback function invoked when download updates occur
	logAttrs [][2]string                   // optional attributes to include in log messages
	etag     string                        // HTTP Etag for caching purposes
}

// New returns a new Downloader that can be started.
func New(config Config, client rest.Client, path string) *Downloader {
	return &Downloader{
		config: config,
		client: client,
		path:   path,
		stop:   make(chan chan struct{}),
	}
}

// WithCallback registers a function f to be called when download updates occur.
func (d *Downloader) WithCallback(f func(context.Context, Update)) *Downloader {
	d.f = f
	return d
}

// WithLogAttrs sets an optional set of key/value pair attributes to include in
// log messages emitted by the downloader.
func (d *Downloader) WithLogAttrs(attrs [][2]string) *Downloader {
	d.logAttrs = attrs
	return d
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

		d.logDebug("Waiting %v before next download/retry.", delay)
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

	b, etag, err := d.download(ctx)

	if d.f != nil {
		d.f(ctx, Update{ETag: etag, Bundle: b, Error: err})
	}

	d.etag = etag

	return err
}

func (d *Downloader) download(ctx context.Context) (*bundle.Bundle, string, error) {

	d.logDebug("Download starting.")

	resp, err := d.client.WithHeader("If-None-Match", d.etag).Do(ctx, "GET", d.path)
	if err != nil {
		return nil, "", errors.Wrap(err, "request failed")
	}

	defer util.Close(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		if resp.Body != nil {
			d.logDebug("Download in progress.")
			b, err := bundle.NewReader(resp.Body).Read()
			if err != nil {
				return nil, "", err
			}
			return &b, resp.Header.Get("ETag"), nil
		}

		d.logDebug("Server replied with empty body.")
		return nil, "", nil

	case http.StatusNotModified:
		return nil, resp.Header.Get("ETag"), nil
	case http.StatusNotFound:
		return nil, "", fmt.Errorf("server replied with not found")
	case http.StatusUnauthorized:
		return nil, "", fmt.Errorf("server replied with not authorized")
	default:
		return nil, "", fmt.Errorf("server replied with HTTP %v", resp.StatusCode)
	}
}

func (d *Downloader) logError(fmt string, a ...interface{}) {
	logrus.WithFields(d.logrusFields()).Errorf(fmt, a...)
}

func (d *Downloader) logInfo(fmt string, a ...interface{}) {
	logrus.WithFields(d.logrusFields()).Infof(fmt, a...)
}

func (d *Downloader) logDebug(fmt string, a ...interface{}) {
	logrus.WithFields(d.logrusFields()).Debugf(fmt, a...)
}

func (d *Downloader) logrusFields() logrus.Fields {
	flds := logrus.Fields{}
	for i := range d.logAttrs {
		flds[d.logAttrs[i][0]] = flds[d.logAttrs[i][1]]
	}
	return flds
}
