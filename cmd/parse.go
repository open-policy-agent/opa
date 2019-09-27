// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/util"
)

const (
	parseFormatPretty = "pretty"
	parseFormatJSON   = "json"
)

var parseParams = struct {
	format *util.EnumFlag
}{
	format: util.NewEnumFlag(parseFormatPretty, []string{parseFormatPretty, parseFormatJSON}),
}

var parseCommand = &cobra.Command{
	Use:   "parse <path>",
	Short: "Parse Rego source file",
	Long:  `Parse Rego source file and print AST.`,
	PreRunE: func(Cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("no source file specified")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(parse(args))
	},
}

func parse(args []string) int {

	if len(args) == 0 {
		return 0
	}

	result, err := loader.Rego(args[0])

	switch parseParams.format.String() {
	case parseFormatJSON:
		if err != nil {
			pr.JSON(os.Stderr, pr.Output{Errors: pr.NewOutputErrors(err)})
			return 1
		}

		bs, err := json.MarshalIndent(result.Parsed, "", "  ")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println(string(bs))
	default:
		ast.Pretty(os.Stdout, result.Parsed)
	}

	return 0
}

func init() {
	parseCommand.Flags().VarP(parseParams.format, "format", "f", "set output format")
	RootCommand.AddCommand(parseCommand)
}
