// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"

	iversion "github.com/open-policy-agent/opa/internal/version"
)

// UserAgent lets you override the OPA UA sent with all the HTTP requests.
// It's another vanity thing -- if you build your own version of OPA, you
// may want to adjust this.
// NOTE(sr): Caution: Please consider this experimental, I have the hunch
// that we'll find a better way to make this adjustment in the future.
func UserAgent(agent string) {
	iversion.UserAgent = agent
}

func Command(rootCommand *cobra.Command, brand string) *cobra.Command {
	// rootCommand is the base CLI command that all subcommands are added to.
	if rootCommand == nil {
		rootCommand = &cobra.Command{
			Use:   "opa",
			Short: "Open Policy Agent (OPA)",
			Long:  "An open source project to policy-enable your service.",
		}
	}

	initBench(rootCommand, brand)
	initBuild(rootCommand, brand)
	initCapabilities(rootCommand, brand)
	initCheck(rootCommand, brand)
	initDeps(rootCommand, brand)
	initEval(rootCommand, brand)
	initExec(rootCommand, brand)
	initFmt(rootCommand, brand)
	initInspect(rootCommand, brand)
	initOracle(rootCommand, brand)
	initParse(rootCommand, brand)
	initRefactor(rootCommand, brand)
	initRun(rootCommand, brand)
	initSign(rootCommand, brand)
	initTest(rootCommand, brand)
	initVersion(rootCommand, brand)
	return rootCommand
}
