// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"bytes"
	"strings"
	"testing"
)

func TestPretty(t *testing.T) {

	module := MustParseModule(`
	package foo.bar

	import data.baz as qux

	p[x] = y {
		x = a + b
		y = {"foo": [{1, null}, true]}
	}

	f(x) = g(x)
	`)

	var buf bytes.Buffer
	Pretty(&buf, module)

	expected := `module
 package
  ref
   data
   "foo"
   "bar"
 import
  ref
   data
   "baz"
  qux
 rule
  head
   ref
    p
    x
   y
  body
   expr index=0
    ref
     eq
    x
    call
     ref
      plus
     a
     b
   expr index=1
    ref
     eq
    y
    object
     "foo"
     array
      set
       null
       1
      true
 rule
  head
   ref
    f
   args
    x
   call
    ref
     g
    x
  body
   expr index=0
    true`

	result := strings.TrimSpace(buf.String())
	expected = strings.TrimSpace(expected)

	if result != expected {

		resultLines := strings.Split(result, "\n")
		expectedLines := strings.Split(expected, "\n")

		minLines := len(resultLines)
		if minLines > len(expectedLines) {
			minLines = len(expectedLines)
		}

		for i := 0; i < minLines; i++ {
			if resultLines[i] != expectedLines[i] {
				t.Fatalf("Expected line %d to be:\n\n%q\n\nGot:\n\n%q", i, expectedLines[i], resultLines[i])
			}
		}

		if len(resultLines) != len(expectedLines) {
			t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", expectedLines, resultLines)
		}
	}
}
