// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package util

import (
	"testing"
	"time"
)

func TestWaitFunc(t *testing.T) {
	trueAfter := func(after time.Duration) func() bool {
		t := time.Now().Add(after)
		return func() bool {
			return time.Now().After(t)
		}
	}

	cases := []struct {
		trueAfter  time.Duration
		interval   time.Duration
		timeout    time.Duration
		shouldFail bool
	}{
		{0, 1 * time.Millisecond, 100 * time.Millisecond, false},
		{1 * time.Millisecond, 1 * time.Millisecond, 100 * time.Millisecond, false},
		{100 * time.Millisecond, 1 * time.Millisecond, 1 * time.Millisecond, true},
		{100 * time.Millisecond, 1000 * time.Millisecond, 1 * time.Millisecond, true},
	}

	for _, c := range cases {
		err := WaitFunc(trueAfter(c.trueAfter), c.interval, c.timeout)
		if err != nil && c.shouldFail {
			continue
		}
		if err != nil {
			t.Error(err)
		} else if c.shouldFail {
			t.Errorf("Expected error for case: %+v", c)
		}
	}
}
