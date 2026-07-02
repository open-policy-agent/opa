// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/logging"
	testLogger "github.com/open-policy-agent/opa/v1/logging/test"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func eventWithNDCache() EventV1 {
	// Purposely oversize NDBCache entry will force dropping during Log().
	ndbCacheExample := ast.MustJSON(builtins.NDBCache{
		"test.custom_space_waster": ast.NewObject([2]*ast.Term{
			ast.ArrayTerm(),
			ast.StringTerm(strings.Repeat("Wasted space... ", 200)),
		}),
	}.AsValue())
	var result any = false
	var expInput any = map[string]any{"method": "GET"}
	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	return EventV1{
		DecisionID:     "abc",
		Path:           "foo/bar",
		Input:          &expInput,
		Result:         &result,
		RequestedBy:    "test",
		Timestamp:      ts,
		NDBuiltinCache: &ndbCacheExample,
	}
}

func TestLastDroppedNDSize(t *testing.T) {
	enc := newChunkEncoder(200).WithMetrics(metrics.New())

	if enc.lastDroppedNDSize != 0 {
		t.Errorf("expected 0 got %d", enc.lastDroppedNDSize)
	}

	event := eventWithNDCache()

	eventBytes, err := json.Marshal(&event)
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := enc.Encode(event, eventBytes)
	if err != nil {
		t.Fatal(err)
	}

	var expectedChunks int
	if len(chunk) != expectedChunks {
		t.Errorf("expected %v chunks, got %d", expectedChunks, len(chunk))
	}

	expectedBytesWritten := 157 // size after dropping the ND cache
	if enc.bytesWritten != expectedBytesWritten {
		t.Errorf("Expected %d bytes written but got %d", expectedBytesWritten, enc.bytesWritten)
	}

	expectedLastDroppedSize := int64(3414) // size before dropping the ND cache
	if enc.lastDroppedNDSize != expectedLastDroppedSize {
		t.Errorf("expected %v got %d", expectedLastDroppedSize, enc.lastDroppedNDSize)
	}

	eventBytes, err = json.Marshal(&event)
	if err != nil {
		t.Fatal(err)
	}
	chunk, err = enc.Encode(event, eventBytes)
	if err != nil {
		t.Fatal(err)
	}

	if len(chunk) != expectedChunks {
		t.Errorf("expected %v chunks, got %d", expectedChunks, len(chunk))
	}

	expectedBytesWritten = 314 // size of two events written
	if enc.bytesWritten != expectedBytesWritten {
		t.Errorf("Expected %d bytes written but got %d", expectedBytesWritten, enc.bytesWritten)
	}

	if enc.lastDroppedNDSize != expectedLastDroppedSize {
		t.Errorf("expected %v got %d", expectedLastDroppedSize, enc.lastDroppedNDSize)
	}
}

func TestChunkMaxUploadSizeLimitNDBCacheDropping(t *testing.T) {
	t.Parallel()

	// note this only tests the size buffer type so that the encoder is used immediately on log
	tests := []struct {
		name                         string
		uploadSizeLimitBytes         int64
		expectedDroppedNDCacheEvents uint64
		expectedBytesWritten         int
		expectedUncompressedLimit    int64
		expectedEncodingFailures     uint64
	}{
		{
			name:                         "drop ND cache to fit",
			uploadSizeLimitBytes:         200,
			expectedDroppedNDCacheEvents: 1,
			// written after dropping the ND cache, otherwise the size would have been 3472
			expectedBytesWritten:      157,
			expectedUncompressedLimit: 400,
		},
		{
			name:                      "dropping the ND cache doesn't help, drop the event",
			uploadSizeLimitBytes:      120,
			expectedUncompressedLimit: 120, // never gets adjusted
			expectedEncodingFailures:  1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enc := newChunkEncoder(tc.uploadSizeLimitBytes).WithMetrics(metrics.New())

			event := eventWithNDCache()

			eventBytes, err := json.Marshal(&event)
			if err != nil {
				t.Fatal(err)
			}
			chunk, err := enc.Encode(event, eventBytes)
			if err != nil {
				t.Fatal(err)
			}

			if enc.metrics.Counter(logEncodingFailureCounterName).Value().(uint64) != tc.expectedEncodingFailures {
				t.Errorf("Expected %d dropped events but got %d", tc.expectedDroppedNDCacheEvents, enc.metrics.Counter(logNDBDropCounterName))
			}

			if enc.metrics.Counter(logNDBDropCounterName).Value().(uint64) != tc.expectedDroppedNDCacheEvents {
				t.Errorf("Expected %d dropped events but got %d", tc.expectedDroppedNDCacheEvents, enc.metrics.Counter(logNDBDropCounterName))
			}

			if chunk != nil {
				t.Errorf("expected nil result but got %v", chunk)
			}

			if enc.bytesWritten != tc.expectedBytesWritten {
				t.Errorf("Expected %d bytes written but got %d", tc.expectedBytesWritten, enc.bytesWritten)
			}

			if enc.uncompressedLimit != tc.expectedUncompressedLimit {
				t.Errorf("Expected %d uncompressed limit but got %d", tc.expectedUncompressedLimit, enc.uncompressedLimit)
			}
		})
	}
}

