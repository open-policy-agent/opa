// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"path"

	"github.com/open-policy-agent/opa/runtime"
	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/util"
	"github.com/spf13/cobra"
)

const (
	defaultAddr                        = ":8181"        // default listening address for server
	defaultHistoryFile                 = ".opa_history" // default filename for shell history
	defaultServerDiagnosticsBufferSize = 10             // default number of items to keep in diagnostics buffer for server
)

var (
	logLevel                    *util.EnumFlag
	logFormat                   *util.EnumFlag
	authentication              *util.EnumFlag
	authorization               *util.EnumFlag
	serverMode                  bool
	tlsCertFile                 string
	tlsPrivateKeyFile           string
	authenticationSchemes       map[string]server.AuthenticationScheme
	authorizationSchemes        map[string]server.AuthorizationScheme
	serverDiagnosticsBufferSize int
	params                      runtime.Params
)

func setAuthnSchemesMap() {
	authenticationSchemes = map[string]server.AuthenticationScheme{
		"token": server.AuthenticationToken,
		"off":   server.AuthenticationOff,
	}
}

func getAuthnSchemesMap() map[string]server.AuthenticationScheme {
	return authenticationSchemes
}

func setDefaultAuthnSchemeFlag() {
	authentication = util.NewEnumFlag("off", []string{"token", "off"})
}

func setAuthzSchemesMap() {
	authorizationSchemes = map[string]server.AuthorizationScheme{
		"basic": server.AuthorizationBasic,
		"off":   server.AuthorizationOff,
	}
}

func getAuthzSchemesMap() map[string]server.AuthorizationScheme {
	return authorizationSchemes
}

func setDefaultAuthzSchemeFlag() {
	authorization = util.NewEnumFlag("off", []string{"basic", "off"})
}

func setDefaultLogLevelFlag() {
	logLevel = util.NewEnumFlag("info", []string{"debug", "info", "error"})
}

func setDefaultLogFormatFlag() {
	logFormat = util.NewEnumFlag("text", []string{"text", "json"})
}

func setupRuntimeDefaults() {

	// set default authentication schemes
	setAuthnSchemesMap()
	setDefaultAuthnSchemeFlag()

	// set default authorization schemes
	setAuthzSchemesMap()
	setDefaultAuthzSchemeFlag()

	// set default log level and format
	setDefaultLogLevelFlag()
	setDefaultLogFormatFlag()

	// OPA runtime config
	params = runtime.NewParams()
}

func GetOpaParams() runtime.Params {
	return params
}

func setOpaParams(parameters *runtime.Params, args []string) {

	cert, err := loadCertificate(tlsCertFile, tlsPrivateKeyFile)
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	parameters.Authentication = getAuthnSchemesMap()[authentication.String()]
	parameters.Authorization = getAuthzSchemesMap()[authorization.String()]
	parameters.Certificate = cert
	parameters.Logging = runtime.LoggingConfig{
		Level:  logLevel.String(),
		Format: logFormat.String(),
	}
	parameters.DiagnosticsBuffer = server.NewBoundedBuffer(serverDiagnosticsBufferSize)
	parameters.Paths = args
}

func getOpaRuntime(ctx context.Context, params runtime.Params) (*runtime.Runtime, error) {
	return runtime.NewRuntime(ctx, params)
}

func StartOPAInServerMode(ctx context.Context, rt *runtime.Runtime) {
	rt.StartServer(ctx)
}

func StartOPAInReplMode(ctx context.Context, rt *runtime.Runtime) {
	rt.StartREPL(ctx)
}

// New returns a new instance of the OPA runtime
func New(ctx context.Context, args []string, runCommand *cobra.Command) (*runtime.Runtime, error) {

	// initial OPA setup
	setupRuntimeDefaults()

	// parse OPA's command line options
	parseOpaCliOptions(runCommand)

	// get OPA's runtime config
	opaParams := GetOpaParams()

	// set OPA's runtime config after parsing command line
	setOpaParams(&opaParams, args)

	return getOpaRuntime(ctx, opaParams)
}

func parseOpaCliOptions(runCommand *cobra.Command) {

	runCommand.Flags().StringVarP(&params.ConfigFile, "config-file", "c", "", "set path of configuration file")
	runCommand.Flags().BoolVarP(&serverMode, "server", "s", false, "start the runtime in server mode")
	runCommand.Flags().StringVarP(&params.HistoryPath, "history", "H", historyPath(), "set path of history file")
	runCommand.Flags().StringVarP(&params.Addr, "addr", "a", defaultAddr, "set listening address of the server")
	runCommand.Flags().StringVarP(&params.InsecureAddr, "insecure-addr", "", "", "set insecure listening address of the server")
	runCommand.Flags().StringVarP(&params.OutputFormat, "format", "f", "pretty", "set shell output format, i.e, pretty, json")
	runCommand.Flags().BoolVarP(&params.Watch, "watch", "w", false, "watch command line files for changes")
	setMaxErrors(runCommand.Flags(), &params.ErrorLimit)
	runCommand.Flags().IntVarP(&serverDiagnosticsBufferSize, "server-diagnostics-buffer-size", "", defaultServerDiagnosticsBufferSize, "set the size of the server's diagnostics buffer")
	runCommand.Flags().StringVarP(&tlsCertFile, "tls-cert-file", "", "", "set path of TLS certificate file")
	runCommand.Flags().StringVarP(&tlsPrivateKeyFile, "tls-private-key-file", "", "", "set path of TLS private key file")
	runCommand.Flags().VarP(authentication, "authentication", "", "set authentication scheme")
	runCommand.Flags().VarP(authorization, "authorization", "", "set authorization scheme")
	runCommand.Flags().VarP(logLevel, "log-level", "l", "set log level")
	runCommand.Flags().VarP(logFormat, "log-format", "", "set log format")
}

func run(cmd *cobra.Command, args []string) {
	setOpaParams(&params, args)

	ctx := context.Background()

	rt, err := getOpaRuntime(ctx, params)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if serverMode {
		StartOPAInServerMode(ctx, rt)
	} else {
		StartOPAInReplMode(ctx, rt)
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
