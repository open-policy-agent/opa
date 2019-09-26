// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/cmd"
)

func main() {
	if err := cmd.RootCommand.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Rego parser generation:
//
//go:generate build/run-pigeon.sh -o ast/parser.go ast/rego.peg
//go:generate build/run-goimports.sh -w ast/parser.go
//go:generate build/gen-opa-wasm.sh internal/cmd/genopawasm/main.go -o internal/compiler/wasm/opa/opa.go internal/compiler/wasm/opa/opa.wasm
