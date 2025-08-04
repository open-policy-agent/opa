// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/open-policy-agent/opa/v1/ast"
	version2 "github.com/open-policy-agent/opa/v1/version"
	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/cmd/internal/env"
	"github.com/open-policy-agent/opa/internal/report"
)

func initVersion(root *cobra.Command, brand string) {
	var check bool
	var versionCommand = &cobra.Command{
		Use:   "version",
		Short: `Print the version of ` + brand,
		Long:  `Show version and build information for ` + brand + `.`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return env.CmdFlags.CheckEnvironmentVariables(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			return generateCmdOutput(os.Stdout, check)
		},
	}

	// The version command can also be used to check for the latest released OPA version.
	// Some tools could use this for feature flagging purposes and hence this option is OFF by-default.
	versionCommand.Flags().BoolVarP(&check, "check", "c", false, "check for latest "+brand+" release")
	root.AddCommand(versionCommand)
}

func generateCmdOutput(out io.Writer, check bool) error {
	fmt.Fprintln(out, "Version: "+version2.Version)
	fmt.Fprintln(out, "Build Commit: "+version2.Vcs)
	fmt.Fprintln(out, "Build Timestamp: "+version2.Timestamp)
	fmt.Fprintln(out, "Build Hostname: "+version2.Hostname)
	fmt.Fprintln(out, "Go Version: "+version2.GoVersion)
	fmt.Fprintln(out, "Platform: "+version2.Platform)
	fmt.Fprintln(out, "Rego Version: "+ast.DefaultRegoVersion.String())

	var wasmAvailable string

	if version2.WasmRuntimeAvailable() {
		wasmAvailable = "available"
	} else {
		wasmAvailable = "unavailable"
	}

	fmt.Fprintln(out, "WebAssembly: "+wasmAvailable)

	if check {
		err := checkOPAUpdate(out)
		if err != nil {
			fmt.Fprintf(out, "Error: %v\n", err)
			return err
		}
	}
	return nil
}

func checkOPAUpdate(out io.Writer) error {
	reporter, err := report.New(report.Options{})
	if err != nil {
		return err
	}

	resp, err := reporter.SendReport(context.Background())
	if err != nil {
		return err
	}

	fmt.Fprintln(out, resp.Pretty())
	return nil
}
