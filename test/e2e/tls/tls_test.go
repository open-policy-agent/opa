package tls

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime
var pool *x509.CertPool

// print error to stderr, exit 1
func fatal(err interface{}) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}

func TestMain(m *testing.M) {
	flag.Parse()

	caCertPEM, err := ioutil.ReadFile("testdata/ca.pem")
	if err != nil {
		fatal(err)
	}
	pool = x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCertPEM); !ok {
		fatal("failed to parse CA cert")
	}
	cert, err := tls.LoadX509KeyPair("testdata/server-cert.pem", "testdata/server-key.pem")
	if err != nil {
		fatal(err)
	}

	// We need the policy to be present already, otherwise authorization
	// for the health endpoint is going to fail on server startup.
	authzPolicy := []byte(`package system.authz
import input.identity
default allow = false
allow {
	identity = "CN=my-client"
}`)

	tmpfile, err := ioutil.TempFile("", "authz.*.rego")
	if err != nil {
		fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write(authzPolicy); err != nil {
		fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		fatal(err)
	}

	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.Addrs = &[]string{"https://127.0.0.1:0"}
	testServerParams.CertPool = pool
	testServerParams.Certificate = &cert
	testServerParams.Authentication = server.AuthenticationTLS
	testServerParams.Authorization = server.AuthorizationBasic
	testServerParams.Paths = []string{"system.authz:" + tmpfile.Name()}

	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		fatal(err)
	}

	// We need a client with proper TLS setup, otherwise the health check
	// that loops to determine if the server is ready will fail.
	testRuntime.Client = newClient(pool, "testdata/client-cert.pem", "testdata/client-key.pem")

	os.Exit(testRuntime.RunTests(m))
}

func TestAuthenticationTLS(t *testing.T) {
	endpoint := testRuntime.URL() + "/v1/data/foo"

	// Note: This test is redundant. When the testRuntime starts the server, it
	// already queries the health endpoint using a properly authenticated, and
	// authorized, http client.
	t.Run("happy path", func(t *testing.T) {
		c := newClient(pool, "testdata/client-cert.pem", "testdata/client-key.pem")
		resp, err := c.Get(endpoint)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %s", resp.Status)
		}
	})

	t.Run("authn successful, authz failed", func(t *testing.T) {
		c := newClient(pool, "testdata/client-cert-2.pem", "testdata/client-key-2.pem")
		resp, err := c.Get(endpoint)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %s", resp.Status)
		}
	})

	t.Run("client trusts server, but doesn't provide client cert", func(t *testing.T) {
		c := newClient(pool)
		_, err := c.Get(endpoint)
		if _, ok := err.(*url.Error); !ok {
			t.Errorf("expected *url.Error, got %T: %v", err, err)
		}
	})
}

func newClient(pool *x509.CertPool, clientKeyPair ...string) *http.Client {
	c := *http.DefaultClient
	// Note: zero-values in http.Transport are bad settings -- they let the client
	// leak connections -- but it's good enough for these tests. Don't instantiate
	// http.Transport without providing non-zero values in non-test code, please.
	// See https://github.com/golang/go/issues/19620 for details.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
	if len(clientKeyPair) == 2 {
		clientCert, err := tls.LoadX509KeyPair(clientKeyPair[0], clientKeyPair[1])
		if err != nil {
			panic(err)
		}
		tr.TLSClientConfig.Certificates = []tls.Certificate{clientCert}
	}
	c.Transport = tr
	return &c
}
