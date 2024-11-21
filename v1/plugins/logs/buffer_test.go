// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logs

import (
	"bytes"
	"testing"
)

func TestLogBuffer(t *testing.T) {

	buffer := newLogBuffer(int64(20)) // 20 byte limit for test purposes

	dropped := buffer.Push(make([]byte, 20))
	if dropped != 0 {
		t.Fatal("Expected dropped to be zero")
	}

	bs := buffer.Pop()
	if len(bs) != 20 {
		t.Fatal("Expected buffer size to be 20")
	}

	bs = buffer.Pop()
	if bs != nil {
		t.Fatal("Expected buffer to be nil")
	}

	dropped = buffer.Push(bytes.Repeat([]byte(`1`), 10))
	if dropped != 0 {
		t.Fatal("Expected dropped to be zero")
	}

	dropped = buffer.Push(bytes.Repeat([]byte(`2`), 10))
	if dropped != 0 {
		t.Fatal("Expected dropped to be zero")
	}

	dropped = buffer.Push(bytes.Repeat([]byte(`3`), 10))
	if dropped != 1 {
		t.Fatal("Expected dropped to be 1")
	}

	bs = buffer.Pop()
	exp := bytes.Repeat([]byte(`2`), 10)
	if !bytes.Equal(bs, exp) {
		t.Fatalf("Expected %v but got %v", exp, bs)
	}

	if buffer.usage != 10 {
		t.Fatalf("Expected buffer usage to be 10 but got %v", buffer.usage)
	}

}
