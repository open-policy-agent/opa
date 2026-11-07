// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build race

package ast

// raceEnabled is whether the tests run under the race detector, which makes
// sync.Pool drop some of what is put back, so that allocation counts of code
// using one do not hold.
const raceEnabled = true
