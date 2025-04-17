// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"strconv"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/metrics"
)

func TestChunkEncoder(t *testing.T) {
	enc := newChunkEncoder(1000)
	var result interface{} = false
	var expInput interface{} = map[string]interface{}{"method": "GET"}
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

	bs, err := enc.Write(event)
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
	enc := newChunkEncoder(1).WithMetrics(metrics.New())
	var result interface{} = false
	var expInput interface{} = map[string]interface{}{"method": "GET"}
	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		t.Fatal(err)
	}
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
	chunks, err := enc.Write(event)
	if err != nil {
		t.Error(err)
	}
	if len(chunks) != 0 {
		t.Errorf("Unexpected result: %v", result)
	}
	if err := enc.w.Flush(); err != nil {
		t.Fatal(err)
	}
	// only the expected flush contents (header+Z_SYNC_FLUSH content) without the event is expected
	if enc.buf.Len() != 15 {
		t.Errorf("Unexpected buffer size: %v", enc.buf.Len())
	}
	if enc.bytesWritten != 0 {
		t.Errorf("Unexpected bytes written: %v", enc.bytesWritten)
	}
}

func TestChunkEncoderAdaptive(t *testing.T) {
	enc := newChunkEncoder(1000).WithMetrics(metrics.New())
	var result interface{} = false
	var expInput interface{} = map[string]interface{}{"method": "GET"}
	ts, err := time.Parse(time.RFC3339Nano, "2018-01-01T12:00:00.123456Z")
	if err != nil {
		panic(err)
	}

	var chunks [][]byte
	numEvents := 400
	for i := range numEvents {

		bundles := map[string]BundleInfoV1{}
		bundles["authz"] = BundleInfoV1{Revision: strconv.Itoa(i)}

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

		chunk, err := enc.Write(event)
		if err != nil {
			t.Fatal(err)
		}
		if chunk != nil {
			chunks = append(chunks, chunk...)
		}
	}

	// decode the chunks and check the number of events is equal to the encoded events

	numEventsActual := decodeChunks(t, chunks)

	// flush the encoder
	for {
		bs, err := enc.Flush()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if len(bs) == 0 {
			break
		}

		numEventsActual += decodeChunks(t, bs)
	}

	if numEvents != numEventsActual {
		t.Fatalf("Expected %v events but got %v", numEvents, numEventsActual)
	}

	actualScaleUpEvents := enc.metrics.Counter(encUncompressedLimitScaleUpCounterName).Value().(uint64)
	actualScaleDownEvents := enc.metrics.Counter(encUncompressedLimitScaleDownCounterName).Value().(uint64)
	actualEquiEvents := enc.metrics.Counter(encUncompressedLimitStableCounterName).Value().(uint64)

	expectedScaleUpEvents := uint64(5)
	expectedScaleDownEvents := uint64(0)
	expectedEquiEvents := uint64(0)

	if actualScaleUpEvents != expectedScaleUpEvents {
		t.Fatalf("Expected scale up events %v but got %v", expectedScaleUpEvents, actualScaleUpEvents)
	}

	if actualScaleDownEvents != expectedScaleDownEvents {
		t.Fatalf("Expected scale down events %v but got %v", expectedScaleDownEvents, actualScaleDownEvents)
	}

	if actualEquiEvents != expectedEquiEvents {
		t.Fatalf("Expected equilibrium events %v but got %v", expectedEquiEvents, actualEquiEvents)
	}
}

func decodeChunks(t *testing.T, bs [][]byte) int {
	t.Helper()

	numEvents := 0
	for _, chunk := range bs {
		events, err := newChunkDecoder(chunk).decode()
		if err != nil {
			t.Fatal(err)
		}
		numEvents += len(events)
	}
	return numEvents
}
