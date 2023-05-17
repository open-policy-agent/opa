// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm
// +build opa_wasm

package authz

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/test/authz"
	testAuthz "github.com/open-policy-agent/opa/test/authz"
	"github.com/open-policy-agent/opa/test/e2e"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

var testRuntime *e2e.TestRuntime

var queryPath = strings.Replace(authz.AllowQuery, ".", "/", -1)

func TestMain(m *testing.M) {
	flag.Parse()

	testServerParams := e2e.NewAPIServerTestParams()
	var cleanup func() error
	testServerParams.DiskStorage, cleanup = diskStorage()
	var err error
	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		panic(err)
	}

	files := map[string]string{
		"policy.rego": testAuthz.Policy,
		".manifest":   `{"roots": ["/policy/restauthz"]}`,
	}

	bundleBuf := bytes.Buffer{}
	ctx := context.Background()

	// TODO: Is there an option to do this more easily?
	// Would be cool to be able to pass already loaded data/files into the compile stuff
	test.WithTempFS(files, func(rootDir string) {
		err = compile.New().
			WithAsBundle(true).
			WithEntrypoints(strings.TrimPrefix(queryPath, "data/")). // Entrypoints shouldn't be "data" prefixed
			WithOutput(&bundleBuf).
			WithPaths(rootDir).
			WithTarget(compile.TargetWasm).
			Build(ctx)

		if err != nil {
			panic(err)
		}
	})

	testBundle, err := bundle.NewCustomReader(bundle.NewTarballLoader(&bundleBuf)).Read()
	if err != nil {
		panic(err)
	}

	// Sneak the bundle in...
	err = storage.Txn(ctx, testRuntime.Runtime.Store, storage.WriteParams, func(txn storage.Transaction) error {
		compiler := ast.NewCompiler().WithPathConflictsCheck(storage.NonEmpty(ctx, testRuntime.Runtime.Store, txn))
		m := metrics.New()

		activation := &bundle.ActivateOpts{
			Ctx:      ctx,
			Store:    testRuntime.Runtime.Store,
			Txn:      txn,
			Compiler: compiler,
			Metrics:  m,
			Bundles:  map[string]*bundle.Bundle{"bundle1": &testBundle},
		}

		return bundle.Activate(activation)
	})
	if err != nil {
		panic(err)
	}

	errc := testRuntime.RunTests(m)
	if errc == 0 && cleanup != nil {
		if err := cleanup(); err != nil {
			panic(err)
		}
	}
	os.Exit(errc)
}

func BenchmarkRESTAuthzForbidAuthn(b *testing.B) {
	runAuthzBenchmark(b, testAuthz.ForbidIdentity, 10)
}

func BenchmarkRESTAuthzForbidPath(b *testing.B) {
	runAuthzBenchmark(b, testAuthz.ForbidPath, 10)
}

func BenchmarkRESTAuthzForbidMethod(b *testing.B) {
	runAuthzBenchmark(b, testAuthz.ForbidMethod, 10)
}

func BenchmarkRESTAuthzAllow10Paths(b *testing.B) {
	runAuthzBenchmark(b, testAuthz.Allow, 10)
}

func BenchmarkRESTAuthzAllow100Paths(b *testing.B) {
	runAuthzBenchmark(b, testAuthz.Allow, 100)
}

// TODO: Re-enable when performance issues have been addressed.
// func BenchmarkRESTAuthzAllow1000Paths(b *testing.B) {
// 	runAuthzBenchmark(b, testAuthz.Allow, 1000)
// }

func runAuthzBenchmark(b *testing.B, mode testAuthz.InputMode, numPaths int) {
	// Generate test data and create a new bundle from it
	profile := testAuthz.DataSetProfile{
		NumTokens: 1000,
		NumPaths:  numPaths,
	}
	data := testAuthz.GenerateDataset(profile)

	err := testRuntime.UploadDataToPath("/restauthz", bytes.NewReader(util.MustMarshalJSON(data["restauthz"])))
	if err != nil {
		b.Fatal(err)
	}

	url := testRuntime.URL() + "/v1/" + queryPath

	input, expected := testAuthz.GenerateInput(profile, mode)
	inputPayload := util.MustMarshalJSON(map[string]interface{}{
		"input": input,
	})
	inputReader := bytes.NewReader(inputPayload)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		// The benchmark will include the time it takes to make the request,
		// receive a response, and do any normal client error checking on
		// the response. The benchmark is for the OPA server, not how
		// long it takes the golang client to unpack the response body.
		b.StartTimer()
		resp, err := testRuntime.GetDataWithRawInput(url, inputReader)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()

		body, err := io.ReadAll(resp)
		if err != nil {
			b.Fatalf("unexpected error reading response body: %s", err)
		}
		resp.Close()

		parsedBody := struct {
			Result bool `json:"result"`
		}{}

		err = json.Unmarshal(body, &parsedBody)
		if err != nil {
			b.Fatalf("Failed to parse body: \n\nActual: %s\n\nExpected: {\"result\": BOOL}\n\nerr = %s ", string(body), err)
		}
		if parsedBody.Result != expected {
			b.Fatalf("Unexpected result: %v", parsedBody.Result)
		}

		inputReader.Reset(inputPayload)
	}
}
