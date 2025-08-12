// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package download implements low-level OPA bundle downloading.
package download

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"github.com/open-policy-agent/opa/v1/util"
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
	Raw     io.Reader
	Size    int
}

// Downloader implements low-level OPA bundle downloading. Downloader can be
// started and stopped. After starting, the downloader will request bundle
// updates from the remote HTTP endpoint that the client is configured to
// connect to.
type Downloader struct {
	config             Config                        // downloader configuration for tuning polling and other downloader behaviour
	client             rest.Client                   // HTTP client to use for bundle downloading
	path               string                        // path to use in bundle download request
	trigger            chan chan struct{}            // channel to signal downloads when manual triggering is enabled
	stop               chan chan struct{}            // used to signal plugin to stop running
	f                  func(context.Context, Update) // callback function invoked when download updates occur
	etag               string                        // HTTP Etag for caching purposes
	sizeLimitBytes     *int64                        // max bundle file size in bytes (passed to reader)
	bvc                *bundle.VerificationConfig
	respHdrTimeoutSec  int64
	wg                 sync.WaitGroup
	logger             logging.Logger
	stopped            bool
	stopOnce           sync.Once
	persist            bool
	longPollingEnabled bool
	lazyLoadingMode    bool
	bundleName         string
	bundleParserOpts   ast.ParserOptions
}

type downloaderResponse struct {
	b        *bundle.Bundle
	raw      io.Reader
	etag     string
	longPoll bool
	size     int
}

// New returns a new Downloader that can be started.
func New(config Config, client rest.Client, path string) *Downloader {
	return &Downloader{
		config:             config,
		client:             client,
		path:               path,
		trigger:            make(chan chan struct{}),
		stop:               make(chan chan struct{}),
		logger:             client.Logger(),
		longPollingEnabled: config.Polling.LongPollingTimeoutSeconds != nil,
	}
}

// WithCallback registers a function f to be called when download updates occur.
func (d *Downloader) WithCallback(f func(context.Context, Update)) *Downloader {
	d.f = f
	return d
}

