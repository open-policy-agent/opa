// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"bytes"
	"testing"
)

func TestChunkEncoder(t *testing.T) {

	enc := newChunkEncoder(1000)

	bs, err := enc.Write(bytes.Repeat([]byte(`1`), 500))
	if bs != nil || err != nil {
		t.Fatalf("Unexpected error or chunk produced: err: %v", err)
	}

	bs, err = enc.Write(bytes.Repeat([]byte(`1`), 498))
	if bs != nil || err != nil {
		t.Fatalf("Unexpected error or chunk produced: err: %v", err)
	}

	bs, err = enc.Write(bytes.Repeat([]byte(`1`), 1))
	if bs == nil || err != nil {
		t.Fatalf("Unexpected error or NO chunk produced: err: %v", err)
	}

	bs, err = enc.Write(bytes.Repeat([]byte(`1`), 1))
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

	bs, err = enc.Write(nil)
	if bs != nil || err != nil || enc.bytesWritten != 0 {
		t.Fatalf("Unexpected error chunk produced or bytesWritten != 0: err: %v, bytesWritten: %v", err, enc.bytesWritten)
	}

}
