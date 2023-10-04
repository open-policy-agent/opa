package diagnostics

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime

func TestMain(m *testing.M) {
	flag.Parse()
	testServerParams := e2e.NewAPIServerTestParams()
	testServerParams.Addrs = &[]string{"localhost:0"}
	testServerParams.DiagnosticAddrs = &[]string{"localhost:0"}

	var err error
	testRuntime, err = e2e.NewTestRuntimeWithOpts(e2e.TestRuntimeOpts{}, testServerParams)
	if err != nil {
		os.Exit(1)
	}

	os.Exit(testRuntime.RunTests(m))
}

func TestServerWithDiagnosticAddrHealthCheck(t *testing.T) {
	if err := testRuntime.HealthCheck(diagURL(t)); err != nil {
		t.Fatal(err)
	}

	// Ensure the "main" listener is still OK
	if err := testRuntime.HealthCheck(testRuntime.URL()); err != nil {
		t.Fatal(err)
	}
}

func TestServerWithDiagnosticAddrProtectedAPIs(t *testing.T) {
	cases := []string{
		"/",
		"/v0/data",
		"/v0/data/foo",
		"/v1/data",
		"/v1/data/foo",
		"/v1/policies",
		"/v1/policies/foo",
		"/v1/query",
		"/v1/compile",
	}

	baseURL := diagURL(t)

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodTrace,
		http.MethodPatch,
		http.MethodConnect,
		http.MethodDelete,
		http.MethodOptions,
		http.MethodHead,
	}

	for _, tc := range cases {
		url := baseURL + tc
		for _, method := range methods {
			t.Run(fmt.Sprintf("%s %s", method, tc), func(t *testing.T) {
				assert404(t, method, url)
			})
		}
	}
}

func diagURL(t *testing.T) string {
	t.Helper()
	addr := testRuntime.Runtime.DiagnosticAddrs()[0]
	diagURL, err := testRuntime.AddrToURL(addr)
	if err != nil {
		t.Error("Unexpected error: ", err)
	}
	return diagURL
}

func assert404(t *testing.T, method string, url string) {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Errorf("Unexpected error creating request: %s", err)
	}
	resp, err := testRuntime.Client.Do(req)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("Unexpected response, expected 404, got: %d %s", resp.StatusCode, resp.Status)
	}
}