// WithLogAttrs sets an optional set of key/value pair attributes to include in
// log messages emitted by the downloader.
func (d *Downloader) WithLogAttrs(attrs map[string]any) *Downloader {
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

// WithBundlePersistence specifies if the downloaded bundle will eventually be persisted to disk.
func (d *Downloader) WithBundlePersistence(persist bool) *Downloader {
	d.persist = persist
	return d
}

// WithLazyLoadingMode specifies how the downloaded bundle should be read.
// If true, data files in the bundle will not be deserialized
// and the check to validate that the bundle data does not contain paths
// outside the bundle's roots will not be performed while reading the bundle.
func (d *Downloader) WithLazyLoadingMode(yes bool) *Downloader {
	d.lazyLoadingMode = yes
	return d
}

// WithBundleName specifies the name of the downloaded bundle.
func (d *Downloader) WithBundleName(bundleName string) *Downloader {
	d.bundleName = bundleName
	return d
}

// WithBundleParserOpts specifies the parser options to use when parsing downloaded bundles.
func (d *Downloader) WithBundleParserOpts(opts ast.ParserOptions) *Downloader {
	d.bundleParserOpts = opts
	return d
}

// ClearCache is deprecated. Use SetCache instead.
func (d *Downloader) ClearCache() {
	d.etag = ""
}

// SetCache sets the given etag value on the downloader.
func (d *Downloader) SetCache(etag string) {
	d.etag = etag
}

// Trigger can be used to control when the downloader attempts to download
// a new bundle in manual triggering mode.
func (d *Downloader) Trigger(ctx context.Context) error {
	done := make(chan error)

	go func() {
		err := d.oneShot(ctx)
		if err != nil {
			d.logger.Error("Bundle download failed: %v.", err)
			if ctx.Err() == nil {
				done <- err
			}
		}
		close(done)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start tells the Downloader to begin downloading bundles.
func (d *Downloader) Start(ctx context.Context) {
	if *d.config.Trigger == plugins.TriggerPeriodic {
		go d.doStart(ctx)
	}
}

func (d *Downloader) doStart(context.Context) {
	// We'll revisit context passing/usage later.
	ctx, cancel := context.WithCancel(context.Background())

	d.wg.Add(1)
	go d.loop(ctx)

	done := <-d.stop // blocks until there's something to read
	cancel()
	d.wg.Wait()
	d.stopped = true
	close(done)
}

// Stop tells the Downloader to stop downloading bundles.
func (d *Downloader) Stop(context.Context) {
	if *d.config.Trigger == plugins.TriggerManual {
		return
	}

	d.stopOnce.Do(func() {
		done := make(chan struct{})
		d.stop <- done
		<-done
	})
}

func (d *Downloader) loop(ctx context.Context) {
	defer d.wg.Done()

	var retry int

	for {

		var delay time.Duration

		err := d.oneShot(ctx)

		if ctx.Err() != nil {
			return
		}

		// Note: MaxDelaySeconds, MinDelaySeconds and LongPollingTimeoutSeconds
		// are scaled from int seconds to ns in ValidateAndInjectDefaults.
		if err != nil {
			// when there was an error, use a delay that's based on the retry count
			delay = util.DefaultBackoff(float64(minRetryDelay), float64(*d.config.Polling.MaxDelaySeconds), retry)
		} else if !d.longPollingEnabled || d.config.Polling.LongPollingTimeoutSeconds == nil {
			// revert the response header timeout value on the http client's transport
			if *d.client.Config().ResponseHeaderTimeoutSeconds == 0 {
				d.client = d.client.SetResponseHeaderTimeout(&d.respHdrTimeoutSec)
			}

			// when polling, use a jittered delay based on min and max delay config
			min := float64(*d.config.Polling.MinDelaySeconds)
			max := float64(*d.config.Polling.MaxDelaySeconds)
			delay = time.Duration(((max - min) * rand.Float64()) + min)
		}

		d.logger.Debug("Waiting %v before next download/retry.", delay)

		timer, timerCancel := util.TimerWithCancel(delay)
		select {
		case <-timer.C:
			if err != nil {
				retry++
			} else {
				retry = 0
			}
		case <-ctx.Done():
			timerCancel() // explicitly cancel the timer.
			return
		}
	}
}

func (d *Downloader) oneShot(ctx context.Context) error {
	m := metrics.New()
	resp, err := d.download(ctx, m)
	if err != nil {
		d.etag = ""

		if d.f != nil {
			d.f(ctx, Update{ETag: "", Bundle: nil, Error: err, Metrics: m, Raw: nil})
		}
		return err
	}

	d.etag = resp.etag
	d.longPollingEnabled = resp.longPoll

	if d.f != nil {
		d.f(ctx, Update{ETag: resp.etag, Bundle: resp.b, Error: nil, Metrics: m, Raw: resp.raw, Size: resp.size})
	}
	return nil
}

func (d *Downloader) download(ctx context.Context, m metrics.Metrics) (*downloaderResponse, error) {
	d.logger.Debug("Download starting.")

	d.client = d.client.WithHeader("If-None-Match", d.etag)

	preferences := []string{fmt.Sprintf("modes=%v,%v", defaultBundleMode, deltaBundleMode)}

	if d.longPollingEnabled && d.config.Polling.LongPollingTimeoutSeconds != nil {
		wait := "wait=" + strconv.FormatInt(*d.config.Polling.LongPollingTimeoutSeconds, 10)
		preferences = append(preferences, wait)

		// fetch existing response header timeout value on the http client's transport and
		// clear it for the long poll request
		current := d.client.Config().ResponseHeaderTimeoutSeconds
		if *current != 0 {
			d.respHdrTimeoutSec = *current
			t := int64(0)
			d.client = d.client.SetResponseHeaderTimeout(&t)
		}
	}

	preferValue := strings.Join(preferences, ";")
	d.client = d.client.WithHeader("Prefer", preferValue)

	m.Timer(metrics.BundleRequest).Start()
	resp, err := d.client.Do(ctx, "GET", d.path)
	m.Timer(metrics.BundleRequest).Stop()
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	defer util.Close(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		var buf bytes.Buffer
		if resp.Body != nil {
			d.logger.Debug("Download in progress.")
			m.Timer(metrics.RegoLoadBundles).Start()
			defer m.Timer(metrics.RegoLoadBundles).Stop()
			baseURL := path.Join(d.client.Config().URL, d.path)

			cnt := &count{}
			r := io.TeeReader(resp.Body, cnt)

			var loader bundle.DirectoryLoader
			if d.persist {
				tee := io.TeeReader(r, &buf)
				loader = bundle.NewTarballLoaderWithBaseURL(tee, baseURL)
			} else {
				loader = bundle.NewTarballLoaderWithBaseURL(r, baseURL)
			}

			// Setting the size limit on the loader allows early exit in the case
			// of any file exceeding the limit, without the file getting loaded
			if d.sizeLimitBytes != nil {
				loader = loader.WithSizeLimitBytes(*d.sizeLimitBytes)
			}

			etag := resp.Header.Get("ETag")

			reader := bundle.NewCustomReader(loader).
				WithRegoVersion(d.bundleParserOpts.RegoVersion).
				WithMetrics(m).
				WithBundleVerificationConfig(d.bvc).
				WithBundleEtag(etag).
				WithLazyLoadingMode(d.lazyLoadingMode).
				WithBundleName(d.bundleName).
				WithBundlePersistence(d.persist)

			if d.sizeLimitBytes != nil {
				reader = reader.WithSizeLimitBytes(*d.sizeLimitBytes)
			}

			if d.logger.GetLevel() >= logging.Debug {
				expectedBundleContentType := []string{
					"application/gzip",
					"application/octet-stream",
					"application/vnd.openpolicyagent.bundles",
				}
				contentType := resp.Header.Get("content-type")

				if !slices.Contains(expectedBundleContentType, contentType) {
					d.logger.Debug("Content-Type response header set to %v. Expected one of %v. "+
						"Possibly not a bundle being downloaded.",
						contentType,
						expectedBundleContentType,
					)
				}
			}

			b, err := reader.Read()
			if err != nil {
				return nil, err
			}

			return &downloaderResponse{
				b:        &b,
				raw:      &buf,
				etag:     etag,
				longPoll: isLongPollSupported(resp.Header),
				size:     cnt.Bytes(),
			}, nil
		}

		d.logger.Debug("Server replied with empty body.")
		return &downloaderResponse{
			b:        nil,
			raw:      nil,
			etag:     "",
			longPoll: isLongPollSupported(resp.Header),
		}, nil
	case http.StatusNotModified:
		etag := resp.Header.Get("ETag")
		if etag == "" {
			etag = d.etag
		}
		return &downloaderResponse{
			b:        nil,
			raw:      nil,
			etag:     etag,
			longPoll: d.longPollingEnabled,
		}, nil
	default:
		if d.logger.GetLevel() == logging.Debug && resp.Body != nil {
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				d.logger.Debug("bundle download error response with response body: %s", body)
			}
		}

		return nil, HTTPError{StatusCode: resp.StatusCode}
	}
}

type count struct {
	total int
}

func (c *count) Write(p []byte) (n int, err error) {
	n = len(p)
	c.total += n
	return
}

func (c *count) Bytes() int {
	return c.total
}

func isLongPollSupported(header http.Header) bool {
	return header.Get("Content-Type") == "application/vnd.openpolicyagent.bundles"
}

type HTTPError struct {
	StatusCode int
}

func (e HTTPError) Error() string {
	return "server replied with " + http.StatusText(e.StatusCode)
}
