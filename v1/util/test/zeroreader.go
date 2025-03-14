// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package test

// ZeroReader is an io.Reader implementation that returns an infinite stream of zeros
type ZeroReader struct{}

func (ZeroReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// NewZeroReader creates a new ZeroReader
func NewZeroReader() *ZeroReader {
	return &ZeroReader{}
}
