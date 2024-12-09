package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/cmd/internal/env"
	"github.com/open-policy-agent/opa/cmd/internal/exec"
	"github.com/open-policy-agent/opa/internal/config"
	internal_logging "github.com/open-policy-agent/opa/internal/logging"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/plugins"
	"github.com/open-policy-agent/opa/v1/plugins/bundle"
	"github.com/open-policy-agent/opa/v1/plugins/discovery"
	"github.com/open-policy-agent/opa/v1/plugins/logs"
	"github.com/open-policy-agent/opa/v1/plugins/status"
	"github.com/open-policy-agent/opa/v1/sdk"
	"github.com/open-policy-agent/opa/v1/util"
)

func init() {

	var bundlePaths repeatedStringFlag

	params := exec.NewParams(os.Stdout)

	var cmd = &cobra.Command{
		Use:   `exec <path> [<path> [...]]`,
		Short: "Execute against input files",
		Long: `Execute against input files.

The 'exec' command executes OPA against one or more input files. If the paths
refer to directories, OPA will execute against files contained inside those
directories, recursively.

The 'exec' command accepts a --config-file/-c or series of --set options as
arguments. These options behave the same as way as 'opa run'. Since the 'exec'
command is intended to execute OPA in one-shot, the 'exec' command will
manually trigger plugins before and after policy execution:

Before: Discovery -> Bundle -> Status
After: Decision Logs

By default, the 'exec' command executes the "default decision" (specified in
the OPA configuration) against each input file. This can be overridden by
specifying the --decision argument and pointing at a specific policy decision,
e.g., opa exec --decision /foo/bar/baz ...
`,

		Example: fmt.Sprintf(`  Loading input from stdin:
    %s exec [<path> [...]] --stdin-input [flags]
`, RootCommand.Use),

		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return env.CmdFlags.CheckEnvironmentVariables(cmd)
		},
		Run: func(_ *cobra.Command, args []string) {
			params.Paths = args
			params.BundlePaths = bundlePaths.v
			if err := runExec(params); err != nil {
				logging.Get().WithFields(map[string]interface{}{"err": err}).Error("Unexpected error.")
				os.Exit(1)
			}
		},
	}

	addBundleFlag(cmd.Flags(), &bundlePaths)
	addOutputFormat(cmd.Flags(), params.OutputFormat)
	addConfigFileFlag(cmd.Flags(), &params.ConfigFile)
	addConfigOverrides(cmd.Flags(), &params.ConfigOverrides)
	addConfigOverrideFiles(cmd.Flags(), &params.ConfigOverrideFiles)
	cmd.Flags().StringVarP(&params.Decision, "decision", "", "", "set decision to evaluate")
	cmd.Flags().BoolVarP(&params.FailDefined, "fail-defined", "", false, "exits with non-zero exit code on defined result and errors")
	cmd.Flags().BoolVarP(&params.Fail, "fail", "", false, "exits with non-zero exit code on undefined result and errors")
	cmd.Flags().BoolVarP(&params.FailNonEmpty, "fail-non-empty", "", false, "exits with non-zero exit code on non-empty result and errors")
	cmd.Flags().VarP(params.LogLevel, "log-level", "l", "set log level")
	cmd.Flags().Var(params.LogFormat, "log-format", "set log format")
	cmd.Flags().StringVar(&params.LogTimestampFormat, "log-timestamp-format", "", "set log timestamp format (OPA_LOG_TIMESTAMP_FORMAT environment variable)")
	cmd.Flags().BoolVarP(&params.StdIn, "stdin-input", "I", false, "read input document from stdin rather than a static file")
	cmd.Flags().DurationVar(&params.Timeout, "timeout", 0, "set exec timeout with a Go-style duration, such as '5m 30s'. (default unlimited)")
	addV0CompatibleFlag(cmd.Flags(), &params.V0Compatible, false)
	addV1CompatibleFlag(cmd.Flags(), &params.V1Compatible, false)

	RootCommand.AddCommand(cmd)
}

func runExec(params *exec.Params) error {
	ctx := context.Background()
	if params.Timeout != 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}
	return runExecWithContext(ctx, params)
}

