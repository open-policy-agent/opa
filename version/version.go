// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

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
