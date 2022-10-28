// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/internal/uuid"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/logging/test"
	"github.com/open-policy-agent/opa/runtime"
	"github.com/open-policy-agent/opa/server/types"
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

	// unless overridden, don't log from tests
	params.Logger = logging.NewNoOpLogger()

	params.GracefulShutdownPeriod = 10 // seconds

	params.DecisionIDFactory = func() string {
		id, err := uuid.New(rand.Reader)
		if err != nil {
			return ""
		}
		return id
	}
	return params
}

// TestRuntime holds metadata and provides helper methods
// to interact with the runtime being tested.
type TestRuntime struct {
	Params         runtime.Params
	Runtime        *runtime.Runtime
	Ctx            context.Context
	Cancel         context.CancelFunc
	Client         *http.Client
	ConsoleLogger  *test.Logger
	url            string
	urlMtx         *sync.Mutex
	waitForBundles bool
}

// NewTestRuntime returns a new TestRuntime.
func NewTestRuntime(params runtime.Params) (*TestRuntime, error) {
	return NewTestRuntimeWithOpts(TestRuntimeOpts{}, params)
}

// NewTestRuntimeWithOpts returns a new TestRuntime.
func NewTestRuntimeWithOpts(opts TestRuntimeOpts, params runtime.Params) (*TestRuntime, error) {

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	rt, err := runtime.NewRuntime(ctx, params)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create new runtime: %w", err)
	}

	return &TestRuntime{
		Params:         params,
		Runtime:        rt,
		Ctx:            ctx,
		Cancel:         cancel,
		Client:         &http.Client{},
		urlMtx:         new(sync.Mutex),
		waitForBundles: opts.WaitForBundles,
	}, nil
}

// WrapRuntime creates a new TestRuntime by wrapping an existing runtime
func WrapRuntime(ctx context.Context, cancel context.CancelFunc, rt *runtime.Runtime) *TestRuntime {
	return &TestRuntime{
		Params:  rt.Params,
		Runtime: rt,
		Ctx:     ctx,
		Cancel:  cancel,
		Client:  &http.Client{},
		urlMtx:  new(sync.Mutex),
	}
}

// RunAPIServerTests will start the OPA runtime serving with a given
// configuration. This is essentially a wrapper for `m.Run()` that
// handles starting and stopping the local API server. The return
// value is what should be used as the code in `os.Exit` in the
// `TestMain` function.
// Deprecated: Use RunTests instead
func (t *TestRuntime) RunAPIServerTests(m *testing.M) int {
	return t.runTests(m, true)
}

// RunAPIServerBenchmarks will start the OPA runtime and do
// `m.Run()` similar to how RunAPIServerTests works. This
// will suppress logging output on stdout to prevent the tests
// from being overly verbose. If log output is desired set
// the `test.v` flag.
// Deprecated: Use RunTests instead
func (t *TestRuntime) RunAPIServerBenchmarks(m *testing.M) int {
	return t.runTests(m, !testing.Verbose())
}

// RunTests will start the OPA runtime serving with a given
// configuration. This is essentially a wrapper for `m.Run()` that
// handles starting and stopping the local API server. The return
// value is what should be used as the code in `os.Exit` in the
// `TestMain` function.
func (t *TestRuntime) RunTests(m *testing.M) int {
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

	parsed, err := t.AddrToURL(addr)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	t.url = parsed

	return t.url
}

// AddrToURL generates a full URL from an address, as configured on the runtime.
// This can include fully qualified urls, just host/ip, with port, or only port
// (eg, "localhost", ":8181", "http://foo", etc). If the runtime is configured
// with HTTPS certs it will generate an appropriate URL.
func (t *TestRuntime) AddrToURL(addr string) (string, error) {
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
		return "", fmt.Errorf("failed to parse listening address of server: %s", err)
	}

	return parsed.String(), nil
}

func (t *TestRuntime) runTests(m *testing.M, suppressLogs bool) int {
	// Start serving API requests in the background
	done := make(chan error)
	go func() {
		// Suppress the stdlogger in the server
		if suppressLogs {
			logging.Get().SetOutput(io.Discard)
		}
		err := t.Runtime.Serve(t.Ctx)
		done <- err
	}()

	// Turns out this thread gets a different stdlogger
	// so we need to set the output on it here too.
	if suppressLogs {
		logging.Get().SetOutput(io.Discard)
	}

	// wait for the server to be ready
	err := t.WaitForServer()
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

// TestRuntimeOpts contains parameters for the test runtime.
type TestRuntimeOpts struct {
	WaitForBundles bool // indicates if readiness check should depend on bundle activation
}

// WithRuntime invokes f with a new TestRuntime after waiting for server
// readiness. This function can be called inside of each test that requires a
// runtime as opposed to RunTests which can only be called once.
func WithRuntime(t *testing.T, opts TestRuntimeOpts, params runtime.Params, f func(rt *TestRuntime)) {

	t.Helper()

	rt, err := NewTestRuntimeWithOpts(opts, params)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error)
	go func() {
		err := rt.Runtime.Serve(rt.Ctx)
		done <- err
	}()

	err = rt.WaitForServer()
	if err != nil {
		t.Fatal(err)
	}

	f(rt)
	rt.Cancel()
	err = <-done

	if err != nil {
		t.Fatal(err)
	}
}

