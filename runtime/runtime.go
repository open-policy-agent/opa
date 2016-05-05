// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/eval"
)

// Params stores the configuration for an OPA instance.
type Params struct {
	Server      bool
	Paths       []string
	HistoryPath string
}

// Runtime represents a single OPA instance.
type Runtime struct {
	Store *eval.Storage
}

// Start is the entry point of an OPA instance.
func (rt *Runtime) Start(params *Params) {

	store, err := eval.NewStorageFromFiles(params.Paths)

	if err != nil {
		fmt.Println("failed to open storage:", err)
		os.Exit(1)
	}

	rt.Store = store

	if !params.Server {
		rt.runRepl(params)
	} else {
		rt.runServer(params)
	}
}

func (rt *Runtime) runServer(params *Params) {
	fmt.Println("not implemented: server mode")
	os.Exit(1)
}

func (rt *Runtime) runRepl(params *Params) {

	repl := NewRepl(rt, params.HistoryPath, os.Stdout)
	repl.Loop()
}
