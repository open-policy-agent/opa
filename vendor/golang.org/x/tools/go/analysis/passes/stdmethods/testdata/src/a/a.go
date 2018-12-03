// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package a

import "fmt"

type T int

func (T) Scan(x fmt.ScanState, c byte) {} // want "should have signature Scan"

func (T) Format(fmt.State, byte) {} // want `should have signature Format\(fmt.State, rune\)`

type U int

func (U) Format(byte) {} // no error: first parameter must be fmt.State to trigger check

func (U) GobDecode() {} // want `should have signature GobDecode\(\[\]byte\) error`

type I interface {
	ReadByte() byte // want "should have signature ReadByte"
}
