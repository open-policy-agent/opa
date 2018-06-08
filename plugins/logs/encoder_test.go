// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"testing"
	"time"
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
