// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/spf13/cobra"
)

func init() {

	// initial setup
	SetupRuntimeDefaults()

	// parse OPA's command line options
	ParseOpaCliOptions(runCommand)

	usageTemplate := `Usage:
  {{.UseLine}} [flags] [files]

Flags:
{{.LocalFlags.FlagUsages | trimRightSpace}}
`
	runCommand.SetUsageTemplate(usageTemplate)

	RootCommand.AddCommand(runCommand)
}

var runCommand = &cobra.Command{
	Use:   "run",
	Short: "Start OPA in interative or server mode",
	Long: `Start an instance of the Open Policy Agent (OPA).

To run the interactive shell:

	$ opa run

To run the server:

	$ opa run -s

The 'run' command starts an instance of the OPA runtime. The OPA runtime can be
started as an interactive shell or a server.

When the runtime is started as a shell, users can define rules and evaluate
expressions interactively. When the runtime is started as a server, OPA exposes
an HTTP API for managing policies, reading and writing data, and executing
queries.

The runtime can be initialized with one or more files that contain policies or
data. OPA supports both JSON and YAML data. If a directory is given, OPA will
recursively load the files contained in the directory. When loading from
directories, only files with known extensions are considered. The current set of
file extensions that OPA will consider are:

	.json          # JSON data
	.yaml or .yml  # YAML data
	.rego          # Rego file

Data file and directory paths can be prefixed with the desired destination in
the data document with the following syntax:

	<dotted-path>:<file-path>
`,
	Run: run,
}
