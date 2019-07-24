// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/version"
)

func init() {

	var versionCommand = &cobra.Command{
		Use:   "version",
		Short: "Print the version of OPA",
		Long:  "Show version and build information for OPA.",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Version: " + version.Version)
			fmt.Println("Build Commit: " + version.Vcs)
			fmt.Println("Build Timestamp: " + version.Timestamp)
			fmt.Println("Build Hostname: " + version.Hostname)
		},
	}

	RootCommand.AddCommand(versionCommand)
}
