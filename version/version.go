// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
//
// Package version contains version information that is set at build time.
package version

import (
	v1 "github.com/open-policy-agent/opa/v1/version"
)

// Version is the canonical version of OPA.
var Version = v1.Version

// GoVersion is the version of Go this was built with
var GoVersion = v1.GoVersion

// Platform is the runtime OS and architecture of this OPA binary
var Platform = v1.Platform

// Additional version information that is displayed by the "version" command and used to
// identify the version of running instances of OPA.
var (
	Vcs       = v1.Vcs
	Timestamp = v1.Timestamp
	Hostname  = v1.Hostname
)

// WasmRuntimeAvailable indicates if a wasm runtime is available in this OPA.
func WasmRuntimeAvailable() bool {
	return v1.WasmRuntimeAvailable()
}
