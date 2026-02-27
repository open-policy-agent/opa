package h2c_test

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"

	"golang.org/x/net/http2"

	"github.com/open-policy-agent/opa/v1/runtime"
	"github.com/open-policy-agent/opa/v1/test/e2e"
)

var (
	testRuntime       *e2e.TestRuntime
	testSocketPathH2C string
)

func TestMain(m *testing.M) {
	flag.Parse()

	testSocketPathH2C = fmt.Sprintf("/tmp/opa-h2c-test-%d.sock", os.Getpid())
	defer os.Remove(testSocketPathH2C)

	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.Addrs = &[]string{"localhost:0", "unix://" + testSocketPathH2C}
	testServerParams.DiagnosticAddrs = &[]string{"localhost:0"}
	testServerParams.H2CEnabled = true

	var err error
	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		os.Exit(1)
	}

	os.Exit(testRuntime.RunTests(m))
}

func TestH2CHTTPListeners(t *testing.T) {
	client := http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, _ *tls.Config) (net.Conn, error) {
				return net.Dial(network, addr)
			},
		},
	}

	addrs := append(testRuntime.Runtime.Addrs(), testRuntime.Runtime.DiagnosticAddrs()...)

	if expected, actual := 2, len(addrs); expected != actual {
		t.Fatalf("expected %d addresses, found %d", expected, actual)
	}

	for _, addr := range addrs {
		u := "http://" + addr + "/health"

		resp, err := client.Get(u)
		if err != nil {
			t.Fatalf("failed to GET %s: %s", u, err)
		}

		if expected, actual := http.StatusOK, resp.StatusCode; expected != actual {
			t.Errorf("resp status: expected %d, got %d", expected, actual)
		}
		if expected, actual := 2, resp.ProtoMajor; expected != actual {
			t.Errorf("resp.ProtoMajor: expected %d, got %d", expected, actual)
		}

		resp.Body.Close()
	}
}

func TestH2CUnixDomainSocket(t *testing.T) {
	t.Run("HTTP2Client", func(t *testing.T) {
		client := http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLS: func(network, addr string, _ *tls.Config) (net.Conn, error) {
					return net.Dial("unix", testSocketPathH2C)
				},
			},
		}

		resp, err := client.Get("http://localhost/health")
		if err != nil {
			t.Fatalf("failed to GET /health: %s", err)
		}
		defer resp.Body.Close()

		if expected, actual := http.StatusOK, resp.StatusCode; expected != actual {
			t.Errorf("resp status: expected %d, got %d", expected, actual)
		}
		if expected, actual := 2, resp.ProtoMajor; expected != actual {
			t.Errorf("resp.ProtoMajor: expected %d, got %d", expected, actual)
		}
	})

	t.Run("HTTP1Client", func(t *testing.T) {
		client := http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", testSocketPathH2C)
				},
			},
		}

		resp, err := client.Get("http://localhost/health")
		if err != nil {
			t.Fatalf("failed to GET /health: %s", err)
		}
		defer resp.Body.Close()

		if expected, actual := http.StatusOK, resp.StatusCode; expected != actual {
			t.Errorf("resp status: expected %d, got %d", expected, actual)
		}
		if expected, actual := 1, resp.ProtoMajor; expected != actual {
			t.Errorf("resp.ProtoMajor: expected %d, got %d", expected, actual)
		}
	})
}

