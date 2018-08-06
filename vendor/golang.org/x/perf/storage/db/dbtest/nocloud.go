// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !cloud,!plan9

package dbtest

import (
	"testing"
)

// createEmptyDB makes a new, empty database for the test.
func createEmptyDB(t *testing.T) (driver, dsn string, cleanup func()) {
	return "sqlite3", ":memory:", nil
}
