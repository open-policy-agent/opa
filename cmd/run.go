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

func init() {

	var serverMode bool
	var tlsCertFile, tlsPrivateKeyFile, tlsCACertFile string
	var ignore []string

	authentication := util.NewEnumFlag("off", []string{"token", "tls", "off"})

	authenticationSchemes := map[string]server.AuthenticationScheme{
		"token": server.AuthenticationToken,
		"tls":   server.AuthenticationTLS,
		"off":   server.AuthenticationOff,
	}

	authorization := util.NewEnumFlag("off", []string{"basic", "off"})

	authorizationScheme := map[string]server.AuthorizationScheme{
		"basic": server.AuthorizationBasic,
		"off":   server.AuthorizationOff,
	}

	logLevel := util.NewEnumFlag("info", []string{"debug", "info", "error"})
	logFormat := util.NewEnumFlag("json", []string{"text", "json", "json-pretty"})

	params := runtime.NewParams()

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
		Run: func(cmd *cobra.Command, args []string) {

			cert, err := loadCertificate(tlsCertFile, tlsPrivateKeyFile)
			if err != nil {
				fmt.Println("error:", err)
				os.Exit(1)
			}

			if tlsCACertFile != "" {
				pool, err := loadCertPool(tlsCACertFile)
				if err != nil {
					fmt.Println("error:", err)
					os.Exit(1)
				}
				params.CertPool = pool
			}

			params.Authentication = authenticationSchemes[authentication.String()]
			params.Authorization = authorizationScheme[authorization.String()]
			params.Certificate = cert
			params.Logging = runtime.LoggingConfig{
				Level:  logLevel.String(),
				Format: logFormat.String(),
			}
			params.Paths = args
			params.Filter = loaderFilter{
				Ignore: ignore,
			}.Apply

			ctx := context.Background()

			rt, err := runtime.NewRuntime(ctx, params)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}

			if serverMode {
				rt.StartServer(ctx)
			} else {
				rt.StartREPL(ctx)
			}
		},
	}

	runCommand.Flags().StringVarP(&params.ConfigFile, "config-file", "c", "", "set path of configuration file")
	runCommand.Flags().BoolVarP(&serverMode, "server", "s", false, "start the runtime in server mode")
	runCommand.Flags().StringVarP(&params.HistoryPath, "history", "H", historyPath(), "set path of history file")
	params.Addrs = runCommand.Flags().StringSliceP("addr", "a", []string{defaultAddr}, "set listening address of the server (e.g., [ip]:<port> for TCP, unix://<path> for UNIX domain socket)")
	runCommand.Flags().StringVarP(&params.InsecureAddr, "insecure-addr", "", "", "set insecure listening address of the server")
	runCommand.Flags().MarkDeprecated("insecure-addr", "use --addr instead")
	runCommand.Flags().StringVarP(&params.OutputFormat, "format", "f", "pretty", "set shell output format, i.e, pretty, json")
	runCommand.Flags().BoolVarP(&params.Watch, "watch", "w", false, "watch command line files for changes")
	setMaxErrors(runCommand.Flags(), &params.ErrorLimit)
	runCommand.Flags().BoolVarP(&params.PprofEnabled, "pprof", "", false, "enables pprof endpoints")
	runCommand.Flags().StringVarP(&tlsCertFile, "tls-cert-file", "", "", "set path of TLS certificate file")
	runCommand.Flags().StringVarP(&tlsPrivateKeyFile, "tls-private-key-file", "", "", "set path of TLS private key file")
	runCommand.Flags().StringVarP(&tlsCACertFile, "tls-ca-cert-file", "", "", "set path of TLS CA cert file")
	runCommand.Flags().VarP(authentication, "authentication", "", "set authentication scheme")
	runCommand.Flags().VarP(authorization, "authorization", "", "set authorization scheme")
	runCommand.Flags().VarP(logLevel, "log-level", "l", "set log level")
	runCommand.Flags().VarP(logFormat, "log-format", "", "set log format")
	runCommand.Flags().IntVar(&params.GracefulShutdownPeriod, "shutdown-grace-period", 10, "set the time (in seconds) that the server will wait to gracefully shut down")
	runCommand.Flags().StringArrayVar(&params.ConfigOverrides, "set", []string{}, "override config values on the command line (use commas to specify multiple values)")
	runCommand.Flags().StringArrayVar(&params.ConfigOverrideFiles, "set-file", []string{}, "override config values with files on the command line (use commas to specify multiple values)")
	setIgnore(runCommand.Flags(), &ignore)

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
