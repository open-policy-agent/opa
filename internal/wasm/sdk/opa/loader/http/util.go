// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package http

import (
	"math/rand"
	"time"
)

// defaultBackoff returns a delay with an exponential backoff based on the
// number of retries.
func defaultBackoff(base, max float64, retries int) time.Duration {
	return backoff(base, max, .2, 1.6, retries)
}

// backoff returns a delay with an exponential backoff based on the number of
// retries. Same algorithm used in gRPC.
func backoff(base, max, jitter, factor float64, retries int) time.Duration {
	if retries == 0 {
		return 0
	}

	//nolint:unconvert
	backoff, max := float64(base), float64(max)
	for backoff < max && retries > 0 {
		backoff *= factor
		retries--
	}
	if backoff > max {
		backoff = max
	}

	// Randomize backoff delays so that if a cluster of requests start at
	// the same time, they won't operate in lockstep.
	backoff *= 1 + jitter*(rand.Float64()*2-1)
	if backoff < 0 {
		return 0
	}

	return time.Duration(backoff)
}
