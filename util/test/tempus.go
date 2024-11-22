// Copyright 2023 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package test

import (
	"testing"
	"time"

	v1 "github.com/open-policy-agent/opa/v1/util/test"
)

func Eventually(t *testing.T, timeout time.Duration, f func() bool) bool {
	t.Helper()
	return v1.Eventually(t, timeout, f)
}

func EventuallyOrFatal(t *testing.T, timeout time.Duration, f func() bool) {
	t.Helper()
	v1.EventuallyOrFatal(t, timeout, f)
}

type BlockingWriter = v1.BlockingWriter
