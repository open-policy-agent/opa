// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/spf13/pflag"
)

func setMaxErrors(fs *pflag.FlagSet, errLimit *int) {
	fs.IntVarP(errLimit, "max-errors", "m", ast.CompileErrorLimitDefault, "set the number of errors to allow before compilation fails early")
}

func setIgnore(fs *pflag.FlagSet, ignoreNames *[]string) {
	fs.StringSliceVarP(ignoreNames, "ignore", "", []string{}, "set file and directory names to ignore during loading (e.g., '.*' excludes hidden files)")
}
