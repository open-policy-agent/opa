package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/test/e2e"
)

// RunDecisionLoggerBenchmark runs a benchmark for decision logs with a
// pre-configured test runtime
func RunDecisionLoggerBenchmark(b *testing.B, rt *e2e.TestRuntime) {
	ruleCounts := []int{1, 10, 100, 1000}
	rulesHitCounts := []int{0, 1, 10, 100, 1000}

	for _, hitCount := range rulesHitCounts {
		for _, ruleCount := range ruleCounts {
			if hitCount > ruleCount {
				continue
			}
			name := fmt.Sprintf("%dx%d", ruleCount, hitCount)
			policy := GeneratePolicy(ruleCount, hitCount)

			// Push the test policy
			err := rt.UploadPolicy("test", strings.NewReader(policy))
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()

			b.Run(name, func(b *testing.B) {
				input := map[string]interface{}{
					"hit":      true,
					"password": "$up3r$Ecr3t",
					"ssn":      "123-45-6789",
				}
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					b.StartTimer()

					bodyJSON, err := rt.GetDataWithInput("data/test/rule", input)
					if err != nil {
						b.Fatal(err)
					}

					b.StopTimer()

					parsedBody := struct {
						Result bool `json:"result"`
					}{}

					err = json.Unmarshal(bodyJSON, &parsedBody)
					if err != nil {
						b.Fatalf("Failed to parse body: \n\nActual: %s\n\nExpected: {\"result\": BOOL}\n\nerr = %s ", string(bodyJSON), err)
					}
					expected := hitCount != 0
					if parsedBody.Result != expected {
						b.Fatalf("Unexpected result: %v", parsedBody.Result)
					}
				}
			})
		}
	}
}

// GeneratePolicy generates a policy for use in Decision Log e2e tests. The
// `ruleCounts` determine how many total rules to generate, and the `ruleHits`
// are the number of them that will be evaluated. This is keyed off of
// the `input.hit` boolean value.
func GeneratePolicy(ruleCounts int, ruleHits int) string {
	pb := strings.Builder{}
	pb.WriteString("package test\n")
	hits := 0
	for i := 0; i < ruleCounts; i++ {
		pb.WriteString("rule {")
		if hits < ruleHits {
			pb.WriteString("input.hit = true")
			hits++
		} else {
			pb.WriteString("input.hit = false")
		}
		pb.WriteString("}\n")
	}
	return pb.String()
}

// TestLogServer implements the decision log endpoint for e2e testing.
type TestLogServer struct {
	server   *http.Server
	listener net.Listener
}

// URL string representation for the current server. Requires that the server
// has already been started.
func (t *TestLogServer) URL() string {
	return fmt.Sprintf("http://%s", t.listener.Addr().String())
}

// Start the test server listening on a random port.
func (t *TestLogServer) Start() {
	var err error
	t.listener, err = net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	t.server = &http.Server{}
	t.server.SetKeepAlivesEnabled(false)
	go func() {
		err = t.server.Serve(t.listener)
		if err != http.ErrServerClosed {
			panic(err)
		}
	}()
}

// Stop the test server. There is a 5 second graceful shutdown period and then
// it will be forcefully stopped.
func (t *TestLogServer) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*time.Duration(5))
	_ = t.server.Shutdown(ctx)
	cancel()
	err := t.server.Close()
	if err != nil {
		panic(err)
	}
}
