// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package authz

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/test/authz"
	testAuthz "github.com/open-policy-agent/opa/test/authz"
	"github.com/open-policy-agent/opa/test/e2e"
	"github.com/open-policy-agent/opa/util"
)

var testRuntime *e2e.TestRuntime

func TestMain(m *testing.M) {
	flag.Parse()

	testServerParams := e2e.NewAPIServerTestParams()
	disk, cleanup := diskStorage()
	testServerParams.DiskStorage = disk
	var err error
	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		os.Exit(1)
	}

	errc := testRuntime.RunTests(m)
	if cleanup != nil {
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

func BenchmarkRESTAuthzAllow1000Paths(b *testing.B) {
	runAuthzBenchmark(b, testAuthz.Allow, 1000)
}

func runAuthzBenchmark(b *testing.B, mode testAuthz.InputMode, numPaths int) {
	// Generate test data and push it into the server
	profile := testAuthz.DataSetProfile{
		NumTokens: 1000,
		NumPaths:  numPaths,
	}
	data := testAuthz.GenerateDataset(profile)
	err := testRuntime.UploadData(bytes.NewReader(util.MustMarshalJSON(data)))
	if err != nil {
		b.Fatal(err)
	}

	// Push the test policy
	err = testRuntime.UploadPolicy("restauthz", strings.NewReader(testAuthz.Policy))
	if err != nil {
		b.Fatal(err)
	}

	queryPath := strings.Replace(authz.AllowQuery, ".", "/", -1)
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
