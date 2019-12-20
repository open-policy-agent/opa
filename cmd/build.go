// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/rego"
)

var buildParams = struct {
	outputFile  string
	debug       bool
	dataPaths   repeatedStringFlag
	ignore      []string
	bundlePaths repeatedStringFlag
}{}

var buildCommand = &cobra.Command{
	Use:   "build <query>",
	Short: "Compile Rego policy queries",
	Long: `Compile a Rego policy query into an executable for enforcement.

The 'build' command takes a policy query as input and compiles it into an
executable that can be loaded into an enforcement point and evaluated with
input values. By default, the build command produces WebAssembly (WASM)
executables.`,
	PreRunE: func(Cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("specify exactly one query argument")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := build(args); err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
	},
}

func build(args []string) error {

	ctx := context.Background()

	f := loaderFilter{
		Ignore: buildParams.ignore,
	}

	regoArgs := []func(*rego.Rego){
		rego.Query(args[0]),
	}

	if buildParams.dataPaths.isFlagSet() {
		regoArgs = append(regoArgs, rego.Load(buildParams.dataPaths.v, f.Apply))
	}

	if buildParams.bundlePaths.isFlagSet() {
		for _, bundleDir := range buildParams.bundlePaths.v {
			regoArgs = append(regoArgs, rego.LoadBundle(bundleDir))
		}
	}

	if buildParams.debug {
		regoArgs = append(regoArgs, rego.Dump(os.Stderr))
	}

	r := rego.New(regoArgs...)
	cr, err := r.Compile(ctx, rego.CompilePartial(false))
	if err != nil {
		return err
	}

	out, err := os.Create(buildParams.outputFile)
	if err != nil {
		return err
	}

	defer out.Close()

	_, err = out.Write(cr.Bytes)
	return err
}

func init() {
	buildCommand.Flags().StringVarP(&buildParams.outputFile, "output", "o", "policy.wasm", "set the filename of the compiled policy")
	buildCommand.Flags().BoolVarP(&buildParams.debug, "debug", "D", false, "enable debug output")
	buildCommand.Flags().VarP(&buildParams.dataPaths, "data", "d", "set data file(s) or directory path(s)")
	buildCommand.Flags().VarP(&buildParams.bundlePaths, "bundle", "b", "set bundle file(s) or directory path(s)")
	addIgnoreFlag(buildCommand.Flags(), &buildParams.ignore)
	RootCommand.AddCommand(buildCommand)
}
