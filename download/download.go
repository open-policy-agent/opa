// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package download implements low-level OPA bundle downloading.
package download

import (
	"github.com/open-policy-agent/opa/plugins/rest"
	v1 "github.com/open-policy-agent/opa/v1/download"
)

// Update contains the result of a download. If an error occurred, the Error
// field will be non-nil. If a new bundle is available, the Bundle field will
// be non-nil.
type Update = v1.Update

// Downloader implements low-level OPA bundle downloading. Downloader can be
// started and stopped. After starting, the downloader will request bundle
// updates from the remote HTTP endpoint that the client is configured to
// connect to.
type Downloader = v1.Downloader

// New returns a new Downloader that can be started.
func New(config Config, client rest.Client, path string) *Downloader {
	return v1.New(config, client, path)
}

type HTTPError = v1.HTTPError
