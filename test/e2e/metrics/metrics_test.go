package metrics

import (
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/test/e2e"
)

var testRuntime *e2e.TestRuntime

func TestMain(m *testing.M) {
	flag.Parse()
	testServerParams := e2e.NewAPIServerTestParams()

	var err error
	testRuntime, err = e2e.NewTestRuntime(testServerParams)
	if err != nil {
		os.Exit(1)
	}

	os.Exit(testRuntime.RunTests(m))
}

func TestMetricsEndpoint(t *testing.T) {

	policy := `
	package test
	p = true
	`

	err := testRuntime.UploadPolicy(t.Name(), strings.NewReader(policy))
	if err != nil {
		t.Fatal(err)
	}

	dr := struct {
		Result bool `json:"result"`
	}{}

	if err := testRuntime.GetDataWithInputTyped("test/p", nil, &dr); err != nil {
		t.Fatal(err)
	}

	if !dr.Result {
		t.Fatalf("Unexpected response: %+v", dr)
	}

	mr, err := http.Get(testRuntime.URL() + "/metrics")
	if err != nil {
		t.Fatal(err)
	}

	defer mr.Body.Close()

	bs, err := ioutil.ReadAll(mr.Body)
	if err != nil {
		t.Fatal(err)
	}

	str := string(bs)

	expected := []string{
		`http_request_duration_seconds_count{code="200",handler="v1/policies",method="put"} 1`,
		`http_request_duration_seconds_count{code="200",handler="v1/data",method="post"} 1`,
	}

	for _, exp := range expected {
		if !strings.Contains(str, exp) {
			t.Fatalf("Expected to find %q but got:\n\n%v", exp, str)
		}
	}
}
