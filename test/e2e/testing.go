// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/open-policy-agent/opa/runtime"
	"github.com/open-policy-agent/opa/util"
)

const (
	defaultAddr = ":0" // default listening address for server, use a random open port
)

// NewAPIServerTestParams creates a new set of runtime.Params with enough
// default values filled in to start the server. Options can/should
// be customized for the test case.
func NewAPIServerTestParams() runtime.Params {
	params := runtime.NewParams()

	// Add in some defaults
	params.Addrs = &[]string{defaultAddr}

	params.Logging = runtime.LoggingConfig{
		Level:  "debug",
		Format: "json-pretty",
	}

	params.GracefulShutdownPeriod = 10 // seconds

	return params
}

// TestRuntime holds metadata and provides helper methods
// to interact with the runtime being tested.
type TestRuntime struct {
	Params  runtime.Params
	Runtime *runtime.Runtime
	Ctx     context.Context
	Cancel  context.CancelFunc
	Client  *http.Client
	url     string
	urlMtx  *sync.Mutex
}

// NewTestRuntime returns a new TestRuntime which
func NewTestRuntime(params runtime.Params) (*TestRuntime, error) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	rt, err := runtime.NewRuntime(ctx, params)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("unable to create new runtime: %s", err)
	}

	return &TestRuntime{
		Params:  params,
		Runtime: rt,
		Ctx:     ctx,
		Cancel:  cancel,
		Client:  &http.Client{},
		urlMtx:  new(sync.Mutex),
	}, nil
}

// RunAPIServerTests will start the OPA runtime serving with a given
// configuration. This is essentially a wrapper for `m.Run()` that
// handles starting and stopping the local API server. The return
// value is what should be used as the code in `os.Exit` in the
// `TestMain` function.
func (t *TestRuntime) RunAPIServerTests(m *testing.M) int {
	return t.runTests(m, false)
}

// RunAPIServerBenchmarks will start the OPA runtime and do
// `m.Run()` similar to how RunAPIServerTests works. This
// will suppress logging output on stdout to prevent the tests
// from being overly verbose. If log output is desired set
// the `test.v` flag.
func (t *TestRuntime) RunAPIServerBenchmarks(m *testing.M) int {
	return t.runTests(m, !testing.Verbose())
}

// URL will return the URL that the server is listening on. If
// the server hasn't started listening this will return an empty string.
// It is not expected for the URL to change throughout the lifetime of the
// TestRuntime. Runtimes configured with >1 address will only get the
// first URL.
func (t *TestRuntime) URL() string {
	if t.url != "" {
		// fast path once it has been computed
		return t.url
	}

	t.urlMtx.Lock()
	defer t.urlMtx.Unlock()

	// check again in the lock, it might have changed on us..
	if t.url != "" {
		return t.url
	}

	addrs := t.Runtime.Addrs()
	if len(addrs) == 0 {
		return ""
	}
	// Just pick the first one, if a test was configured with >1 they
	// will need to determine the URLs themselves.
	addr := addrs[0]

	if strings.HasPrefix(addr, ":") {
		addr = "localhost" + addr
	}

	if !strings.Contains(addr, "://") {
		scheme := "http://"
		if t.Params.Certificate != nil {
			scheme = "https://"
		}
		addr = scheme + addr
	}

	parsed, err := url.Parse(addr)
	if err != nil {
		fmt.Printf("Failed to parse listening address of server: %s", err)
		os.Exit(1)
	}

	t.url = parsed.String()

	return t.url
}

func (t *TestRuntime) runTests(m *testing.M, suppressLogs bool) int {
	// Start serving API requests in the background
	done := make(chan error)
	go func() {
		// Suppress the stdlogger in the server
		if suppressLogs {
			logrus.SetOutput(ioutil.Discard)
		}
		err := t.Runtime.Serve(t.Ctx)
		done <- err
	}()

	// Turns out this thread gets a different stdlogger
	// so we need to set the output on it here too.
	if suppressLogs {
		logrus.SetOutput(ioutil.Discard)
	}

	// wait for the server to be ready
	err := t.waitForServer()
	if err != nil {
		return 1
	}

	// Actually run the unit tests/benchmarks
	errc := m.Run()

	// Wait for the API server to stop
	t.Cancel()
	err = <-done

	if err != nil && errc == 0 {
		// even if the tests passed return an error code if
		// the server encountered an error
		errc = 1
	}

	return errc
}

func (t *TestRuntime) waitForServer() error {
	delay := time.Duration(100) * time.Millisecond
	retries := 100 // 10 seconds before we give up
	for i := 0; i < retries; i++ {
		// First make sure it has started listening and we have an address
		if t.URL() != "" {
			// Then make sure it has started serving
			resp, err := http.Get(t.URL() + "/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				logrus.Infof("Test server ready and listening on: %s", t.URL())
				return nil
			}
		}
		time.Sleep(delay)
	}
	return fmt.Errorf("API Server not ready in time")
}

// UploadPolicy will upload the given policy to the runtime via the v1 policy API
func (t *TestRuntime) UploadPolicy(name string, policy io.Reader) error {
	req, err := http.NewRequest("PUT", t.URL()+"/v1/policies/"+name, policy)
	if err != nil {
		return fmt.Errorf("Unexpected error creating request: %s", err)
	}
	resp, err := t.Client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to PUT the test policy: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected response: %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}

// UploadData will upload the given data to the runtime via the v1 data API
func (t *TestRuntime) UploadData(data io.Reader) error {
	client := &http.Client{}
	req, err := http.NewRequest("PUT", t.URL()+"/v1/data", data)
	if err != nil {
		return fmt.Errorf("Unexpected error creating request: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to PUT data: %s", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Unexpected response: %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}

// GetDataWithInput will use the v1 data API and POST with the given input. The returned
// value is the full response body.
func (t *TestRuntime) GetDataWithInput(path string, input interface{}) ([]byte, error) {
	inputPayload := util.MustMarshalJSON(map[string]interface{}{
		"input": input,
	})

	path = strings.TrimPrefix(path, "/")
	if !strings.HasPrefix(path, "data") {
		path = "data/" + path
	}

	resp, err := http.Post(t.URL()+"/v1/"+path, "application/json", bytes.NewReader(inputPayload))
	if err != nil {
		return nil, fmt.Errorf("Unexpected error: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected response status: %d %s", resp.StatusCode, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Unexpected error reading response body: %s", err)
	}

	return body, nil
}

// GetDataWithInputTyped returns an unmarshalled response from GetDataWithInput.
func (t *TestRuntime) GetDataWithInputTyped(path string, input interface{}, response interface{}) error {

	bs, err := t.GetDataWithInput(path, input)
	if err != nil {
		return err
	}

	return json.Unmarshal(bs, response)
}
