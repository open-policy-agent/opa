// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/storage/inmem"

	"github.com/open-policy-agent/opa/rego"
	"github.com/spf13/cobra"
)

var buildParams = struct {
	outputFile string
	debug      bool
	dataPaths  repeatedStringFlag
	ignore     []string
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
		if len(args) == 0 {
			return fmt.Errorf("specify query argument")
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

	loaded, err := loader.Filtered(buildParams.dataPaths.v, f.Apply)
	if err != nil {
		return err
	}

	regoArgs = append(regoArgs, rego.Store(inmem.NewFromObject(loaded.Documents)))
	for _, file := range loaded.Modules {
		regoArgs = append(regoArgs, rego.Module(file.Name, string(file.Raw)))
	}

	if buildParams.debug {
		regoArgs = append(regoArgs, rego.Dump(os.Stderr))
	}

	r := rego.New(regoArgs...)
	cr, err := r.Compile(ctx)
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
	setIgnore(buildCommand.Flags(), &buildParams.ignore)
	RootCommand.AddCommand(buildCommand)
}
