// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"flag"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/open-policy-agent/opa/runtime"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// default filename for the interactive shell's history
var defaultHistoryFile = ".opa_history"

// default policy definition storage directory
var defaultPolicyDir = "policies"

// default listening address for the server
var defaultAddr = ":8181"

func init() {

	params := &runtime.Params{}

	runCommand := &cobra.Command{
		Use:   "run",
		Short: "Start OPA in interative or server mode",
		Long: `Start an instance of the Open Policy Agent (OPA).

To run the interactive shell:

	$ opa run

To run the server without saving policies:

	$ opa run -s

To run the server and persist policies to a local directory:

	$ opa run -s -p ./policies/

The 'run' command starts an instance of the OPA runtime. The OPA
runtime can be started as an interactive shell or a server.

When the runtime is started as a shell, users can define rules and
evaluate expressions interactively. When the runtime is started as
a server, users can access OPA's APIs via HTTP.

The runtime can be initialized with one or more files that represent
base documents (e.g., example.json) or policies (e.g., example.rego).

If the --policy-dir option is specified any files inside the directory
will be considered policy definitions and will be loaded on startup. API
calls to create new policies save the definition file to this direcory.
In addition, API calls to delete policies will remove the definition file.
`,
		Run: func(cmd *cobra.Command, args []string) {
			params.Paths = args
			rt := &runtime.Runtime{}
			rt.Start(params)
		},
	}

	runCommand.Flags().BoolVarP(&params.Server, "server", "s", false, "start the runtime in server mode")
	runCommand.Flags().StringVarP(&params.HistoryPath, "history", "H", historyPath(), "set path of history file")
	runCommand.Flags().StringVarP(&params.PolicyDir, "policy-dir", "p", "", "set directory to store policy definitions")
	runCommand.Flags().StringVarP(&params.Addr, "addr", "a", defaultAddr, "set listening address of the server")
	runCommand.Flags().StringVarP(&params.OutputFormat, "format", "f", "pretty", "set shell output format, i.e, pretty, json")
	runCommand.Flags().BoolVarP(&params.Watch, "watch", "w", false, "watch command line files for changes")

	wrapFlags(runCommand.Flags())
	flag.Parse()

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

func policyDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return defaultPolicyDir
	}
	return filepath.Join(cwd, defaultPolicyDir)
}

type flagWrapper struct {
	inner flag.Value
	typ   string
}

func (f *flagWrapper) String() string {
	return f.inner.String()
}

func (f *flagWrapper) Set(s string) error {
	return f.inner.Set(s)
}

func (f *flagWrapper) Type() string {
	return f.typ
}

func wrapFlag(v flag.Value) pflag.Value {
	if pf, ok := v.(pflag.Value); ok {
		return pf
	}
	return &flagWrapper{
		inner: v,
		typ:   strings.TrimSuffix(reflect.TypeOf(v).Elem().Name(), "Value"),
	}
}

func wrapFlags(flags *pflag.FlagSet) {
	flag.CommandLine.VisitAll(func(flg *flag.Flag) {
		flags.Var(wrapFlag(flg.Value), flg.Name, flg.Usage)
	})
}
