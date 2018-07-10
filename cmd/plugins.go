// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Specifies additional cmd commands that available to systems that can load plugins
// +build linux,cgo darwin,cgo

package cmd

import (
	"github.com/open-policy-agent/opa/runtime"
	"github.com/spf13/cobra"
)

func init() {
	var pluginDir string

	// flag is persistent (can be loaded on all children commands)
	RootCommand.PersistentFlags().StringVar(&pluginDir, "plugin-dir", "", `set path to directory containing
shared object files to dynamically load plugins and builtins`)

	// Runs before *all* children commands
	RootCommand.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// only register custom plugins if directory specified
		if pluginDir != "" {
			err := runtime.RegisterBuiltinsFromDir(pluginDir)
			if err != nil {
				return err
			}
		}
		return nil
	}
}
