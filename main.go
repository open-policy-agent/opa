// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"os"

	"github.com/open-policy-agent/opa/cmd"
)

func main() {
	var exit int
	defer func() {
		if exit != 0 {
			os.Exit(exit)
		}
	}() // orderly shutdown, run all defer routines

	if err := cmd.RootCommand.Execute(); err != nil {
		var e *cmd.ExitError
		if errors.As(err, &e) {
			exit = e.Exit
		} else {
			exit = 1
		}
	}
}

//go:generate build/gen-run-go.sh internal/cmd/genopacapabilities/main.go capabilities.json
//go:generate build/gen-run-go.sh internal/cmd/genbuiltinmetadata/main.go builtin_metadata.json
//go:generate build/gen-run-go.sh internal/cmd/genversionindex/main.go v1/ast/version_index.json
