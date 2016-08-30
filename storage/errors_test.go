// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import "testing"

func TestIsNotFound(t *testing.T) {
	err1 := &Error{
		Code:    NotFoundErr,
		Message: "",
	}

	err2 := &Error{
		Code:    InternalErr,
		Message: "",
	}

	if !IsNotFound(err1) {
		t.Errorf("Expected err1 to be not found error")
	}

	if IsNotFound(err2) {
		t.Errorf("Did not expect err2 to be not found error")
	}
}
