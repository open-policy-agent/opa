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

	expectedIds := make(map[string]struct{})
	var expectedDropped uint64
	limit := int64(2)
	b := newEventBuffer(limit, rest.Client{}, "", 0).WithMetrics(metrics.New())

	id := "id1"
	expectedIds[id] = struct{}{}
	b.Push(newTestEvent(t, id, false))
	checkBufferState(t, limit, b, expectedDropped, expectedIds)

	id = "id2"
	expectedIds[id] = struct{}{}
	b.Push(newTestEvent(t, id, false))
	checkBufferState(t, limit, b, expectedDropped, expectedIds)

	id = "id3"
	expectedIds[id] = struct{}{}
	b.Push(newTestEvent(t, id, false))
	// Three events were pushed, but limit is 2 so the oldest even should have been dropped
	delete(expectedIds, "id1")
	expectedDropped++
	checkBufferState(t, limit, b, expectedDropped, expectedIds)

	if int64(len(b.buffer)) != limit {
		t.Fatalf("buffer size mismatch, expected %d, got %d", limit, len(b.buffer))
	}

	// Increase the limit, forcing the buffer to change
	limit = int64(3)
	b.Reconfigure(limit, rest.Client{}, "", 0)
	checkBufferState(t, limit, b, expectedDropped, expectedIds)

	id = "id4"
	expectedIds[id] = struct{}{}
	b.Push(newTestEvent(t, id, false))
	checkBufferState(t, limit, b, expectedDropped, expectedIds)

	id = "id5"
	expectedIds[id] = struct{}{}
	b.Push(newTestEvent(t, id, true))
	// Four events were pushed, but limit is 3 so the oldest even should have been dropped
	expectedDropped++
	delete(expectedIds, "id2")
	checkBufferState(t, limit, b, expectedDropped, expectedIds)

	limit = int64(1)
	b.Reconfigure(limit, rest.Client{}, "", 0)
	// Limit reconfigured from 3->1, dropping 2 more events.
	expectedDropped = 4
	delete(expectedIds, "id3")
	delete(expectedIds, "id4")
	checkBufferState(t, limit, b, expectedDropped, expectedIds)

	// Nothing changed
	b.Reconfigure(limit, rest.Client{}, "", 0)
	checkBufferState(t, limit, b, expectedDropped, expectedIds)
}

func checkBufferState(t *testing.T, limit int64, b *eventBuffer, expectedDropped uint64, expectedIds map[string]struct{}) {
	t.Helper()

	dropped := b.metrics.Counter(logBufferEventDropCounterName).Value().(uint64)
	if dropped != expectedDropped {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", expectedDropped, dropped)
	}

	if len(b.buffer) != len(expectedIds) {
		t.Fatalf("buffer size mismatch, expected %d, got %d", len(expectedIds), len(b.buffer))
	}

	close(b.buffer)
	newBuffer := make(chan bufferItem, limit)
	for event := range b.buffer {
		if _, ok := expectedIds[event.DecisionID]; !ok {
			t.Fatalf("received unexpected event %v", event)
		}
		newBuffer <- event
	}

	b.buffer = newBuffer
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
			e := newEventBuffer(tc.eventLimit, client, uploadPath, tc.uploadSizeLimitBytes).WithMetrics(metrics.New()).WithLogger(logging.NewNoOpLogger())

			for i := range tc.numberOfEvents {
				e.Push(newTestEvent(t, strconv.Itoa(i), true))
			}

			ok, err := e.Upload(context.Background())
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

	return bufferItem{EventV1: &e}
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