func TestChunkEncoder(t *testing.T) {
	t.Parallel()

	enc := newChunkEncoder(1000)
	var result any = false
	var expInput any = map[string]any{"method": "GET"}
	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	event := EventV1{
		Labels: map[string]string{
			"id":  "test-instance-id",
			"app": "example-app",
		},
		Revision:    "a",
		DecisionID:  "a",
		Path:        "foo/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
	}

	eventBytes, err := json.Marshal(&event)
	if err != nil {
		t.Fatal(err)
	}
	bs, err := enc.Encode(event, eventBytes)
	if bs != nil || err != nil {
		t.Fatalf("Unexpected error or chunk produced: err: %v", err)
	}

	bs, err = enc.Flush()
	if bs == nil || err != nil {
		t.Fatalf("Unexpected error or NO chunk produced: err: %v", err)
	}

	bs, err = enc.Flush()
	if bs != nil || err != nil {
		t.Fatalf("Unexpected error chunk produced: err: %v", err)

	}
}

func TestChunkEncoderSizeLimit(t *testing.T) {
	t.Parallel()

	enc := newChunkEncoder(90).WithMetrics(metrics.New())
	var result any = false
	var expInput any = map[string]any{"method": "GET"}
	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		t.Fatal(err)
	}

	// test adding a small event that would fit in the minUploadSizeLimitBytes
	smallestEvent := EventV1{
		DecisionID: "1",
	}

	eventBytes, err := json.Marshal(&smallestEvent)
	if err != nil {
		t.Fatal(err)
	}
	chunks, err := enc.Encode(smallestEvent, eventBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 0 {
		t.Errorf("Unexpected result: %v", result)
	}
	if err := enc.w.Flush(); err != nil {
		t.Fatal(err)
	}
	// expect the event to be written because it fits the minimum event size
	expectedBufferSize := 78 // the compressed size of an absurd small event
	if enc.buf.Len() != expectedBufferSize {
		t.Errorf("Expected %v buffer size but got: %v", expectedBufferSize, enc.buf.Len())
	}
	expectedBytesWritten := 69 // the uncompressed size of the event
	if enc.bytesWritten != expectedBytesWritten {
		t.Errorf("Expected %v bytes written but got: %v", expectedBytesWritten, enc.bytesWritten)
	}
	expectedEventsWritten := int64(1)
	if enc.eventsWritten != expectedEventsWritten {
		t.Errorf("Expected %v events written but got: %v", expectedEventsWritten, enc.eventsWritten)
	}

	// write an event that is too big
	event := EventV1{
		Labels: map[string]string{
			"id":  "test-instance-id",
			"app": "example-app",
		},
		DecisionID:  "123",
		Path:        "foo/bar",
		Input:       &expInput,
		Result:      &result,
		RequestedBy: "test",
		Timestamp:   ts,
	}

	logger := testLogger.New()
	enc.WithLogger(logger)
	eventBytes, err = json.Marshal(&event)
	if err != nil {
		t.Fatal(err)
	}
	chunks, err = enc.Encode(event, eventBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Errorf("Unexpected chunk: %v", chunks)
	}
	// the incoming event doesn't fit, but the previous small event size is between 90-100% capacity so chunk is returned
	// the incoming event is too large and will be dropped
	actualStableCounter := enc.metrics.Counter(encUncompressedLimitStableCounterName).Value().(uint64)
	expectedStableCounter := uint64(1)
	if actualStableCounter != expectedStableCounter {
		t.Errorf("Expected %d encoding failure but got: %d", expectedStableCounter, actualStableCounter)
	}
	if err := enc.w.Flush(); err != nil {
		t.Fatal(err)
	}
	expectedBufferSize = 15
	if enc.buf.Len() != expectedBufferSize {
		t.Errorf("Expected %v buffer size but got: %v", expectedBufferSize, enc.buf.Len())
	}
	expectedBytesWritten = 0
	if enc.bytesWritten != expectedBytesWritten {
		t.Errorf("Expected %v bytes written but got: %v", expectedBytesWritten, enc.bytesWritten)
	}
	expectedEventsWritten = 0
	if enc.eventsWritten != expectedEventsWritten {
		t.Errorf("Expected %v events written but got: %v", expectedEventsWritten, enc.eventsWritten)
	}

	entries := logger.Entries()
	expectedEntries := 1
	if len(entries) != expectedEntries {
		t.Fatalf("Expected %v log entry but got: %v", expectedEntries, len(entries))
	}
	if entries[0].Level != logging.Error &&
		entries[0].Message != "Log encoding failed: received a decision event size (176) that exceeded the upload_size_limit_bytes (25). No ND cache to drop." {
		t.Errorf("Unexpected log entry: %v", entries[0])
	}
	if enc.metrics.Counter(encUncompressedLimitScaleDownCounterName).Value().(uint64) != 0 {
		t.Fatalf("Expected zero uncompressed limit scale down: %v", enc.metrics.Counter(encUncompressedLimitScaleDownCounterName).Value().(uint64))
	}
	if enc.metrics.Counter(encLogExUploadSizeLimitCounterName).Value().(uint64) != 1 {
		t.Fatalf("Expected one upload size limit exceed but got: %v", enc.metrics.Counter(encLogExUploadSizeLimitCounterName).Value().(uint64))
	}
	if enc.metrics.Counter(logEncodingFailureCounterName).Value().(uint64) != 1 {
		t.Errorf("Expected one encoding failure but got: %v", enc.metrics.Counter(logEncodingFailureCounterName).Value().(uint64))
	}

	// 179 is the size of the event compressed into a chunk by itself
	// the eventBytes size is 197 uncompressed, bigger than the limit
	// this tests that a chunk should be returned containing this single event
	// because after compression it adheres to the limit
	enc = newChunkEncoder(179).WithMetrics(metrics.New())
	chunks, err = enc.Encode(event, eventBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Errorf("Unexpected chunk: %v", chunks)
	}

	// 198 is the size of eventBytes + 1, +1 accounting for the closing bracket
	// this tests that a chunk should be returned containing this single event
	// because after compression it adheres to the limit
	enc = newChunkEncoder(198).WithMetrics(metrics.New())
	chunks, err = enc.Encode(event, eventBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 {
		t.Errorf("Unexpected chunk: %v", chunks)
	}
}

func TestChunkEncoderAdaptive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                      string
		limit                     int64
		numEvents                 int
		expectedUncompressedLimit int64
		expectedMaxEventsInChunk  int64
		expectedScaleUpEvents     uint64
		expectedScaleDownEvents   uint64
		expectedEquiEvents        uint64
	}{
		{
			// only one event can fit, after one scale up the uncompressed limit falls within the 90-100% utilization
			name:                      "an uncompressed limit that stabilizes immediately",
			limit:                     201,
			numEvents:                 1000,
			expectedUncompressedLimit: 464,
			expectedMaxEventsInChunk:  1,
			expectedScaleUpEvents:     1,
			expectedScaleDownEvents:   0,
			expectedEquiEvents:        998,
		},
		{
			// 61 events can fit, but takes some guessing before it gets to the uncompressed limit 7200
			name:                      "an uncompressed limit that stabilizes after a few guesses",
			limit:                     400,
			numEvents:                 1000,
			expectedUncompressedLimit: 7200,
			expectedMaxEventsInChunk:  61,
			expectedScaleUpEvents:     5,
			expectedScaleDownEvents:   2,
			expectedEquiEvents:        31,
		},
		{
			// it is possible to set a limit that the algorithm fails to stabilize, it is just guessing
			name:                      "an uncompressed limit that doesn't stabilize",
			limit:                     1000,
			numEvents:                 1000,
			expectedUncompressedLimit: 40500,
			expectedMaxEventsInChunk:  341,
			expectedScaleUpEvents:     14,
			expectedScaleDownEvents:   11,
			expectedEquiEvents:        0,
		},
		{
			// a different limit can stabilize
			name:                      "larger limit that does stabilize",
			limit:                     3000,
			numEvents:                 2000,
			expectedUncompressedLimit: 108000,
			expectedMaxEventsInChunk:  908,
			expectedScaleUpEvents:     5,
			expectedScaleDownEvents:   1,
			expectedEquiEvents:        3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			enc := newChunkEncoder(tc.limit).WithMetrics(metrics.New())
			var result any = false
			var expInput any = map[string]any{"method": "GET"}
			ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
			if err != nil {
				panic(err)
			}

			var chunks [][]byte
			for i := range tc.numEvents {

				bundles := map[string]BundleInfoV1{}
				bundles["authz"] = BundleInfoV1{Revision: strconv.Itoa(i)}

				// uncompressed the size of an event is 232 bytes
				// when compressed by itself this event is 186 bytes
				event := EventV1{
					Labels: map[string]string{
						"id":  "test-instance-id",
						"app": "example-app",
					},
					Bundles:     bundles,
					DecisionID:  strconv.Itoa(i),
					Path:        "foo/bar",
					Input:       &expInput,
					Result:      &result,
					RequestedBy: "test",
					Timestamp:   ts,
				}

				eventBytes, err := json.Marshal(&event)
				if err != nil {
					t.Fatal(err)
				}
				chunk, err := enc.Encode(event, eventBytes)
				if err != nil {
					t.Fatal(err)
				}
				if chunk != nil {
					chunks = append(chunks, chunk...)
				}
			}

			if enc.uncompressedLimit != tc.expectedUncompressedLimit {
				t.Errorf("Expected %v uncompressed limit but got %v", tc.expectedUncompressedLimit, enc.uncompressedLimit)
			}

			h := enc.metrics.Histogram(encNumberOfEventsInChunkHistogramName).Value().(map[string]any)
			if h["max"].(int64) != tc.expectedMaxEventsInChunk {
				t.Errorf("Expected %v max events in a chunk, got %v", tc.expectedMaxEventsInChunk, h["max"].(int64))
			}

			// decode the chunks and check the number of events is equal to the encoded events
			actualEvents := decodeChunks(t, chunks)

			// flush the encoder
			for {
				bs, err := enc.Flush()
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				if len(bs) == 0 {
					break
				}

				actualEvents = append(actualEvents, decodeChunks(t, bs)...)
			}

			if tc.numEvents != len(actualEvents) {
				t.Fatalf("Expected %v events but got %v", tc.numEvents, len(actualEvents))
			}

			// make sure there aren't any missing IDs
			for i := range actualEvents {
				id, err := strconv.Atoi(actualEvents[i].DecisionID)
				if err != nil {
					t.Fatal(err)
				}
				if id != i {
					t.Fatalf("Expected decision ID %d but got %d", i, id)
				}
			}

			actualScaleUpEvents := enc.metrics.Counter(encUncompressedLimitScaleUpCounterName).Value().(uint64)
			actualScaleDownEvents := enc.metrics.Counter(encUncompressedLimitScaleDownCounterName).Value().(uint64)
			actualEquiEvents := enc.metrics.Counter(encUncompressedLimitStableCounterName).Value().(uint64)

			if actualScaleUpEvents != tc.expectedScaleUpEvents {
				t.Errorf("Expected scale up events %v but got %v", tc.expectedScaleUpEvents, actualScaleUpEvents)
			}

			if actualScaleDownEvents != tc.expectedScaleDownEvents {
				t.Errorf("Expected scale down events %v but got %v", tc.expectedScaleDownEvents, actualScaleDownEvents)
			}

			if actualEquiEvents != tc.expectedEquiEvents {
				t.Errorf("Expected equilibrium events %v but got %v", tc.expectedEquiEvents, actualEquiEvents)
			}
		})
	}
}

func decodeChunks(t *testing.T, bs [][]byte) []EventV1 {
	t.Helper()

	events := make([]EventV1, 0, len(bs))
	for _, chunk := range bs {
		e, err := newChunkDecoder(chunk).decode()
		if err != nil {
			t.Fatal(err)
		}
		events = append(events, e...)
	}
	return events
}
