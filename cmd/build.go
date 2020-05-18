// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/util"
)

type buildParams struct {
	target            *util.EnumFlag
	bundleMode        bool
	optimizationLevel int
	entrypoints       repeatedStringFlag
	outputFile        string
	revision          string
	ignore            []string
	debug             bool
}

func newBuildParams() buildParams {
	var buildParams buildParams
	buildParams.target = util.NewEnumFlag(compile.TargetRego, []string{compile.TargetRego, compile.TargetWasm})
	return buildParams
}

func init() {

	buildParams := newBuildParams()

	var buildCommand = &cobra.Command{
		Use:   "build <path> [<path> [...]]",
		Short: "Build an OPA bundle",
		Long: `Build an OPA bundle.

The 'build' command packages OPA policy and data files into bundles. Bundles are
gzipped tarballs containing policies and data. Paths referring to directories are
loaded recursively.

	$ ls
	example.rego

	$ opa build -b .

You can load bundles into OPA on the command-line:

	$ ls
	bundle.tar.gz example.rego

	$ opa run bundle.tar.gz

You can also configure OPA to download bundles from remote HTTP endpoints:

	$ opa run --server \
		--set bundles.example.resource=bundle.tar.gz \
		--set services.example.url=http://localhost:8080

Inside another terminal in the same directory, serve the bundle via HTTP:

	$ python3 -m http.server --bind localhost 8080

For more information on bundles see https://www.openpolicyagent.org/docs/latest/management.

When -b is specified the 'build' command assumes paths refer to existing bundle files
or directories following the bundle structure. If multiple bundles are provided, their
contents are merged. If there are any merge conflicts (e.g., due to conflicting bundle
roots), the command fails.

The -O flag controls the optimization level. By default, optimization is disabled (-O=0).
When optimization is enabled the 'build' command generates a bundle that is semantically
equivalent to the input files however the structure of the files in the bundle may have
been changed by rewriting, inlining, pruning, etc. Higher optimization levels may result
in longer build times.

The 'build' command supports targets (specified by -t):

    rego    The default target emits a bundle containing a set of policy and data files
            that are semantically equivalent to the input files. If optimizations are
            disabled the output may simply contain a copy of the input policy and data
            files. If optimization is enabled at least one entrypoint (-e) must be supplied.

    wasm    The wasm target emits a bundle containing a WebAssembly module compiled from
            the input files. The bundle may contain the original policy or data files.
            The wasm target requires exactly one entrypoint (-e) be supplied.

The -e flag tells the 'build' command which documents will be queried by the software
asking for policy decisions, so that it can focus optimization efforts and ensure
that document is not eliminated by the optimizer.`,
		PreRunE: func(Cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("expected at least one path")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := dobuild(buildParams, args); err != nil {
				fmt.Println("error:", err)
				os.Exit(1)
			}
		},
	}

	buildCommand.Flags().VarP(buildParams.target, "target", "t", "set the output bundle target type")
	buildCommand.Flags().BoolVarP(&buildParams.debug, "debug", "", false, "enable debug output")
	buildCommand.Flags().IntVarP(&buildParams.optimizationLevel, "optimize", "O", 0, "set optimization level")
	buildCommand.Flags().VarP(&buildParams.entrypoints, "entrypoint", "e", "set slash separated entrypoint path")
	buildCommand.Flags().StringVarP(&buildParams.revision, "revision", "r", "", "set output bundle revision")
	buildCommand.Flags().StringVarP(&buildParams.outputFile, "output", "o", "bundle.tar.gz", "set the output filename")
	addBundleModeFlag(buildCommand.Flags(), &buildParams.bundleMode, false)
	addIgnoreFlag(buildCommand.Flags(), &buildParams.ignore)
	RootCommand.AddCommand(buildCommand)
}

func dobuild(params buildParams, args []string) error {

	buf := bytes.NewBuffer(nil)

	compiler := compile.New().
		WithTarget(params.target.String()).
		WithAsBundle(params.bundleMode).
		WithOptimizationLevel(params.optimizationLevel).
		WithOutput(buf).
		WithEntrypoints(params.entrypoints.v...).
		WithPaths(args...).
		WithFilter(buildCommandLoaderFilter(params.bundleMode, params.ignore)).
		WithRevision(params.revision)

	err := compiler.Build(context.Background())

	if params.debug {
		printdebug(os.Stderr, compiler.Debug())
	}

	if err != nil {
		return err
	}

	out, err := os.Create(params.outputFile)
	if err != nil {
		return err
	}

	_, err = io.Copy(out, buf)
	if err != nil {
		return err
	}

	return out.Close()
}

func buildCommandLoaderFilter(bundleMode bool, ignore []string) func(string, os.FileInfo, int) bool {
	return func(abspath string, info os.FileInfo, depth int) bool {
		if !bundleMode {
			if !info.IsDir() && strings.HasSuffix(abspath, ".tar.gz") {
				return true
			}
		}
		return loaderFilter{Ignore: ignore}.Apply(abspath, info, depth)
	}
}

func printdebug(w io.Writer, debug []compile.Debug) {
	for i := range debug {
		fmt.Fprintln(w, debug[i])
	}
}
