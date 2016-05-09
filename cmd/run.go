// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"os"
	"path"

	"github.com/open-policy-agent/opa/runtime"
	"github.com/spf13/cobra"
)

// default filename for the interactive shell's history
var defaultHistoryFile = ".opa_history"

func init() {

	params := &runtime.Params{}

	runCommand := &cobra.Command{
		Use:   "run",
		Short: "Start OPA in interative or server mode",
		Long: `Start an instance of the Open Policy Agent (OPA).

The 'run' command starts an instance of the OPA runtime. The OPA
runtime can be started as an interactive shell or a server.

When the runtime is started as a shell, users can define rules and
evaluate expressions interactively. When the runtime is started as
a server, users can access OPA's APIs via HTTP.

The runtime can be initialized with one or more files that represent
base documents (e.g., example.json) or policies (e.g., example.rego).
`,
		Run: func(cmd *cobra.Command, args []string) {
			params.Paths = args
			rt := &runtime.Runtime{}
			rt.Start(params)
		},
	}

	runCommand.Flags().BoolVarP(&params.Server, "server", "s", false, "start the runtime in server mode")
	runCommand.Flags().StringVarP(&params.HistoryPath, "history", "H", historyPath(), "set path of history file")

	usageTemplate := `Usage:
  {{.UseLine}} [flags] [files]
  
Flags:
{{.LocalFlags.FlagUsages | trimRightSpace}}
`

	runCommand.SetUsageTemplate(usageTemplate)

	RootCommand.AddCommand(runCommand)
}

func historyPath() string {
	home := os.Getenv("HOME")
	if len(home) == 0 {
		return defaultHistoryFile
	}
	return path.Join(home, defaultHistoryFile)
}
