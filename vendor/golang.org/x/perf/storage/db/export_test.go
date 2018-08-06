// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package db

import (
	"database/sql"
	"time"
)

func DBSQL(db *DB) *sql.DB {
	return db.sql
}

func SetNow(t time.Time) {
	if t.IsZero() {
		now = time.Now
		return
	}
	now = func() time.Time { return t }
}
