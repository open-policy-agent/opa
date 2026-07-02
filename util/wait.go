// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"time"

	v1 "github.com/open-policy-agent/opa/v1/util"
)

// WaitFunc will call passed function at an interval and return nil
// as soon this function returns true.
// If timeout is reached before the passed in function returns true
// an error is returned.
func WaitFunc(fun func() bool, interval, timeout time.Duration) error {
	return v1.WaitFunc(fun, interval, timeout)
}
