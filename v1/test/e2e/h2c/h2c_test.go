package h2c_test

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

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

// TestH2CMaxConcurrentStreams verifies that the server advertises the configured
// H2CMaxConcurrentStreams value in its HTTP/2 SETTINGS frame by performing the
// HTTP/2 connection preface exchange and inspecting SETTINGS_MAX_CONCURRENT_STREAMS.
func TestH2CMaxConcurrentStreams(t *testing.T) {
	socketPath := fmt.Sprintf("/tmp/opa-h2c-max-streams-test-%d.sock", os.Getpid())
	socketAddr := "unix://" + socketPath
	t.Cleanup(func() {
		_ = os.Remove(socketPath)
	})

	const configuredLimit = uint32(512)

	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.Addrs = &[]string{"localhost:0", socketAddr}
	testServerParams.H2CEnabled = true
	testServerParams.H2CMaxConcurrentStreams = configuredLimit

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

	// readMaxConcurrentStreams performs the HTTP/2 "prior knowledge" connection
	// preface exchange over conn and returns the MAX_CONCURRENT_STREAMS value
	// from the server's initial SETTINGS frame.
	readMaxConcurrentStreams := func(t *testing.T, conn net.Conn) uint32 {
		t.Helper()
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second)) //nolint:errcheck

		// Send the HTTP/2 client connection preface followed by an empty
		// SETTINGS frame, as required by RFC 9113 §3.4.
		if _, err := io.WriteString(conn, http2.ClientPreface); err != nil {
			t.Fatalf("write client preface: %s", err)
		}
		framer := http2.NewFramer(conn, conn)
		if err := framer.WriteSettings(); err != nil {
			t.Fatalf("write client SETTINGS: %s", err)
		}

		// Read frames until we receive the server's initial SETTINGS frame
		// (not an ACK). Per RFC 9113 §3.4 this is always the first frame
		// the server sends.
		for {
			frame, err := framer.ReadFrame()
			if err != nil {
				t.Fatalf("read frame: %s", err)
			}
			sf, ok := frame.(*http2.SettingsFrame)
			if !ok || sf.IsAck() {
				continue
			}
			val, _ := sf.Value(http2.SettingMaxConcurrentStreams)
			return val
		}
	}

	t.Run("TCPListener", func(t *testing.T) {
		addrs := rt.Runtime.Addrs()
		if len(addrs) == 0 {
			t.Fatal("no TCP addresses available")
		}
		conn, err := net.Dial("tcp", addrs[0])
		if err != nil {
			t.Fatalf("dial TCP: %s", err)
		}
		if got := readMaxConcurrentStreams(t, conn); got != configuredLimit {
			t.Errorf("SETTINGS_MAX_CONCURRENT_STREAMS: got %d, want %d", got, configuredLimit)
		}
	})

	t.Run("UnixSocketListener", func(t *testing.T) {
		conn, err := net.Dial("unix", socketPath)
		if err != nil {
			t.Fatalf("dial unix: %s", err)
		}
		if got := readMaxConcurrentStreams(t, conn); got != configuredLimit {
			t.Errorf("SETTINGS_MAX_CONCURRENT_STREAMS: got %d, want %d", got, configuredLimit)
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
