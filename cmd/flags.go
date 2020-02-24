// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
)

func addFailFlag(fs *pflag.FlagSet, fail *bool, value bool) {
	fs.BoolVarP(fail, "fail", "", value, "exits with non-zero exit code on undefined/empty result and errors")
}

func addDataFlag(fs *pflag.FlagSet, paths *repeatedStringFlag) {
	fs.VarP(paths, "data", "d", "set data file(s) or directory path(s)")
}

func addBundleFlag(fs *pflag.FlagSet, paths *repeatedStringFlag) {
	fs.VarP(paths, "bundle", "b", "set bundle file(s) or directory path(s)")
}

func addInputFlag(fs *pflag.FlagSet, inputPath *string) {
	fs.StringVarP(inputPath, "input", "i", "", "set input file path")
}

func addImportFlag(fs *pflag.FlagSet, imports *repeatedStringFlag) {
	fs.VarP(imports, "import", "", "set query import(s)")
}

func addPackageFlag(fs *pflag.FlagSet, pkg *string) {
	fs.StringVarP(pkg, "package", "", "", "set query package")
}

func addQueryStdinFlag(fs *pflag.FlagSet, stdin *bool) {
	fs.BoolVarP(stdin, "stdin", "", false, "read query from stdin")
}

func addInputStdinFlag(fs *pflag.FlagSet, stdinInput *bool) {
	fs.BoolVarP(stdinInput, "stdin-input", "I", false, "read input document from stdin")
}

func addMetricsFlag(fs *pflag.FlagSet, metrics *bool, value bool) {
	fs.BoolVarP(metrics, "metrics", "", value, "report query performance metrics")
}

func addOutputFormat(fs *pflag.FlagSet, outputFormat *util.EnumFlag) {
	fs.VarP(outputFormat, "format", "f", "set output format")
}

func addBenchmemFlag(fs *pflag.FlagSet, benchMem *bool, value bool) {
	fs.BoolVar(benchMem, "benchmem", value, "report memory allocations with benchmark results")
}

func addCountFlag(fs *pflag.FlagSet, count *int, cmdType string) {
	fs.IntVar(count, "count", 1, fmt.Sprintf("number of times to repeat each %s (default 1)", cmdType))
}

func addMaxErrorsFlag(fs *pflag.FlagSet, errLimit *int) {
	fs.IntVarP(errLimit, "max-errors", "m", ast.CompileErrorLimitDefault, "set the number of errors to allow before compilation fails early")
}

func addIgnoreFlag(fs *pflag.FlagSet, ignoreNames *[]string) {
	fs.StringSliceVarP(ignoreNames, "ignore", "", []string{}, "set file and directory names to ignore during loading (e.g., '.*' excludes hidden files)")
}

const (
	explainModeOff   = "off"
	explainModeFull  = "full"
	explainModeNotes = "notes"
	explainModeFails = "fails"
)

func newExplainFlag(modes []string) *util.EnumFlag {
	return util.NewEnumFlag(modes[0], modes)
}

func setExplainFlag(fs *pflag.FlagSet, explain *util.EnumFlag) {
	fs.VarP(explain, "explain", "", "enable query explanations")
}
