// Copyright 2026 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/open-policy-agent/opa/v1/test/e2e"
)

// startOPA starts an OPA server for the duration of the test, and returns its URL.
func startOPA(t *testing.T) string {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())

	params := e2e.NewAPIServerTestParams()
	params.Addrs = &[]string{"0.0.0.0:0"}
	testRuntime, err := e2e.NewTestRuntime(params)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		if err := testRuntime.Runtime.Serve(ctx); err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	}()
	t.Cleanup(cancel)

	if err := testRuntime.WaitForServer(); err != nil {
		t.Fatal(err)
	}
	t.Log(testRuntime.URL())
	return testRuntime.URL()
}

// putPolicy uploads the policy, replacing any with the same file name.
func putPolicy(t *testing.T, opaURL, file, rego string) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, opaURL+"/v1/policies/"+file, strings.NewReader(rego))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("put policy: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("put policy: status %d", resp.StatusCode)
	}
}

// postCompile sends the payload to the Compile API for the given target (Accept
// header), and returns the status code and body.
func postCompile(t *testing.T, opaURL, path, accept string, payload any) (int, []byte) {
	t.Helper()
	bs, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, opaURL+"/v1/compile/"+path, bytes.NewReader(bs))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	return resp.StatusCode, body
}

// containerConfig holds the configuration of a container started by startContainer.
type containerConfig struct {
	image       string
	cmd         []string
	port        string
	env         map[string]string
	waitFor     wait.Strategy
	urlTemplate string
}

// startContainer starts the container, and returns it with the host and mapped
// port that reach config.port.
func startContainer(ctx context.Context, config containerConfig) (testcontainers.Container, string, string, error) {
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        config.image,
			Cmd:          config.cmd,
			ExposedPorts: []string{config.port},
			WaitingFor:   config.waitFor,
			Env:          config.env,
		},
		Started: true,
	})
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to start container: %v", err)
	}

	port, err := container.MappedPort(ctx, config.port)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get container port: %v", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get container host: %v", err)
	}
	return container, host, port.Port(), nil
}
