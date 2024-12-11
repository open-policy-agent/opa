// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"net/http"

	v1 "github.com/open-policy-agent/opa/v1/util"
)

// Close reads the remaining bytes from the response and then closes it to
// ensure that the connection is freed. If the body is not read and closed, a
// leak can occur.
func Close(resp *http.Response) {
	v1.Close(resp)
}
