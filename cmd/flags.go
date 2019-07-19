// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util"
	"github.com/spf13/pflag"
)

func setMaxErrors(fs *pflag.FlagSet, errLimit *int) {
	fs.IntVarP(errLimit, "max-errors", "m", ast.CompileErrorLimitDefault, "set the number of errors to allow before compilation fails early")
}

func setIgnore(fs *pflag.FlagSet, ignoreNames *[]string) {
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

func setExplain(fs *pflag.FlagSet, explain *util.EnumFlag) {
	fs.VarP(explain, "explain", "", "enable query explanations")
}
