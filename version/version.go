// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package version contains version information that is set at build time.
package version

import (
	"os"
	"runtime"
)

// Version is the canonical version of OPA.
var Version = "0.49.0-dev"

// GoVersion is the version of Go this was built with
var GoVersion = runtime.Version()

// Platform is the runtime OS and architecture of this OPA binary
var Platform = runtime.GOOS + "/" + runtime.GOARCH

// Image is set to 'official' in released OPA images, otherwise it is empty
var Image = os.Getenv("OPA_DOCKER_IMAGE")

// ImageFlavor is set released images and it used in initialization
// policies to determine start up messages
var ImageFlavor = os.Getenv("OPA_DOCKER_IMAGE_FLAVOR")

// Additional version information that is displayed by the "version" command and used to
// identify the version of running instances of OPA.
var (
	Vcs       = ""
	Timestamp = ""
	Hostname  = ""
)
