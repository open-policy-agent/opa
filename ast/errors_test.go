// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"testing"
)

func TestErrorsString(t *testing.T) {

	err := Errors{
		fmt.Errorf("test1"),
		fmt.Errorf("test2"),
		fmt.Errorf("test3"),
	}

	expected := `3 errors occurred:
test1
test2
test3`
	result := err.Error()

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	err = Errors{fmt.Errorf("testx")}
	expected = `1 error occurred: testx`
	result = err.Error()

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	expected = `no error(s)`
	result = Errors{}.Error()
	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

}
