// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package http

import (
	"net/http"
	"time"

	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa/errors"
)

// WithURL configures the URL to download the bundle from.
func (l *Loader) WithURL(url string) *Loader {
	l.url = url
	return l
}

// WithClient configures the HTTP client to use. If not configured,
// http.DefaultClient is used.
func (l *Loader) WithClient(client *http.Client) *Loader {
	if client == nil {
		l.configErr = errors.New(errors.InvalidConfigErr, "client")
		return l
	}

	l.client = client
	return l
}

// WithInterval configures the minimum and maximum delay between bundle downloads.
func (l *Loader) WithInterval(min, max time.Duration) *Loader {
	if min > max {
		l.configErr = errors.New(errors.InvalidConfigErr, "interval min > max")
		return l
	}

	l.minDelay = min
	l.maxDelay = max
	return l
}

// WithPrepareRequest configures a handler to customize the HTTP requests before their sending. The
// HTTP request is not modified after the handle invocation.
func (l *Loader) WithPrepareRequest(prepare func(*http.Request) error) *Loader {
	if prepare == nil {
		l.configErr = errors.New(errors.InvalidConfigErr, "missing prepare")
		return l
	}

	l.prepareRequest = prepare
	return l
}

// WithErrorLogger configures an error logger invoked with all the errors.
func (l *Loader) WithErrorLogger(logger func(error)) *Loader {
	if logger == nil {
		l.configErr = errors.New(errors.InvalidConfigErr, "missing logger")
		return l
	}

	l.logError = logger
	return l
}
