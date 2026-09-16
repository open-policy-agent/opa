// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package server contains the policy engine's server handlers.
//
// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package server

import (
	v1 "github.com/open-policy-agent/opa/v1/server"
)

// Info contains information describing a policy decision.
type Info = v1.Info

// BundleInfo contains information describing a bundle.
type BundleInfo = v1.BundleInfo

// AuthenticationScheme enumerates the supported authentication schemes. The
// authentication scheme determines how client identities are established.
type AuthenticationScheme = v1.AuthenticationScheme

// Set of supported authentication schemes.
const (
	AuthenticationOff   = v1.AuthenticationOff
	AuthenticationToken = v1.AuthenticationToken
	AuthenticationTLS   = v1.AuthenticationTLS
)

// AuthorizationScheme enumerates the supported authorization schemes. The authorization
// scheme determines how access to OPA is controlled.
type AuthorizationScheme = v1.AuthorizationScheme

// Set of supported authorization schemes.
const (
	AuthorizationOff   = v1.AuthorizationOff
	AuthorizationBasic = v1.AuthorizationBasic
)

// Set of handlers for use in the "handler" dimension of the duration metric.
const (
	PromHandlerV0Data     = v1.PromHandlerV0Data
	PromHandlerV1Data     = v1.PromHandlerV1Data
	PromHandlerV1Query    = v1.PromHandlerV1Query
	PromHandlerV1Policies = v1.PromHandlerV1Policies
	PromHandlerV1Compile  = v1.PromHandlerV1Compile
	PromHandlerV1Config   = v1.PromHandlerV1Config
	PromHandlerV1Status   = v1.PromHandlerV1Status
	PromHandlerIndex      = v1.PromHandlerIndex
	PromHandlerCatch      = v1.PromHandlerCatch
	PromHandlerHealth     = v1.PromHandlerHealth
	PromHandlerAPIAuthz   = v1.PromHandlerAPIAuthz
)

// Server represents an instance of OPA running in server mode.
type Server = v1.Server

// Metrics defines the interface that the server requires for recording HTTP
// handler metrics.
type Metrics = v1.Metrics

// TLSConfig represents the TLS configuration for the server.
// This configuration is used to configure file watchers to reload each file as it
// changes on disk.
type TLSConfig = v1.TLSConfig

// Loop will contain all the calls from the server that we'll be listening on.
type Loop = v1.Loop

// New returns a new Server.
func New() *Server {
	return v1.New()
}
