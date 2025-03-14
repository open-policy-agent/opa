package logs

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/keys"
	"github.com/open-policy-agent/opa/v1/plugins/rest"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func TestEventBuffer_Push(t *testing.T) {
	t.Parallel()

	limit := int64(2)
	b := newEventBuffer(limit, rest.Client{}, "", 200)
	dropped, err := b.Push(newTestEvent(t, "1", false))
	if err != nil {
		t.Fatal(err)
	}
	if dropped {
		t.Fatal("expected no events to be dropped")
	}
	dropped, err = b.Push(newTestEvent(t, "2", false))
	if err != nil {
		t.Fatal(err)
	}
	if dropped {
		t.Fatal("expected no events to be dropped")
	}
	dropped, err = b.Push(newTestEvent(t, "3", false))
	if err != nil {
		t.Fatal(err)
	}
	if !dropped {
		t.Fatal("expected 1 event to be dropped")
	}

	if int64(len(b.buffer)) != limit {
		t.Fatalf("buffer size mismatch, expected %d, got %d", limit, len(b.buffer))
	}

	// drop all events that don't meet the upload size limit anymore
	droppedCount, errs := b.Reconfigure(limit, rest.Client{}, "", 100)
	for _, err := range errs {
		expectedErrorMsg := "upload chunk size (195) exceeds upload_size_limit_bytes (100)"
		if err == nil {
			t.Fatal("error expected")
		} else if err.Error() != expectedErrorMsg {
			t.Fatalf("expected error %v but got %v", expectedErrorMsg, err.Error())
		}
	}
	if droppedCount != 2 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 2, droppedCount)
	}
	dropped, err = b.Push(newTestEvent(t, "4", false))
	expectedErrorMsg := "upload chunk size (195) exceeds upload_size_limit_bytes (100)"
	if err == nil {
		t.Fatal("error expected")
	} else if err.Error() != expectedErrorMsg {
		t.Fatalf("expected error %v but got %v", expectedErrorMsg, err.Error())
	}
	if dropped {
		t.Fatal("expected no events to be dropped")
	}

	droppedCount, errs = b.Reconfigure(limit, rest.Client{}, "", 196)
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if droppedCount != 0 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 0, droppedCount)
	}
	dropped, err = b.Push(newTestEvent(t, "5", false))
	if err != nil {
		t.Fatal(err)
	}
	if dropped {
		t.Fatal("expected no events to be dropped")
	}
	dropped, err = b.Push(newTestEvent(t, "6", true))
	if !errors.Is(err, droppedNDCache{}) {
		t.Fatalf("expected error %v but got %v", droppedNDCache{}.Error(), err.Error())
	}
	if dropped {
		t.Fatal("expected no events to be dropped")
	}

	dropped, err = b.Push(newTestEvent(t, "7", true))
	if !errors.Is(err, droppedNDCache{}) {
		t.Fatalf("expected error %v but got %v", droppedNDCache{}.Error(), err.Error())
	}
	if !dropped {
		t.Fatal("expected 1 event to be dropped")
	}

	if int64(len(b.buffer)) != 2 {
		t.Fatalf("buffer size mismatch, expected %d, got %d", 2, len(b.buffer))
	}

	limit = int64(3)
	droppedCount, errs = b.Reconfigure(limit, rest.Client{}, "", 200)
	for _, err := range errs {
		if !errors.Is(err, droppedNDCache{}) && err != nil {
			t.Fatal(err)
		}
	}
	if droppedCount != 0 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 0, droppedCount)
	}
	dropped, err = b.Push(newTestEvent(t, "8", false))
	if err != nil {
		t.Fatal(err)
	}
	if dropped {
		t.Fatal("expected no events to be dropped")
	}

	// change nothing
	droppedCount, errs = b.Reconfigure(limit, rest.Client{}, "", 200)
	for _, err := range errs {
		if !errors.Is(err, droppedNDCache{}) && err != nil {
			t.Fatal(err)
		}
	}
	if droppedCount != 0 {
		t.Fatalf("number of dropped event mismatch, expected %d, got %d", 0, droppedCount)
	}

	close(b.buffer)
	events := make([]EventV1, 0, limit)
	for event := range b.buffer {
		var e EventV1
		if err := json.Unmarshal(event, &e); err != nil {
			t.Fatal(err)
		}
		if e.DecisionID == "1" {
			t.Fatal("got unexpected decision ID 1")
		}

		events = append(events, e)
	}

	if int64(len(events)) != limit {
		t.Errorf("EventBuffer pushed %d events, expected %d", len(events), limit)
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
			name:                 "Trigger upload",
			eventLimit:           4,
			numberOfEvents:       3,
			uploadSizeLimitBytes: int64(32768),
			handleFunc: func(w http.ResponseWriter, r *http.Request) {
				events := readEventBody(t, r.Body)
				if len(events) != 3 {
					t.Errorf("expected 3 events, got %d", len(events))
				}

				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name:                 "Trigger upload and hit upload size limit",
			eventLimit:           4,
			numberOfEvents:       4,
			uploadSizeLimitBytes: 400, // Each test event is 195 bytes
			handleFunc: func(w http.ResponseWriter, r *http.Request) {
				events := readEventBody(t, r.Body)
				if len(events) != 2 {
					t.Errorf("expected 2 events, got %d", len(events))
				}
				w.WriteHeader(http.StatusOK)
			},
		},
		{
			name:                 "Get error from failed upload",
			eventLimit:           1,
			numberOfEvents:       1,
			uploadSizeLimitBytes: int64(32768),
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

			for i := range tc.numberOfEvents {
				_, err := e.Push(newTestEvent(t, strconv.Itoa(i), false))
				if err != nil {
					t.Fatal(err)
				}
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

func TestEventBuffer_Reconfigure(t *testing.T) {
	t.Parallel()

	bufferLimit := int64(3)
	client := rest.Client{}
	uploadPath := ""
	uploadLimit := int64(300)

	b := newEventBuffer(bufferLimit, client, uploadPath, uploadLimit)
	if int64(cap(b.buffer)) != bufferLimit {
		t.Fatalf("expected buffer size %d, got %d", bufferLimit, cap(b.buffer))
	}

	// add events that should be copied between buffers during resizing
	for range 4 {
		_, err := b.Push(newTestEvent(t, "1", true))
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(b.buffer) != 3 {
		t.Fatalf("expected 3 events, got %d", len(b.buffer))
	}

	bufferLimit = int64(1)
	b.Reconfigure(bufferLimit, client, uploadPath, int64(195)) // size without an ND cache

	if int64(cap(b.buffer)) != bufferLimit {
		t.Fatalf("expected buffer size %d, got %d", bufferLimit, cap(b.buffer))
	}
	if len(b.buffer) != 1 {
		t.Fatalf("expected 1 events, got %d", len(b.buffer))
	}

	bufferLimit = int64(4)
	b.Reconfigure(bufferLimit, client, uploadPath, uploadLimit)

	if int64(cap(b.buffer)) != bufferLimit {
		t.Fatalf("expected buffer size %d, got %d", bufferLimit, cap(b.buffer))
	}
	if len(b.buffer) != 1 {
		t.Fatalf("expected 1 events, got %d", len(b.buffer))
	}
}

func newTestEvent(t *testing.T, id string, enableNDCache bool) EventV1 {
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

	return e
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
