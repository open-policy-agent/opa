// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
)

// main demonstrates the loading and executing of OPA produced wasm
// policy binary. To execute run 'go run main.go .' in the directory
// of the main.go.
func main() {
	if len(os.Args) < 2 {
		fmt.Printf("%s: first argument must a path to a directory with example-1.wasm and example-2.wasm.\n", os.Args[0])
		return
	}

	directory := os.Args[1]

	// Setup the SDK

	policy, err := os.ReadFile(path.Join(directory, "example-1.wasm"))
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	rego, err := opa.New().WithPolicyBytes(policy).Init()
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	defer rego.Close()

	// Evaluate the policy once.

	var input interface{} = map[string]interface{}{
		"foo": true,
		"bar": false,
	}

	ctx := context.Background()

	eps, err := rego.Entrypoints(ctx)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	entrypointID, ok := eps["example/allow"]
	if !ok {
		fmt.Println("error: Unable to find entrypoint 'example/allow'")
		return
	}

	result, err := rego.Eval(ctx, opa.EvalOpts{Entrypoint: entrypointID, Input: &input})
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	fmt.Printf("Policy 1 result: %v\n", result)

	// Update the policy on the fly.

	policy, err = os.ReadFile(path.Join(directory, "example-2.wasm"))
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	// Get an updated entrypoint ID, they may have changed!
	eps, err = rego.Entrypoints(ctx)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	entrypointID, ok = eps["example/allow"]
	if !ok {
		fmt.Println("error: Unable to find entrypoint 'example/allow'")
		return
	}

	// Evaluate the new policy.

	if err := rego.SetPolicy(ctx, policy); err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	result, err = rego.Eval(ctx, opa.EvalOpts{Entrypoint: entrypointID, Input: &input})
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	fmt.Printf("Policy 2 result: %v\n", result)
}
