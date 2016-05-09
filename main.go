// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import "fmt"
import "os"
import "github.com/open-policy-agent/opa/cmd"

func main() {
	if err := cmd.RootCommand.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Rego parser generation:
//
//go:generate pigeon -o ast/parser.go ast/rego.peg
//go:generate goimports -w ast/parser.go
