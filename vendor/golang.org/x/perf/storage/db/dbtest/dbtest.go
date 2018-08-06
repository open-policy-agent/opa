// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !plan9

package dbtest

import (
	"testing"

	"golang.org/x/perf/storage/db"
	_ "golang.org/x/perf/storage/db/sqlite3"
)

// NewDB makes a connection to a testing database, either sqlite3 or
// Cloud SQL depending on the -cloud flag. cleanup must be called when
// done with the testing database, instead of calling db.Close()
func NewDB(t *testing.T) (*db.DB, func()) {
	driverName, dataSourceName, cloudCleanup := createEmptyDB(t)
	d, err := db.OpenSQL(driverName, dataSourceName)
	if err != nil {
		if cloudCleanup != nil {
			cloudCleanup()
		}
		t.Fatalf("open database: %v", err)
	}

	cleanup := func() {
		if cloudCleanup != nil {
			cloudCleanup()
		}
		d.Close()
	}
	// Make sure the database really is empty.
	uploads, err := d.CountUploads()
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	if uploads != 0 {
		cleanup()
		t.Fatalf("found %d row(s) in Uploads, want 0", uploads)
	}
	return d, cleanup
}
