// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Command gen fills in the want_errors fixtures of the compiler conformance
// corpus.
package main

import (
	"fmt"
	"os"

	cases "github.com/open-policy-agent/opa/build/generate-compiler-cases"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: main <corpus-dir>")
		os.Exit(1)
	}

	if err := cases.Generate(os.Args[1]); err != nil {
		fmt.Println("Error generating compiler test cases:", err)
		os.Exit(1)
	}
}
