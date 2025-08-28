// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package version contains version information that is set at build time.
package version

import (
	"runtime"
	"runtime/debug"
)

var Version = "1.8.0"

// GoVersion is the version of Go this was built with
var GoVersion = runtime.Version()

// Platform is the runtime OS and architecture of this OPA binary
var Platform = runtime.GOOS + "/" + runtime.GOARCH

// Additional version information that is displayed by the "version" command and used to
// identify the version of running instances of OPA.
var (
	Vcs       = ""
	Timestamp = ""
	Hostname  = ""
)

func init() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	var dirty bool
	var binTimestamp, binVcs string

	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.time":
			binTimestamp = s.Value
		case "vcs.revision":
			binVcs = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}

	if Timestamp == "" {
		Timestamp = binTimestamp
	}

	if Vcs == "" {
		Vcs = binVcs
		if dirty {
			Vcs += "-dirty"
		}
	}
}