// WaitForServer will block until the server is running and passes a health check.
func (t *TestRuntime) WaitForServer() error {
	delay := time.Duration(100) * time.Millisecond
	retries := 100 // 10 seconds before we give up
	for i := 0; i < retries; i++ {
		// First make sure it has started listening and we have an address
		if t.URL() != "" {
			// Then make sure it has started serving
			err := t.HealthCheck(t.URL())
			if err == nil {
				logging.Get().Info("Test server ready and listening on: %s", t.URL())
				return nil
			}
		}
		time.Sleep(delay)
	}
	return fmt.Errorf("API Server not ready in time")
}

// DeletePolicy will delete the given policy in the runtime via the v1 policy API
func (t *TestRuntime) DeletePolicy(name string) error {
	req, err := http.NewRequest("DELETE", t.URL()+"/v1/policies/"+name, nil)
	if err != nil {
		return err
	}
	resp, err := t.Client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to DELETE the test policy: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response: %d %s", resp.StatusCode, resp.Status)
	}
	return nil
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
	return t.UploadDataToPath("/", data)
}

// UploadDataToPath will upload the given data to the runtime via the v1 data API
func (t *TestRuntime) UploadDataToPath(path string, data io.Reader) error {
	client := &http.Client{}

	urlPath := strings.TrimSuffix(filepath.Join("/v1/data"+path), "/")

	req, err := http.NewRequest("PUT", t.URL()+urlPath, data)
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

	resp, err := t.GetDataWithRawInput(t.URL()+"/v1/"+path, bytes.NewReader(inputPayload))
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp)
	if err != nil {
		return nil, fmt.Errorf("unexpected error reading response body: %s", err)
	}
	resp.Close()
	return body, nil
}

// GetDataWithRawInput will use the v1 data API and POST with the given input. The returned
// value is the full response body.
func (t *TestRuntime) GetDataWithRawInput(url string, input io.Reader) (io.ReadCloser, error) {
	return t.request("POST", url, input)
}

// GetData will use the v1 data API and GET without input. The returned value is the full
// response body.
func (t *TestRuntime) GetData(url string) (io.ReadCloser, error) {
	return t.request("GET", url, nil)
}

// CompileRequestWithInstrumentation will use the v1 compile API and POST with the given request and instrumentation enabled.
func (t *TestRuntime) CompileRequestWithInstrumentation(req types.CompileRequestV1) (*types.CompileResponseV1, error) {
	return t.compileRequest(req, true)
}

// CompileRequest will use the v1 compile API and POST with the given request.
func (t *TestRuntime) CompileRequest(req types.CompileRequestV1) (*types.CompileResponseV1, error) {
	return t.compileRequest(req, false)
}

func (t *TestRuntime) compileRequest(req types.CompileRequestV1, instrument bool) (*types.CompileResponseV1, error) {
	inputPayload := util.MustMarshalJSON(req)

	url := t.URL() + "/v1/compile"
	if instrument {
		url += "?instrument"
	}
	resp, err := t.request("POST", url, bytes.NewReader(inputPayload))
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(resp)
	if err != nil {
		return nil, fmt.Errorf("unexpected error reading response body: %s", err)
	}
	resp.Close()

	var typedResp types.CompileResponseV1
	err = json.Unmarshal(body, &typedResp)
	if err != nil {
		return nil, err
	}

	return &typedResp, nil
}

func (t *TestRuntime) request(method, url string, input io.Reader) (io.ReadCloser, error) {
	req, err := http.NewRequest(method, url, input)
	if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}
	req.Header.Set("content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unexpected error: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected response status: %s", resp.Status)
	}
	return resp.Body, nil
}

// GetDataWithInputTyped returns an unmarshalled response from GetDataWithInput.
func (t *TestRuntime) GetDataWithInputTyped(path string, input interface{}, response interface{}) error {

	bs, err := t.GetDataWithInput(path, input)
	if err != nil {
		return err
	}

	return json.Unmarshal(bs, response)
}

// HealthCheck will query /health and return an error if the server is not healthy
func (t *TestRuntime) HealthCheck(url string) error {

	url += "/health"
	if t.waitForBundles {
		url += "?bundles"
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("unexpected error creating request: %s", err)
	}
	resp, err := t.Client.Do(req)
	if err != nil {
		return fmt.Errorf("unexpected error: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response: %d %s", resp.StatusCode, resp.Status)
	}
	return nil
}
