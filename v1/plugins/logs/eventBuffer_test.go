package logs

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/keys"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func TestEventBuffer_Push(t *testing.T) {
	t.Parallel()

	limit := int64(2)
	b := newEventBuffer(limit, rest.Client{}, "", 200)
	p := &Plugin{
		metrics: metrics.New(),
	}
	b.Push(p, newTestEvent(t, "1", false))
	dropped := p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64)
	if p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64) != 0 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 0, dropped)
	}
	b.Push(p, newTestEvent(t, "2", false))
	dropped = p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64)
	if p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64) != 0 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 0, dropped)
	}
	b.Push(p, newTestEvent(t, "3", false))
	dropped = p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64)
	if p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64) != 1 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 1, dropped)
	}

	if int64(len(b.buffer)) != limit {
		t.Fatalf("buffer size mismatch, expected %d, got %d", limit, len(b.buffer))
	}

	limit = int64(3)
	b.Reconfigure(p, limit, rest.Client{}, "", 100)
	dropped = p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64)
	if dropped != 1 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 1, dropped)
	}
	b.Push(p, newTestEvent(t, "4", false))
	dropped = p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64)
	if dropped != 1 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 1, dropped)
	}
	b.Push(p, newTestEvent(t, "5", true))
	dropped = p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64)
	if dropped != 2 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 2, dropped)
	}

	if int64(len(b.buffer)) != limit {
		t.Fatalf("buffer size mismatch, expected %d, got %d", limit, len(b.buffer))
	}

	limit = int64(1)
	b.Reconfigure(p, limit, rest.Client{}, "", 200)
	dropped = p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64)
	if dropped != 4 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 4, dropped)
	}
	if int64(len(b.buffer)) != limit {
		t.Fatalf("buffer size mismatch, expected %d, got %d", limit, len(b.buffer))
	}

	b.Reconfigure(p, limit, rest.Client{}, "", 200)
	dropped = p.metrics.Counter(logBufferEventLimitExDropCounterName).Value().(uint64)
	if dropped != 4 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 04, dropped)
	}
	if int64(len(b.buffer)) != limit {
		t.Fatalf("buffer size mismatch, expected %d, got %d", limit, len(b.buffer))
	}
}

func TestEventBuffer_Upload(t *testing.T) {
	t.Parallel()

	uploadPath := "/v1/test"

	tests := []struct {
		name                 string
		eventLimit           int64
		numberOfEvents       int
		uploadSizeLimitBytes int64
		handleFunc           func(w http.ResponseWriter, r *http.Request)
		expectedError        string
	}{
		{
			name:                 "Upload everything in the buffer",
			eventLimit:           4,
			numberOfEvents:       3,
			uploadSizeLimitBytes: defaultUploadSizeLimitBytes,
			handleFunc: func(w http.ResponseWriter, r *http.Request) {
				events := readEventBody(t, r.Body)
				if len(events) != 3 {
					t.Errorf("expected 3 events, got %d", len(events))
				}

				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name:                 "Upload in chunks determined by upload size limit",
			eventLimit:           4,
			numberOfEvents:       4,
			uploadSizeLimitBytes: 200, // Each test event is 195 bytes
			handleFunc: func(w http.ResponseWriter, r *http.Request) {
				events := readEventBody(t, r.Body)
				if len(events) != 1 {
					t.Errorf("expected 1 events, got %d", len(events))
				}
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name:                 "Get error from failed upload",
			eventLimit:           1,
			numberOfEvents:       1,
			uploadSizeLimitBytes: defaultUploadSizeLimitBytes,
			handleFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			expectedError: "log upload failed, server replied with HTTP 400 Bad Request",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, ts := setupTestServer(t, uploadPath, tc.handleFunc)
			defer ts.Close()
			e := newEventBuffer(tc.eventLimit, client, uploadPath, tc.uploadSizeLimitBytes)
			p := &Plugin{
				metrics: metrics.New(),
				logger:  logging.NewNoOpLogger(),
			}

			for i := range tc.numberOfEvents {
				e.Push(p, newTestEvent(t, strconv.Itoa(i), true))
			}

			ok, err := e.Upload(context.Background(), p)
			if !ok || err != nil {
				if tc.expectedError == "" || tc.expectedError != "" && err.Error() != tc.expectedError {
					t.Fatal(err)
				}
			}
		})
	}
}

func newTestEvent(t *testing.T, id string, enableNDCache bool) bufferItem {
	var result interface{} = false
	var expInput interface{} = map[string]interface{}{"method": "GET"}
	timestamp, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		t.Fatal(err)
	}
	e := EventV1{
		Labels: map[string]string{
			"id":  "test-instance-id",
			"app": "example-app",
		},
		DecisionID:  id,
		Path:        "foo/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   timestamp,
	}

	if enableNDCache {
		var ndbCacheExample = ast.MustJSON(builtins.NDBCache{
			"time.now_ns": ast.NewObject([2]*ast.Term{
				ast.ArrayTerm(),
				ast.NumberTerm("1663803565571081429"),
			}),
		}.AsValue())
		e.NDBuiltinCache = &ndbCacheExample
	}

	return bufferItem{EventV1: e}
}

func setupTestServer(t *testing.T, uploadPath string, handleFunc func(w http.ResponseWriter, r *http.Request)) (rest.Client, *httptest.Server) {
	mux := http.NewServeMux()
	ts := httptest.NewServer(mux)

	mux.HandleFunc(uploadPath, handleFunc)

	config := fmt.Sprintf(`{
		"name": "foo",
		"url": %q,
		"response_header_timeout_seconds": 20,
	}`, ts.URL)
	ks := map[string]*keys.Config{}
	client, err := rest.New([]byte(config), ks)
	if err != nil {
		t.Fatal(err)
	}

	return client, ts
}

func readEventBody(t *testing.T, r io.Reader) []EventV1 {
	gr, err := gzip.NewReader(r)
	if err != nil {
		t.Fatal(err)
	}
	var events []EventV1
	if err := json.NewDecoder(gr).Decode(&events); err != nil {
		t.Fatal(err)
	}
	if err := gr.Close(); err != nil {
		t.Fatal(err)
	}

	return events
}
