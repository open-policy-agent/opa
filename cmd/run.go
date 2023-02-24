// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path"
	"time"

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
	rt                 runtime.Params
	tlsCertFile        string
	tlsPrivateKeyFile  string
	tlsCACertFile      string
	tlsCertRefresh     time.Duration
	ignore             []string
	serverMode         bool
	skipVersionCheck   bool // skipVersionCheck is deprecated. Use disableTelemetry instead
	disableTelemetry   bool
	authentication     *util.EnumFlag
	authorization      *util.EnumFlag
	minTLSVersion      *util.EnumFlag
	logLevel           *util.EnumFlag
	logFormat          *util.EnumFlag
	logTimestampFormat string
	algorithm          string
	scope              string
	pubKey             string
	pubKeyID           string
	skipBundleVerify   bool
	excludeVerifyFiles []string
}

func newRunParams() runCmdParams {
	return runCmdParams{
		rt:             runtime.NewParams(),
		authentication: util.NewEnumFlag("off", []string{"token", "tls", "off"}),
		authorization:  util.NewEnumFlag("off", []string{"basic", "off"}),
		minTLSVersion:  util.NewEnumFlag("1.2", []string{"1.0", "1.1", "1.2", "1.3"}),
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

    $ opa run repl.input:input.json

Which will load the "input.json" file at path "data.repl.input".

Use the "help input" command in the interactive shell to see more options.


File paths can be specified as URLs to resolve ambiguity in paths containing colons:

    $ opa run file:///c:/path/to/data.json

URL paths to remote public bundles (http or https) will be parsed as shorthand
configuration equivalent of using repeated --set flags to accomplish the same:

	$ opa run -s https://example.com/bundles/bundle.tar.gz

The above shorthand command is identical to:

    $ opa run -s --set "services.cli1.url=https://example.com" \
                 --set "bundles.cli1.service=cli1" \
                 --set "bundles.cli1.resource=/bundles/bundle.tar.gz" \
                 --set "bundles.cli1.persist=true"

The 'run' command can also verify the signature of a signed bundle.
A signed bundle is a normal OPA bundle that includes a file
named ".signatures.json". For more information on signed bundles
see https://www.openpolicyagent.org/docs/latest/management-bundles/#signing.

The key to verify the signature of signed bundle can be provided
using the --verification-key flag. For example, for RSA family of algorithms,
the command expects a PEM file containing the public key.
For HMAC family of algorithms (eg. HS256), the secret can be provided
using the --verification-key flag.

The --verification-key-id flag can be used to optionally specify a name for the
key provided using the --verification-key flag.

The --signing-alg flag can be used to specify the signing algorithm.
The 'run' command uses RS256 (by default) as the signing algorithm.

The --scope flag can be used to specify the scope to use for
bundle signature verification.

Example:

    $ opa run --verification-key secret --signing-alg HS256 --bundle bundle.tar.gz

The 'run' command will read the bundle "bundle.tar.gz", check the
".signatures.json" file and perform verification using the provided key.
An error will be generated if "bundle.tar.gz" does not contain a ".signatures.json" file.
For more information on the bundle verification process see
https://www.openpolicyagent.org/docs/latest/management-bundles/#signature-verification.

The 'run' command can ONLY be used with the --bundle flag to verify signatures
for existing bundle files or directories following the bundle structure.

To skip bundle verification, use the --skip-verify flag.
`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()
			rt, err := initRuntime(ctx, cmdParams, args)
			if err != nil {
				fmt.Println("error:", err)
				os.Exit(1)
			}
			startRuntime(ctx, rt, cmdParams.serverMode)
		},
	}

	addConfigFileFlag(runCommand.Flags(), &cmdParams.rt.ConfigFile)
	runCommand.Flags().BoolVarP(&cmdParams.serverMode, "server", "s", false, "start the runtime in server mode")
	runCommand.Flags().IntVar(&cmdParams.rt.ReadyTimeout, "ready-timeout", 0, "wait (in seconds) for configured plugins before starting server (value <= 0 disables ready check)")
	runCommand.Flags().StringVarP(&cmdParams.rt.HistoryPath, "history", "H", historyPath(), "set path of history file")
	cmdParams.rt.Addrs = runCommand.Flags().StringSliceP("addr", "a", []string{defaultAddr}, "set listening address of the server (e.g., [ip]:<port> for TCP, unix://<path> for UNIX domain socket)")
	cmdParams.rt.DiagnosticAddrs = runCommand.Flags().StringSlice("diagnostic-addr", []string{}, "set read-only diagnostic listening address of the server for /health and /metric APIs (e.g., [ip]:<port> for TCP, unix://<path> for UNIX domain socket)")
	runCommand.Flags().BoolVar(&cmdParams.rt.H2CEnabled, "h2c", false, "enable H2C for HTTP listeners")
	runCommand.Flags().StringVarP(&cmdParams.rt.OutputFormat, "format", "f", "pretty", "set shell output format, i.e, pretty, json")
	runCommand.Flags().BoolVarP(&cmdParams.rt.Watch, "watch", "w", false, "watch command line files for changes")
	addMaxErrorsFlag(runCommand.Flags(), &cmdParams.rt.ErrorLimit)
	runCommand.Flags().BoolVar(&cmdParams.rt.PprofEnabled, "pprof", false, "enables pprof endpoints")
	runCommand.Flags().StringVar(&cmdParams.tlsCertFile, "tls-cert-file", "", "set path of TLS certificate file")
	runCommand.Flags().StringVar(&cmdParams.tlsPrivateKeyFile, "tls-private-key-file", "", "set path of TLS private key file")
	runCommand.Flags().StringVar(&cmdParams.tlsCACertFile, "tls-ca-cert-file", "", "set path of TLS CA cert file")
	runCommand.Flags().DurationVar(&cmdParams.tlsCertRefresh, "tls-cert-refresh-period", 0, "set certificate refresh period")
	runCommand.Flags().Var(cmdParams.authentication, "authentication", "set authentication scheme")
	runCommand.Flags().Var(cmdParams.authorization, "authorization", "set authorization scheme")
	runCommand.Flags().Var(cmdParams.minTLSVersion, "min-tls-version", "set minimum TLS version to be used by OPA's server")
	runCommand.Flags().VarP(cmdParams.logLevel, "log-level", "l", "set log level")
	runCommand.Flags().Var(cmdParams.logFormat, "log-format", "set log format")
	runCommand.Flags().StringVar(&cmdParams.logTimestampFormat, "log-timestamp-format", "", "set log timestamp format (OPA_LOG_TIMESTAMP_FORMAT environment variable)")
	runCommand.Flags().IntVar(&cmdParams.rt.GracefulShutdownPeriod, "shutdown-grace-period", 10, "set the time (in seconds) that the server will wait to gracefully shut down")
	runCommand.Flags().IntVar(&cmdParams.rt.ShutdownWaitPeriod, "shutdown-wait-period", 0, "set the time (in seconds) that the server will wait before initiating shutdown")
	addConfigOverrides(runCommand.Flags(), &cmdParams.rt.ConfigOverrides)
	addConfigOverrideFiles(runCommand.Flags(), &cmdParams.rt.ConfigOverrideFiles)
	addBundleModeFlag(runCommand.Flags(), &cmdParams.rt.BundleMode, false)

	runCommand.Flags().BoolVar(&cmdParams.skipVersionCheck, "skip-version-check", false, "disables anonymous version reporting (see: https://www.openpolicyagent.org/docs/latest/privacy)")
	err := runCommand.Flags().MarkDeprecated("skip-version-check", "\"skip-version-check\" is deprecated. Use \"disable-telemetry\" instead")
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}

	runCommand.Flags().BoolVar(&cmdParams.disableTelemetry, "disable-telemetry", false, "disables anonymous information reporting (see: https://www.openpolicyagent.org/docs/latest/privacy)")
	addIgnoreFlag(runCommand.Flags(), &cmdParams.ignore)

	// bundle verification config
	addVerificationKeyFlag(runCommand.Flags(), &cmdParams.pubKey)
	addVerificationKeyIDFlag(runCommand.Flags(), &cmdParams.pubKeyID, defaultPublicKeyID)
	addSigningAlgFlag(runCommand.Flags(), &cmdParams.algorithm, defaultTokenSigningAlg)
	addBundleVerificationScopeFlag(runCommand.Flags(), &cmdParams.scope)
	addBundleVerificationSkipFlag(runCommand.Flags(), &cmdParams.skipBundleVerify, false)
	addBundleVerificationExcludeFilesFlag(runCommand.Flags(), &cmdParams.excludeVerifyFiles)

	usageTemplate := `Usage:
  {{.UseLine}} [files]

Flags:
{{.LocalFlags.FlagUsages | trimRightSpace}}
`

	runCommand.SetUsageTemplate(usageTemplate)

	RootCommand.AddCommand(runCommand)
}

func initRuntime(ctx context.Context, params runCmdParams, args []string) (*runtime.Runtime, error) {
	authenticationSchemes := map[string]server.AuthenticationScheme{
		"token": server.AuthenticationToken,
		"tls":   server.AuthenticationTLS,
		"off":   server.AuthenticationOff,
	}

	authorizationScheme := map[string]server.AuthorizationScheme{
		"basic": server.AuthorizationBasic,
		"off":   server.AuthorizationOff,
	}

	minTLSVersions := map[string]uint16{
		"1.0": tls.VersionTLS10,
		"1.1": tls.VersionTLS11,
		"1.2": tls.VersionTLS12,
		"1.3": tls.VersionTLS13,
	}

	cert, err := loadCertificate(params.tlsCertFile, params.tlsPrivateKeyFile)
	if err != nil {
		return nil, err
	}

	params.rt.CertificateFile = params.tlsCertFile
	params.rt.CertificateKeyFile = params.tlsPrivateKeyFile
	params.rt.CertificateRefresh = params.tlsCertRefresh

	if params.tlsCACertFile != "" {
		pool, err := loadCertPool(params.tlsCACertFile)
		if err != nil {
			return nil, err
		}
		params.rt.CertPool = pool
	}

	params.rt.Authentication = authenticationSchemes[params.authentication.String()]
	params.rt.Authorization = authorizationScheme[params.authorization.String()]
	params.rt.MinTLSVersion = minTLSVersions[params.minTLSVersion.String()]
	params.rt.Certificate = cert

	timestampFormat := params.logTimestampFormat
	if timestampFormat == "" {
		timestampFormat = os.Getenv("OPA_LOG_TIMESTAMP_FORMAT")
	}
	params.rt.Logging = runtime.LoggingConfig{
		Level:           params.logLevel.String(),
		Format:          params.logFormat.String(),
		TimestampFormat: timestampFormat,
	}
	params.rt.Paths = args
	params.rt.Filter = loaderFilter{
		Ignore: params.ignore,
	}.Apply

	params.rt.EnableVersionCheck = !params.disableTelemetry

	// For backwards compatibility, check if `--skip-version-check` flag set.
	if params.skipVersionCheck {
		params.rt.EnableVersionCheck = false
	}

	params.rt.SkipBundleVerification = params.skipBundleVerify

	bvc, err := buildVerificationConfig(params.pubKey, params.pubKeyID, params.algorithm, params.scope, params.excludeVerifyFiles)
	if err != nil {
		return nil, err
	}
	params.rt.BundleVerificationConfig = bvc

	if params.rt.BundleVerificationConfig != nil && !params.rt.BundleMode {
		return nil, fmt.Errorf("enable bundle mode (ie. --bundle) to verify bundle files or directories")
	}

	rt, err := runtime.NewRuntime(ctx, params.rt)
	if err != nil {
		return nil, err
	}

	rt.SetDistributedTracingLogging()

	return rt, nil
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
	caCertPEM, err := os.ReadFile(tlsCACertFile)
	if err != nil {
		return nil, fmt.Errorf("read CA cert file: %v", err)
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCertPEM); !ok {
		return nil, fmt.Errorf("failed to parse CA cert %q", tlsCACertFile)
	}
	return pool, nil
}
