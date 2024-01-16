// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package test

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

func Eventually(t *testing.T, timeout time.Duration, f func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if f() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

type BlockingWriter struct {
	m   sync.Mutex
	buf bytes.Buffer
}

func (w *BlockingWriter) Write(p []byte) (n int, err error) {
	w.m.Lock()
	defer w.m.Unlock()
	return w.buf.Write(p)
}

func (w *BlockingWriter) String() string {
	w.m.Lock()
	defer w.m.Unlock()
	return w.buf.String()
}

func (w *BlockingWriter) Reset() {
	w.m.Lock()
	defer w.m.Unlock()
	w.buf.Reset()
}
