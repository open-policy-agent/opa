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
	opaLoader "github.com/open-policy-agent/opa/internal/wasm/sdk/opa/loader"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa/loader/file"
	"github.com/open-policy-agent/opa/internal/wasm/sdk/opa/loader/http"
)

var (
	loader opaLoader.Loader
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
	var err error
	rego, err = opa.New().Init()
	if err != nil {
		return err
	}

	url, err := url.Parse(u)
	if err != nil {
		return err
	}

	switch url.Scheme {
	case "http", "https":
		loader, err = http.New(rego).
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
		loader, err = file.New(rego).
			WithFile(url.String()).
			WithInterval(10 * time.Second).
			Init()
	}

	if err != nil {
		return err
	}

	if err := loader.Start(context.Background()); err != nil {
		return err
	}

	return nil
}

func cleanup() {
	if loader != nil {
		loader.Close()
	}
	if rego != nil {
		rego.Close()
	}
}
