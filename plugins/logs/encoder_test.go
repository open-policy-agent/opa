// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/metrics"
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
	_, err = enc.Write(event)
	if err == nil {
		t.Error("Expected error as upload chunk size exceeds configured limit")
	}
	expected := "upload chunk size (200) exceeds upload_size_limit_bytes (1)"
	if err.Error() != expected {
		t.Errorf("expected: '%s', got: '%s'", expected, err.Error())
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
	for i := 0; i < numEvents; i++ {

		bundles := map[string]BundleInfoV1{}
		bundles["authz"] = BundleInfoV1{Revision: fmt.Sprint(i)}

		event := EventV1{
			Labels: map[string]string{
				"id":  "test-instance-id",
				"app": "example-app",
			},
			Bundles:     bundles,
			DecisionID:  fmt.Sprint(i),
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

	actualScaleUpEvents := enc.metrics.Counter(encSoftLimitScaleUpCounterName).Value().(uint64)
	actualScaleDownEvents := enc.metrics.Counter(encSoftLimitScaleDownCounterName).Value().(uint64)
	actualEquiEvents := enc.metrics.Counter(encSoftLimitStableCounterName).Value().(uint64)

	expectedScaleUpEvents := uint64(8)
	expectedScaleDownEvents := uint64(3)
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
