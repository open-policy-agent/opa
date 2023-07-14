// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/cmd/internal/deprecation"
)

// RootCommand is the base CLI command that all subcommands are added to.
var RootCommand = &cobra.Command{
	Use:   path.Base(os.Args[0]),
	Short: "Open Policy Agent (OPA)",
	Long:  "An open source project to policy-enable your service.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {

		message, fatal := deprecation.CheckWarnings(os.Environ(), cmd.Use)
		if message != "" {
			cmd.PrintErr(message)
			if fatal {
				os.Exit(1)
			}
		}

	},
}
