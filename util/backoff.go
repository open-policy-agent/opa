// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"time"

	v1 "github.com/open-policy-agent/opa/v1/util"
)

// DefaultBackoff returns a delay with an exponential backoff based on the
// number of retries.
func DefaultBackoff(base, maxNS float64, retries int) time.Duration {
	return v1.DefaultBackoff(base, maxNS, retries)
}

// Backoff returns a delay with an exponential backoff based on the number of
// retries. Same algorithm used in gRPC.
func Backoff(base, maxNS, jitter, factor float64, retries int) time.Duration {
	return v1.Backoff(base, maxNS, jitter, factor, retries)
}
