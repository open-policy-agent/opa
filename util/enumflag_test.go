// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"strings"
	"testing"
)

func TestEnumFlag(t *testing.T) {

	flag := NewEnumFlag("foo", []string{"foo", "bar", "baz"})

	if flag.String() != "foo" {
		t.Fatalf("Expected default value to be foo but got: %v", flag.String())
	}

	if err := flag.Set("bar"); err != nil {
		t.Fatalf("Unexpected error on set: %v", err)
	}

	if flag.String() != "bar" {
		t.Fatalf("Expected value to be bar but got: %v", flag.String())
	}

	if !strings.Contains(flag.Type(), "foo,bar,baz") {
		t.Fatalf("Expected flag type to contain foo,bar,baz but got: %v", flag.Type())
	}

	if err := flag.Set("deadbeef"); err == nil {
		t.Fatalf("Expected error from set")
	}
}
