// +build !linux,!windows

// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"os/user"

	"github.com/open-policy-agent/opa/logging"
)

// checkUserPrivileges could not be running in Docker, so we only warn
// if run as uid/gid 0.
func checkUserPrivileges(logger logging.Logger) {
	usr, err := user.Current()
	if err != nil {
		logger.Debug("Failed to determine uid/gid of process owner")
	} else if usr.Uid == "0" || usr.Gid == "0" {
		logger.Warn("OPA running with uid or gid 0. Running OPA with root privileges is not recommended.")
	}
}
