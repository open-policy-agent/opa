package tls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/server"
	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime
var pool *x509.CertPool

var minTLSVersions = map[string]uint16{
	"1.0": tls.VersionTLS10,
	"1.1": tls.VersionTLS11,
	"1.2": tls.VersionTLS12,
	"1.3": tls.VersionTLS13,
}

// print error to stderr, exit 1
func fatal(err interface{}) {
	fmt.Fprintf(os.Stderr, "%s\n", err)
	os.Exit(1)
}

func TestMain(m *testing.M) {
	minTLSVersion := flag.String("min-tls-version", "1.2", "minimum TLS Version")
	TLSVersion := minTLSVersions[*minTLSVersion]
	flag.Parse()

	caCertPEM, err := os.ReadFile("testdata/ca.pem")
	if err != nil {
		fatal(err)
	}
	pool = x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caCertPEM); !ok {
		fatal("failed to parse CA cert")
	}
	certFile := "testdata/server-cert.pem"
	certKeyFile := "testdata/server-key.pem"
	cert, err := tls.LoadX509KeyPair(certFile, certKeyFile)
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

	tmpfile, err := os.CreateTemp("", "authz.*.rego")
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
	testServerParams.CertificateFile = certFile
	testServerParams.CertificateKeyFile = certKeyFile
	testServerParams.CertificateRefresh = time.Millisecond
	testServerParams.Authentication = server.AuthenticationTLS
	testServerParams.Authorization = server.AuthorizationBasic
	testServerParams.Paths = []string{"system.authz:" + tmpfile.Name()}
	if TLSVersion != 0 {
		testServerParams.MinTLSVersion = TLSVersion
	}

	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		fatal(err)
	}

	// We need a client with proper TLS setup, otherwise the health check
	// that loops to determine if the server is ready will fail.
	testRuntime.Client = newClient(0, pool, "testdata/client-cert.pem", "testdata/client-key.pem")

	os.Exit(testRuntime.RunTests(m))
}

func TestMinTLSVersion(t *testing.T) {
	endpoint := testRuntime.URL()
	t.Run("TLS version not suported by server", func(t *testing.T) {

		c := newClient(tls.VersionTLS10, pool, "testdata/client-cert.pem", "testdata/client-key.pem")
		_, err := c.Get(endpoint)

		if err == nil {
			t.Error("expected err - protocol version not supported, got nil")
		}

	})
	t.Run("TLS Version supported by server", func(t *testing.T) {

		c := newClient(tls.VersionTLS12, pool, "testdata/client-cert.pem", "testdata/client-key.pem")
		resp, err := c.Get(endpoint)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %s", resp.Status)
		}
	})
}

func TestNotDefaultTLSVersion(t *testing.T) {

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"cmd", "--min-tls-version", "1.3"}
	endpoint := testRuntime.URL()
	t.Run("server started with min TLS Version 1.3, client connecting with not supported TLS version", func(t *testing.T) {

		c := newClient(tls.VersionTLS10, pool, "testdata/client-cert.pem", "testdata/client-key.pem")
		_, err := c.Get(endpoint)

		if err == nil {
			t.Error("expected err - protocol version not supported, got nil")
		}
		var exp *url.Error
		if !errors.As(err, &exp) {
			t.Errorf("expected err type %[1]T, got %[2]T: %[2]v", exp, err)
		}
	})

	t.Run("server started with min TLS Version 1.3, client connecting supported TLS version", func(t *testing.T) {

		c := newClient(tls.VersionTLS13, pool, "testdata/client-cert.pem", "testdata/client-key.pem")
		resp, err := c.Get(endpoint)
		if err != nil {
			t.Fatalf("GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %s", resp.Status)
		}
	})

}

func TestAuthenticationTLS(t *testing.T) {
	endpoint := testRuntime.URL() + "/v1/data/foo"

	// Note: This test is redundant. When the testRuntime starts the server, it
	// already queries the health endpoint using a properly authenticated, and
	// authorized, http client.
	t.Run("happy path", func(t *testing.T) {
		c := newClient(0, pool, "testdata/client-cert.pem", "testdata/client-key.pem")
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
		c := newClient(0, pool, "testdata/client-cert-2.pem", "testdata/client-key-2.pem")
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
		c := newClient(0, pool)
		_, err := c.Get(endpoint)
		if _, ok := err.(*url.Error); !ok {
			t.Errorf("expected *url.Error, got %T: %v", err, err)
		}
	})
}

func newClient(maxTLSVersion uint16, pool *x509.CertPool, clientKeyPair ...string) *http.Client {
	c := *http.DefaultClient
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		RootCAs: pool,
	}

	if len(clientKeyPair) == 2 {
		clientCert, err := tls.LoadX509KeyPair(clientKeyPair[0], clientKeyPair[1])
		if err != nil {
			panic(err)
		}
		tr.TLSClientConfig.Certificates = []tls.Certificate{clientCert}
	}
	if maxTLSVersion != 0 {
		tr.TLSClientConfig.MaxVersion = maxTLSVersion
	}
	c.Transport = tr
	return &c
}
