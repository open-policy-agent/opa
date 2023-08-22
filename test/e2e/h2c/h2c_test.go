package h2c_test

import (
	"crypto/tls"
	"flag"
	"net"
	"net/http"
	"os"
	"testing"

	"golang.org/x/net/http2"

	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime

func TestMain(m *testing.M) {
	flag.Parse()
	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.Addrs = &[]string{"localhost:0"}
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
	// h2c-enabled client
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
		defer resp.Body.Close()

		if expected, actual := http.StatusOK, resp.StatusCode; expected != actual {
			t.Errorf("resp status: expected %d, got %d", expected, actual)
		}
		if expected, actual := 2, resp.ProtoMajor; expected != actual {
			t.Errorf("resp.ProtoMajor: expected %d, got %d", expected, actual)
		}
	}
}
