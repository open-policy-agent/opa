// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package main

import (
	"context"
	"fmt"
	gohttp "net/http"
	"net/url"
	"os"
	"time"

	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa/file"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa/http"
)

var (
	loader opa.Loader
	rego   *opa.OPA
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("provide URL or file\n")
		return
	}

	url := os.Args[1]
	token := ""
	if len(os.Args) >= 3 {
		token = os.Args[2]
	}

	// Setup the SDK, either with HTTP bundle loader or file bundle loader.

	if err := setup(url, token); err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	defer cleanup()

	// Evaluate the policy.

	var input interface{} = map[string]interface{}{
		"foo": true,
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

	fmt.Printf("Policy result: %v\n", result)
}

func setup(u string, token string) error {
	r, err := opa.New().Init()
	if err != nil {
		return err
	}

	url, err := url.Parse(u)
	if err != nil {
		return err
	}

	var l opa.Loader

	switch url.Scheme {
	case "http", "https":
		l, err = http.New(r).
			WithURL(url.String()).
			WithPrepareRequest(func(req *gohttp.Request) error {
				if token != "" {
					req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
				}
				return nil
			}).
			WithInterval(30*time.Second, 60*time.Second).
			Init()
	case "file", "":
		l, err = file.New(r).
			WithFile(url.String()).
			WithInterval(10 * time.Second).
			Init()
	}

	if err != nil {
		return err
	}

	if err := l.Start(context.Background()); err != nil {
		return err
	}

	rego, loader = r, l
	return nil
}

func cleanup() {
	loader.Close()
	rego.Close()
}
