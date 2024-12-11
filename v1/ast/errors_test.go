// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import "testing"

func TestErrorsString(t *testing.T) {

	err := Errors{
		NewError(ParseErr, nil, "blah"),
		NewError(ParseErr, NewLocation(nil, "", 100, 2), "bleh"),
		NewError(ParseErr, NewLocation(nil, "foo.rego", 100, 2), "blarg"),
	}

	expected := `3 errors occurred:
rego_parse_error: blah
100:2: rego_parse_error: bleh
foo.rego:100: rego_parse_error: blarg`
	result := err.Error()

	if result != expected {
		t.Errorf("Expected %v but got: %v", expected, result)
	}

	err = Errors{NewError(ParseErr, nil, "blah")}
	expected = `1 error occurred: rego_parse_error: blah`
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
