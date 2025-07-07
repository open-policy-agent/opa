// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package version contains version information that is set at build time.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

var Version = "1.7.0-dev"

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

type SemVer struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
}

func ParseSemVer(s string) (SemVer, error) {
	var v SemVer
	_, err := fmt.Sscanf(s, "%d.%d.%d%s", &v.Major, &v.Minor, &v.Patch, &v.PreRelease)
	if err != nil {
		return v, fmt.Errorf("invalid semantic version: %w", err)
	}

	if v.Major < 0 || v.Minor < 0 || v.Patch < 0 {
		return v, fmt.Errorf("semantic version components must be non-negative")
	}

	return v, nil
}

func (v SemVer) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch) + v.PreRelease
}

func (v SemVer) Compare(other SemVer) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	if v.PreRelease != other.PreRelease {
		if v.PreRelease == "" {
			return 1 // v is a stable release, other is pre-release
		}
		if other.PreRelease == "" {
			return -1 // other is stable release, v is pre-release
		}
		if v.PreRelease < other.PreRelease {
			return -1
		}
		return 1
	}
	return 0
}