func runExecWithContext(ctx context.Context, params *exec.Params) error {
	if minimumInputErr := validateMinimumInput(params); minimumInputErr != nil {
		return minimumInputErr
	}

	stdLogger, consoleLogger, err := setupLogging(params.LogLevel.String(), params.LogFormat.String(), params.LogTimestampFormat)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	if params.Logger != nil {
		stdLogger = params.Logger
	}

	config, err := setupConfig(params.ConfigFile, params.ConfigOverrides, params.ConfigOverrideFiles, params.BundlePaths)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	ready := make(chan struct{})

	opa, err := sdk.New(ctx, sdk.Options{
		Config:        bytes.NewReader(config),
		Logger:        stdLogger,
		ConsoleLogger: consoleLogger,
		Ready:         ready,
		V0Compatible:  params.V0Compatible,
		V1Compatible:  params.V1Compatible,
	})
	if err != nil {
		return fmt.Errorf("runtime error: %w", err)
	}

	if err := triggerPlugins(ctx, opa, []string{discovery.Name, bundle.Name, status.Name}); err != nil {
		return fmt.Errorf("runtime error: %w", err)
	}

	select {
	case <-ctx.Done():
		err := ctx.Err()
		if err == context.DeadlineExceeded {
			return fmt.Errorf("exec error: timed out before OPA was ready. This can happen when a remote bundle is malformed, or the timeout is set too low for normal OPA initialization")
		}
		// Note(philipc): Previously, exec would simply eat the context
		// cancellation error. We now propagate that upwards to the caller.
		return err
	case <-ready:
		// Do nothing; proceed as normal.
	}

	if err := exec.Exec(ctx, opa, params); err != nil {
		return fmt.Errorf("exec error: %w", err)
	}

	if err := triggerPlugins(ctx, opa, []string{logs.Name}); err != nil {
		return fmt.Errorf("runtime error: %w", err)
	}

	return nil
}

func triggerPlugins(ctx context.Context, opa *sdk.OPA, names []string) error {
	for _, name := range names {
		if p, ok := opa.Plugin(name).(plugins.Triggerable); ok {
			if err := p.Trigger(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func setupLogging(level, format, timestampFormat string) (logging.Logger, logging.Logger, error) {

	lvl, err := internal_logging.GetLevel(level)
	if err != nil {
		return nil, nil, err
	}

	if timestampFormat == "" {
		timestampFormat = os.Getenv("OPA_LOG_TIMESTAMP_FORMAT")
	}

	logging.Get().SetFormatter(internal_logging.GetFormatter(format, timestampFormat))
	logging.Get().SetLevel(lvl)

	stdLogger := logging.New()
	stdLogger.SetLevel(lvl)
	stdLogger.SetFormatter(internal_logging.GetFormatter(format, timestampFormat))

	consoleLogger := logging.New()
	consoleLogger.SetFormatter(internal_logging.GetFormatter(format, timestampFormat))

	return stdLogger, consoleLogger, nil
}

func setupConfig(file string, overrides []string, overrideFiles []string, bundlePaths []string) ([]byte, error) {

	bs, err := config.Load(file, overrides, overrideFiles)
	if err != nil {
		return nil, err
	}

	var root map[string]interface{}

	if err := util.Unmarshal(bs, &root); err != nil {
		return nil, err
	}

	if err := injectExplicitBundles(root, bundlePaths); err != nil {
		return nil, err
	}

	// NOTE(tsandall): This could be generalized in the future if we need to
	// deal with arbitrary plugins.

	// NOTE(tsandall): Overriding the discovery trigger mode to manual means
	// that all plugins will inherit the trigger mode by default. If the plugin
	// trigger mode is explicitly set to something other than 'manual' this will
	// result in a configuration error.
	if cfg, ok := root["discovery"].(map[string]interface{}); ok {
		cfg["trigger"] = "manual"
	}

	if cfg, ok := root["bundles"].(map[string]interface{}); ok {
		for _, x := range cfg {
			if bcfg, ok := x.(map[string]interface{}); ok {
				bcfg["trigger"] = "manual"
			}
		}
	}

	if cfg, ok := root["decision_logs"].(map[string]interface{}); ok {
		if rcfg, ok := cfg["reporting"].(map[string]interface{}); ok {
			rcfg["trigger"] = "manual"
		}
	}

	if cfg, ok := root["status"].(map[string]interface{}); ok {
		cfg["trigger"] = "manual"
	}

	return json.Marshal(root)
}

func injectExplicitBundles(root map[string]interface{}, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	bundles, ok := root["bundles"].(map[string]interface{})
	if !ok {
		bundles = map[string]interface{}{}
		root["bundles"] = bundles
	}

	for i := range paths {
		abspath, err := filepath.Abs(paths[i])
		if err != nil {
			return err
		}
		abspath = filepath.ToSlash(abspath)
		bundles[fmt.Sprintf("~%d", i)] = map[string]interface{}{
			"resource": fmt.Sprintf("file://%v", abspath),
		}
	}

	return nil
}

func validateMinimumInput(params *exec.Params) error {
	if !params.StdIn && len(params.Paths) == 0 {
		return errors.New("requires at least 1 path arg, or the --stdin-input flag")
	}
	return nil
}
