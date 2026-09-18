// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
//
// Package download implements low-level OPA bundle downloading.
package download

import (
	"github.com/open-policy-agent/opa/plugins/rest"
	v1 "github.com/open-policy-agent/opa/v1/download"
)

// PollingConfig represents polling configuration for the downloader.
type PollingConfig = v1.PollingConfig

// Config represents the configuration for the downloader.
type Config = v1.Config

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

type OCIDownloader = v1.OCIDownloader
