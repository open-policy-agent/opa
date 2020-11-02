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

// main loads a bundle either from a file or HTTP server.
//
// In the directory of the main.go, execute 'go run main.go
// bundle.tgz' to load the accompanied bundle file. Similarly, execute
// 'go run main.go http://url/to/bundle.tgz' to test the HTTP
// downloading from a HTTP server.
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
	result, err := rego.Eval(ctx, &input)
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
