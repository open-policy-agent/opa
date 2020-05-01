// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/runtime"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/util"
)

const (
	defaultAddr        = ":8181"        // default listening address for server
	defaultHistoryFile = ".opa_history" // default filename for shell history
)

type runCmdParams struct {
	rt                runtime.Params
	tlsCertFile       string
	tlsPrivateKeyFile string
	tlsCACertFile     string
	ignore            []string
	serverMode        bool
	skipVersionCheck  bool
	authentication    *util.EnumFlag
	authorization     *util.EnumFlag
	logLevel          *util.EnumFlag
	logFormat         *util.EnumFlag
}

func newRunParams() runCmdParams {
	return runCmdParams{
		rt:             runtime.NewParams(),
		authentication: util.NewEnumFlag("off", []string{"token", "tls", "off"}),
		authorization:  util.NewEnumFlag("off", []string{"basic", "off"}),
		logLevel:       util.NewEnumFlag("info", []string{"debug", "info", "error"}),
		logFormat:      util.NewEnumFlag("json", []string{"text", "json", "json-pretty"}),
	}
}

func init() {
	cmdParams := newRunParams()

	runCommand := &cobra.Command{
		Use:   "run",
		Short: "Start OPA in interactive or server mode",
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
data. If the '--bundle' option is specified the paths will be treated as policy
bundles and loaded following standard bundle conventions. The path can be a
compressed archive file or a directory which will be treated as a bundle.
Without the '--bundle' flag OPA will recursively load ALL rego, JSON, and YAML
files.

When loading from directories, only files with known extensions are considered.
The current set of file extensions that OPA will consider are:

	.json          # JSON data
	.yaml or .yml  # YAML data
	.rego          # Rego file

Non-bundle data file and directory paths can be prefixed with the desired
destination in the data document with the following syntax:

	<dotted-path>:<file-path>

To set a data file as the input document in the interactive shell use the
"repl.input" path prefix with the input file:

	repl.input:<file-path>

Example:

	opa run repl.input:input.json

Which will load the "input.json" file at path "data.repl.input".

Use the "help input" command in the interactive shell to see more options.


File paths can be specified as URLs to resolve ambiguity in paths containing colons:

	$ opa run file:///c:/path/to/data.json
`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			rt := initRuntime(ctx, cmdParams, args)
			startRuntime(ctx, rt, cmdParams.serverMode)
		},
	}

	addConfigFileFlag(runCommand.Flags(), &cmdParams.rt.ConfigFile)
	runCommand.Flags().BoolVarP(&cmdParams.serverMode, "server", "s", false, "start the runtime in server mode")
	runCommand.Flags().StringVarP(&cmdParams.rt.HistoryPath, "history", "H", historyPath(), "set path of history file")
	cmdParams.rt.Addrs = runCommand.Flags().StringSliceP("addr", "a", []string{defaultAddr}, "set listening address of the server (e.g., [ip]:<port> for TCP, unix://<path> for UNIX domain socket)")
	cmdParams.rt.DiagnosticAddrs = runCommand.Flags().StringSlice("diagnostic-addr", []string{}, "set read-only diagnostic listening address of the server for /health and /metric APIs (e.g., [ip]:<port> for TCP, unix://<path> for UNIX domain socket)")
	runCommand.Flags().StringVarP(&cmdParams.rt.InsecureAddr, "insecure-addr", "", "", "set insecure listening address of the server")
	runCommand.Flags().MarkDeprecated("insecure-addr", "use --addr instead")
	runCommand.Flags().StringVarP(&cmdParams.rt.OutputFormat, "format", "f", "pretty", "set shell output format, i.e, pretty, json")
	runCommand.Flags().BoolVarP(&cmdParams.rt.Watch, "watch", "w", false, "watch command line files for changes")
	addMaxErrorsFlag(runCommand.Flags(), &cmdParams.rt.ErrorLimit)
	runCommand.Flags().BoolVarP(&cmdParams.rt.PprofEnabled, "pprof", "", false, "enables pprof endpoints")
	runCommand.Flags().StringVarP(&cmdParams.tlsCertFile, "tls-cert-file", "", "", "set path of TLS certificate file")
	runCommand.Flags().StringVarP(&cmdParams.tlsPrivateKeyFile, "tls-private-key-file", "", "", "set path of TLS private key file")
	runCommand.Flags().StringVarP(&cmdParams.tlsCACertFile, "tls-ca-cert-file", "", "", "set path of TLS CA cert file")
	runCommand.Flags().VarP(cmdParams.authentication, "authentication", "", "set authentication scheme")
	runCommand.Flags().VarP(cmdParams.authorization, "authorization", "", "set authorization scheme")
	runCommand.Flags().VarP(cmdParams.logLevel, "log-level", "l", "set log level")
	runCommand.Flags().VarP(cmdParams.logFormat, "log-format", "", "set log format")
	runCommand.Flags().IntVar(&cmdParams.rt.GracefulShutdownPeriod, "shutdown-grace-period", 10, "set the time (in seconds) that the server will wait to gracefully shut down")
	addConfigOverrides(runCommand.Flags(), &cmdParams.rt.ConfigOverrides)
	addConfigOverrideFiles(runCommand.Flags(), &cmdParams.rt.ConfigOverrideFiles)
	addBundleModeFlag(runCommand.Flags(), &cmdParams.rt.BundleMode, false)
	runCommand.Flags().BoolVar(&cmdParams.skipVersionCheck, "skip-version-check", false, "disables anonymous version reporting (see: https://openpolicyagent.org/docs/latest/privacy)")
	addIgnoreFlag(runCommand.Flags(), &cmdParams.ignore)

	usageTemplate := `Usage:
  {{.UseLine}} [files]

Flags:
{{.LocalFlags.FlagUsages | trimRightSpace}}
`

	runCommand.SetUsageTemplate(usageTemplate)

	RootCommand.AddCommand(runCommand)
}

func initRuntime(ctx context.Context, params runCmdParams, args []string) *runtime.Runtime {
	authenticationSchemes := map[string]server.AuthenticationScheme{
		"token": server.AuthenticationToken,
		"tls":   server.AuthenticationTLS,
		"off":   server.AuthenticationOff,
	}

	authorizationScheme := map[string]server.AuthorizationScheme{
		"basic": server.AuthorizationBasic,
		"off":   server.AuthorizationOff,
	}

	cert, err := loadCertificate(params.tlsCertFile, params.tlsPrivateKeyFile)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	if params.tlsCACertFile != "" {
		pool, err := loadCertPool(params.tlsCACertFile)
		if err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
		params.rt.CertPool = pool
	}

	params.rt.Authentication = authenticationSchemes[params.authentication.String()]
	params.rt.Authorization = authorizationScheme[params.authorization.String()]
	params.rt.Certificate = cert
	params.rt.Logging = runtime.LoggingConfig{
		Level:  params.logLevel.String(),
		Format: params.logFormat.String(),
	}
	params.rt.Paths = args
	params.rt.Filter = loaderFilter{
		Ignore: params.ignore,
	}.Apply

	params.rt.EnableVersionCheck = !params.skipVersionCheck

	rt, err := runtime.NewRuntime(ctx, params.rt)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	return rt
}

func startRuntime(ctx context.Context, rt *runtime.Runtime, serverMode bool) {
	if serverMode {
		rt.StartServer(ctx)
	} else {
		rt.StartREPL(ctx)
	}
}

func historyPath() string {
	home := os.Getenv("HOME")
	if len(home) == 0 {
		return defaultHistoryFile
	}
	return path.Join(home, defaultHistoryFile)
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

func loadCertPool(tlsCACertFile string) (*x509.CertPool, error) {
	caCertPEM, err := ioutil.ReadFile(tlsCACertFile)
	if err != nil {
		return nil, fmt.Errorf("read CA cert file: %v", err)
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCertPEM); !ok {
		return nil, fmt.Errorf("failed to parse CA cert %q", tlsCACertFile)
	}
	return pool, nil
}
