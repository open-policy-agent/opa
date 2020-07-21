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

// Capabilities file generation:
//go:generate build/gen-run-go.sh internal/cmd/genopacapabilities/main.go capabilities.json

// WASM base binary generation:
//go:generate build/gen-run-go.sh internal/cmd/genopawasm/main.go -o internal/compiler/wasm/opa/opa.go internal/compiler/wasm/opa/opa.wasm
