// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/internal/report"
	"github.com/open-policy-agent/opa/internal/uuid"
	"github.com/open-policy-agent/opa/version"
)

func init() {

	var check bool
	var versionCommand = &cobra.Command{
		Use:   "version",
		Short: "Print the version of OPA",
		Long:  "Show version and build information for OPA.",
		Run: func(cmd *cobra.Command, args []string) {
			generateCmdOutput(os.Stdout, check)
		},
	}

	// The version command can also be used to check for the latest released OPA version.
	// Some tools could use this for feature flagging purposes and hence this option is OFF by-default.
	versionCommand.Flags().BoolVarP(&check, "check", "c", false, "check for latest OPA release")
	RootCommand.AddCommand(versionCommand)
}

func generateCmdOutput(out io.Writer, check bool) {
	fmt.Fprintln(out, "Version: "+version.Version)
	fmt.Fprintln(out, "Build Commit: "+version.Vcs)
	fmt.Fprintln(out, "Build Timestamp: "+version.Timestamp)
	fmt.Fprintln(out, "Build Hostname: "+version.Hostname)
	fmt.Fprintln(out, "Go Version: "+version.GoVersion)

	var wasmAvailable string

	if version.WasmRuntimeAvailable {
		wasmAvailable = "available"
	} else {
		wasmAvailable = "unavailable"
	}

	fmt.Fprintln(out, "WebAssembly: "+wasmAvailable)

	if check {
		err := checkOPAUpdate(out)
		if err != nil {
			fmt.Fprintf(out, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

func checkOPAUpdate(out io.Writer) error {
	id, err := uuid.New(rand.Reader)
	if err != nil {
		return err
	}

	reporter, err := report.New(id)
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
