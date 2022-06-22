// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"os"
	"os/user"

	"github.com/open-policy-agent/opa/logging"
)

// checkUserPrivileges on Linux could be running in Docker, so we check if
// we're running in the official container image.
func checkUserPrivileges(logger logging.Logger) {
	usr, err := user.Current()
	if err != nil {
		logger.Debug("Failed to determine uid/gid of process owner")
	} else if usr.Uid == "0" || usr.Gid == "0" {
		message := "OPA running with uid or gid 0. Running OPA with root privileges is not recommended."
		if os.Getenv("OPA_DOCKER_IMAGE") == "official" {
			message += " Use the -rootless image to avoid running with root privileges. This will be made the default in later OPA releases."
		}
		logger.Warn(message)
	}
}
