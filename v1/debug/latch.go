// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import "sync"

type latch struct {
	paused    bool
	waitGroup sync.WaitGroup
	lock      sync.Mutex
}

func (l *latch) block() {
	l.lock.Lock()
	defer l.lock.Unlock()
	if !l.paused {
		l.waitGroup.Add(1)
		l.paused = true
	}
}

func (l *latch) unblock() {
	l.lock.Lock()
	defer l.lock.Unlock()
	if l.paused {
		l.waitGroup.Done()
		l.paused = false
	}
}

func (l *latch) wait() {
	l.waitGroup.Wait()
}

func (*latch) Close() {
}
