// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package shutdown

import (
	"flag"
	"os"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime

func TestMain(m *testing.M) {
	flag.Parse()
	testServerParams := e2e.NewAPIServerTestParams()

	testServerParams.GracefulShutdownPeriod = 1
	testServerParams.ShutdownWaitPeriod = 2

	var err error
	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		os.Exit(1)
	}

	os.Exit(testRuntime.RunTests(m))
}

func TestShutdownWaitPeriod(t *testing.T) {
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}

	err = proc.Signal(os.Interrupt)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(1500 * time.Millisecond)

	// Ensure that OPA i still running
	err = testRuntime.HealthCheck(testRuntime.URL())
	if err != nil {
		t.Fatalf("Expected health endpoint to be up but got:\n\n%v", err)
	}
}