// TestH2CMaxConcurrentStreams verifies that the server starts correctly and
// serves HTTP/2 requests when H2CMaxConcurrentStreams is configured. It does
// not verify that the stream limit is enforced; that would require negotiating
// concurrent streams and inspecting the HTTP/2 SETTINGS frame.
func TestH2CMaxConcurrentStreams(t *testing.T) {
	socketPath := fmt.Sprintf("/tmp/opa-h2c-max-streams-test-%d.sock", os.Getpid())
	socketAddr := "unix://" + socketPath
	t.Cleanup(func() {
		_ = os.Remove(socketPath)
	})

	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.Addrs = &[]string{"localhost:0", socketAddr}
	testServerParams.H2CEnabled = true
	testServerParams.H2CMaxConcurrentStreams = 512

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	rt, err := e2e.NewTestRuntimeWithOpts(e2e.TestRuntimeOpts{}, testServerParams)
	if err != nil {
		t.Fatalf("failed to create test runtime: %s", err)
	}

	go func() {
		if err := rt.Runtime.Serve(ctx); err != nil {
			t.Logf("server stopped: %s", err)
		}
	}()

	if err := rt.WaitForServerStatus(runtime.ServerInitialized); err != nil {
		t.Fatalf("server failed to start: %s", err)
	}

	t.Run("TCPListener", func(t *testing.T) {
		addrs := rt.Runtime.Addrs()
		if len(addrs) == 0 {
			t.Fatal("no TCP addresses available")
		}

		client := http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLS: func(network, addr string, _ *tls.Config) (net.Conn, error) {
					return net.Dial(network, addr)
				},
			},
		}

		u := "http://" + addrs[0] + "/health"
		resp, err := client.Get(u)
		if err != nil {
			t.Fatalf("failed to GET %s: %s", u, err)
		}
		defer resp.Body.Close()

		if expected, actual := http.StatusOK, resp.StatusCode; expected != actual {
			t.Errorf("resp status: expected %d, got %d", expected, actual)
		}
		if expected, actual := 2, resp.ProtoMajor; expected != actual {
			t.Errorf("resp.ProtoMajor: expected %d, got %d", expected, actual)
		}
	})

	t.Run("UnixSocketListener", func(t *testing.T) {
		client := http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLS: func(network, addr string, _ *tls.Config) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		}

		resp, err := client.Get("http://localhost/health")
		if err != nil {
			t.Fatalf("failed to GET /health: %s", err)
		}
		defer resp.Body.Close()

		if expected, actual := http.StatusOK, resp.StatusCode; expected != actual {
			t.Errorf("resp status: expected %d, got %d", expected, actual)
		}
		if expected, actual := 2, resp.ProtoMajor; expected != actual {
			t.Errorf("resp.ProtoMajor: expected %d, got %d", expected, actual)
		}
	})
}

func TestH2CDisabledUnixDomainSocket(t *testing.T) {
	socketPath := fmt.Sprintf("/tmp/opa-no-h2c-test-%d.sock", os.Getpid())
	socketAddr := "unix://" + socketPath
	t.Cleanup(func() {
		_ = os.Remove(socketPath)
	})

	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.Addrs = &[]string{socketAddr}
	testServerParams.H2CEnabled = false

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	rt, err := e2e.NewTestRuntimeWithOpts(e2e.TestRuntimeOpts{}, testServerParams)
	if err != nil {
		t.Fatalf("failed to create test runtime: %s", err)
	}

	go func() {
		if err := rt.Runtime.Serve(ctx); err != nil {
			t.Logf("server stopped: %s", err)
		}
	}()

	if err := rt.WaitForServerStatus(runtime.ServerInitialized); err != nil {
		t.Fatalf("server failed to start: %s", err)
	}

	t.Run("HTTP1Client", func(t *testing.T) {
		client := http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		}

		resp, err := client.Get("http://localhost/health")
		if err != nil {
			t.Fatalf("failed to GET /health: %s", err)
		}
		defer resp.Body.Close()

		if expected, actual := http.StatusOK, resp.StatusCode; expected != actual {
			t.Errorf("resp status: expected %d, got %d", expected, actual)
		}
		if expected, actual := 1, resp.ProtoMajor; expected != actual {
			t.Errorf("resp.ProtoMajor: expected %d, got %d", expected, actual)
		}
	})

	t.Run("HTTP2ClientShouldFail", func(t *testing.T) {
		client := http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLS: func(network, addr string, _ *tls.Config) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		}

		resp, err := client.Get("http://localhost/health")
		if err != nil {
			t.Logf("Expected failure for HTTP/2 client when h2c disabled: %s", err)
			return
		}
		defer resp.Body.Close()

		if resp.ProtoMajor == 2 {
			t.Errorf("HTTP/2 should not be available when h2c is disabled")
		}
	})
}
