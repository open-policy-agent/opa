// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"crypto/tls"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/open-policy-agent/opa/runtime"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/util"
	"github.com/spf13/cobra"
)

// default filename for the interactive shell's history
var defaultHistoryFile = ".opa_history"

// default policy definition storage directory
var defaultPolicyDir = "policies"

// default listening address for the server
var defaultAddr = ":8181"

func init() {

	var tlsCertFile string
	var tlsPrivateKeyFile string

	authentication := util.NewEnumFlag("off", []string{"token", "off"})

	authenticationSchemes := map[string]server.AuthenticationScheme{
		"token": server.AuthenticationToken,
		"off":   server.AuthenticationOff,
	}

	authorization := util.NewEnumFlag("off", []string{"basic", "off"})

	authorizationScheme := map[string]server.AuthorizationScheme{
		"basic": server.AuthorizationBasic,
		"off":   server.AuthorizationOff,
	}

	logLevel := util.NewEnumFlag("info", []string{"debug", "info", "error"})
	logFormat := util.NewEnumFlag("text", []string{"text", "json"})

	params := runtime.NewParams()

	runCommand := &cobra.Command{
		Use:   "run",
		Short: "Start OPA in interative or server mode",
		Long: `Start an instance of the Open Policy Agent (OPA).

To run the interactive shell:

	$ opa run

To run the server:

	$ opa run -s

To evaluate a query from the command line:

	$ opa run -e 'data.repl.version[key] = value'

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

For example:

	$ echo "[1,2,3]" > example.json
	$ opa run -e 'data.foo' foo.bar:./example.json
	{
	  "bar": [
	    1,
	    2,
	    3
	  ]
	}
`,
		Run: func(cmd *cobra.Command, args []string) {

			cert, err := loadCertificate(tlsCertFile, tlsPrivateKeyFile)
			if err != nil {
				fmt.Println("error:", err)
				os.Exit(1)
			}

			params.Authentication = authenticationSchemes[authentication.String()]
			params.Authorization = authorizationScheme[authorization.String()]
			params.Certificate = cert
			params.Logging = runtime.LoggingConfig{
				Level:  logLevel.String(),
				Format: logFormat.String(),
			}
			params.Paths = args

			rt := &runtime.Runtime{}
			rt.Start(params)
		},
	}

	runCommand.Flags().BoolVarP(&params.Server, "server", "s", false, "start the runtime in server mode")
	runCommand.Flags().StringVarP(&params.Eval, "eval", "e", "", "evaluate, print, exit")
	runCommand.Flags().StringVarP(&params.HistoryPath, "history", "H", historyPath(), "set path of history file")
	runCommand.Flags().StringVarP(&params.Addr, "addr", "a", defaultAddr, "set listening address of the server")
	runCommand.Flags().StringVarP(&params.InsecureAddr, "insecure-addr", "", "", "set insecure listening address of the server")
	runCommand.Flags().StringVarP(&params.OutputFormat, "format", "f", "pretty", "set shell output format, i.e, pretty, json")
	runCommand.Flags().BoolVarP(&params.Watch, "watch", "w", false, "watch command line files for changes")
	runCommand.Flags().StringVarP(&tlsCertFile, "tls-cert-file", "", "", "set path of TLS certificate file")
	runCommand.Flags().StringVarP(&tlsPrivateKeyFile, "tls-private-key-file", "", "", "set path of TLS private key file")
	runCommand.Flags().VarP(authentication, "authentication", "", "set authentication scheme")
	runCommand.Flags().VarP(authorization, "authorization", "", "set authorization scheme")
	runCommand.Flags().VarP(logLevel, "log-level", "l", "set log level")
	runCommand.Flags().VarP(logFormat, "log-format", "", "set log format")

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

func loadCertificate(tlsCertFile, tlsPrivateKeyFile string) (*tls.Certificate, error) {

	if tlsCertFile != "" && tlsPrivateKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsCertFile, tlsPrivateKeyFile)
		if err != nil {
			return nil, err
		}
		return &cert, nil
	} else if tlsCertFile != "" || tlsPrivateKeyFile != "" {
		return nil, fmt.Errorf("--tls-cert-file and --tls-private-key-file must be specified together")
	}

	return nil, nil
}
