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

func TestLogBufferDropsMultipleItems(t *testing.T) {
	buffer := newLogBuffer(int64(20))

	buffer.Push(bytes.Repeat([]byte(`a`), 5))
	buffer.Push(bytes.Repeat([]byte(`b`), 5))
	buffer.Push(bytes.Repeat([]byte(`c`), 5))
	buffer.Push(bytes.Repeat([]byte(`d`), 5))

	// Push a 15-byte item into a full 20-byte buffer.
	// Must evict 3 items (a, b, c) to make room, not just 1.
	dropped := buffer.Push(bytes.Repeat([]byte(`e`), 15))
	if dropped != 3 {
		t.Fatalf("Expected dropped to be 3 but got %v", dropped)
	}

	if buffer.Len() != 2 {
		t.Fatalf("Expected buffer length to be 2 but got %v", buffer.Len())
	}

	if buffer.usage != 20 {
		t.Fatalf("Expected buffer usage to be 20 but got %v", buffer.usage)
	}

	bs := buffer.Pop()
	if exp := bytes.Repeat([]byte(`d`), 5); !bytes.Equal(bs, exp) {
		t.Fatalf("Expected %v but got %v", exp, bs)
	}

	bs = buffer.Pop()
	if exp := bytes.Repeat([]byte(`e`), 15); !bytes.Equal(bs, exp) {
		t.Fatalf("Expected %v but got %v", exp, bs)
	}
}

func TestLogBufferOversizedItem(t *testing.T) {
	buffer := newLogBuffer(int64(20))

	buffer.Push(bytes.Repeat([]byte(`a`), 10))
	buffer.Push(bytes.Repeat([]byte(`b`), 10))

	dropped := buffer.Push(bytes.Repeat([]byte(`c`), 21))
	if dropped != 1 {
		t.Fatalf("Expected dropped to be 1 but got %v", dropped)
	}

	if buffer.Len() != 2 {
		t.Fatalf("Expected buffer length to be 2 but got %v", buffer.Len())
	}

	if buffer.usage != 20 {
		t.Fatalf("Expected buffer usage to be 20 but got %v", buffer.usage)
	}
}
