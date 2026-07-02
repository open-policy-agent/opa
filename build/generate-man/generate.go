// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/open-policy-agent/opa/cmd"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Required argument: man pages output directory")
	}
	out := os.Args[1]
	err := os.MkdirAll(out, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	cmd := cmd.RootCommand
	cmd.Use = "opa [command]"
	cmd.DisableAutoGenTag = true

	header := &doc.GenManHeader{
		Title:   "Open Policy Agent",
		Section: "1",
		Source:  " ",
	}

	err = doc.GenManTree(cmd, header, out)
	if err != nil {
		log.Fatal(err)
	}
}
