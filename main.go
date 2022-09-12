// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"os"

	"github.com/open-policy-agent/opa/cmd"
)

func main() {
	if err := cmd.RootCommand.Execute(); err != nil {
		os.Exit(1)
	}
}

// Capabilities + built-in metadata file generation:
//go:generate build/gen-run-go.sh internal/cmd/genopacapabilities/main.go capabilities.json
//go:generate build/gen-run-go.sh internal/cmd/genbuiltinmetadata/main.go builtin_metadata.json
