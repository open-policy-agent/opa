// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"path"

	"github.com/spf13/cobra"
)

// RootCommand is the base CLI command that all subcommands are added to.
var RootCommand = &cobra.Command{
	Use:   path.Base(os.Args[0]),
	Short: "Open Policy Agent (OPA)",
	Long:  "An open source project to policy-enable your service.",
}
