package test

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/logging"
	test_sdk "github.com/open-policy-agent/opa/sdk/test"
	"github.com/open-policy-agent/opa/test/e2e"
)

type SafeBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *SafeBuffer) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}
func (b *SafeBuffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}
func (b *SafeBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}

func TestEnablePrintStatementsForBundles(t *testing.T) {
	ref := "registry.io/someorg/somerepo:tag"
	server := test_sdk.MustNewServer(test_sdk.MockOCIBundle(ref, map[string]string{
		"post.rego": `
		package peoplefinder.POST.api.users

		import input.user.properties as user_props
		
		default allowed = false
		
		allowed {
			user_props.department == "Operations"
			user_props.title == "IT Manager"
		}	
		`,
	}))
	params := e2e.NewAPIServerTestParams()

	buf := SafeBuffer{}

	logger := logging.New()
	logger.SetLevel(logging.Debug) // set to debug to see the the bundle download skip message
	logger.SetOutput(&buf)
	params.Logger = logger

	params.ConfigOverrides = []string{
		"services.test.url=" + server.URL(),
		"services.test.type=oci",
		"bundles.test.resource=" + ref,
		"bundles.test.polling.min_delay_seconds=1",
		"bundles.test.polling.max_delay_seconds=3",
	}

	// Test runtime uses the local OCI image layers stored in download testdata that contain the
	// rego policies based on the https://github.com/aserto-dev/policy-peoplefinder-abac template
	e2e.WithRuntime(t, e2e.TestRuntimeOpts{WaitForBundles: true}, params, func(rt *e2e.TestRuntime) {
		var readBuf []byte
		type Props struct {
			Department string `json:"department"`
			Title      string `json:"title"`
		}
		type Attributes struct {
			Properties Props `json:"properties"`
		}
		type Input struct {
			User Attributes `json:"user"`
		}

		inputAllowed := Input{User: Attributes{Properties: Props{Department: "Operations", Title: "IT Manager"}}}

		inputNotAllowed := Input{User: Attributes{Properties: Props{Department: "IT", Title: "Engineer"}}}

		readBuf, err := rt.GetDataWithInput("peoplefinder/POST/api/users/allowed", inputAllowed)
		if err != nil {
			t.Fatal("failed to get data from runtime")
		}

		response := string(readBuf)
		if !strings.Contains(response, "true") {
			t.Fatalf("expected true but got: %s", response)
		}
		readBuf, err = rt.GetDataWithInput("peoplefinder/POST/api/users/allowed", inputNotAllowed)
		if err != nil {
			t.Fatal("failed to get data from runtime")
		}
		if !strings.Contains(string(readBuf), "false") {
			t.Fatalf("expected true but got: %s", response)
		}

		time.Sleep(3 * time.Second) // wait for the downloader pooling mechanism to kick in
		expContains := "Bundle loaded and activated successfully"
		skipContains := "Bundle load skipped, server replied with not modified."

		if !strings.Contains(buf.String(), expContains) {
			t.Fatalf("expected logs to contain %q but got: %v", expContains, buf.String())
		}
		time.Sleep(3 * time.Second) // wait a couple of seconds for the second trigger to kick in
		if !strings.Contains(buf.String(), skipContains) {
			t.Fatalf("expected logs to contain %q but got: %v", skipContains, buf.String())
		}
	})
}
