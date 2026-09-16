// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
//
// Package rest implements a REST client for communicating with remote services.
package rest

import (
	"crypto/tls"
	"net/http"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/v1/keys"
	v1 "github.com/open-policy-agent/opa/v1/plugins/rest"
	"github.com/open-policy-agent/opa/v1/tracing"
)

// DefaultTLSConfig defines standard TLS configurations based on the Config
func DefaultTLSConfig(c Config) (*tls.Config, error) {
	return v1.DefaultTLSConfig(c)
}

// DefaultRoundTripperClient is a reasonable set of defaults for HTTP auth plugins
func DefaultRoundTripperClient(t *tls.Config, timeout int64) *http.Client {
	return v1.DefaultRoundTripperClient(t, timeout)
}

// AccessToken holds a GCP access token.
type AccessToken = v1.AccessToken

// An HTTPAuthPlugin represents a mechanism to construct and configure HTTP authentication for a REST service
type HTTPAuthPlugin = v1.HTTPAuthPlugin

// Config represents configuration for a REST client.
type Config = v1.Config

// An AuthPluginLookupFunc can lookup auth plugins by their name.
type AuthPluginLookupFunc = v1.AuthPluginLookupFunc

// Client implements an HTTP/REST client for communicating with remote
// services.
type Client = v1.Client

// Name returns an option that overrides the service name on the client.
func Name(s string) func(*Client) {
	return v1.Name(s)
}

// AuthPluginLookup assigns a function to lookup an HTTPAuthPlugin to a new Client.
// It's intended to be used when creating a Client using New(). Usually this is passed
// the plugins.AuthPlugin func, which retrieves a registered HTTPAuthPlugin from the
// plugin manager.
func AuthPluginLookup(l AuthPluginLookupFunc) func(*Client) {
	return v1.AuthPluginLookup(l)
}

// Logger assigns a logger to the client
func Logger(l logging.Logger) func(*Client) {
	return v1.Logger(l)
}

// DistributedTracingOpts sets the options to be used by distributed tracing.
func DistributedTracingOpts(tr tracing.Options) func(*Client) {
	return v1.DistributedTracingOpts(tr)
}

// New returns a new Client for config.
func New(config []byte, keys map[string]*keys.Config, opts ...func(*Client)) (Client, error) {
	return v1.New(config, keys, opts...)
}
