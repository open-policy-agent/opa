// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !plan9

// Package sqlite3 provides the sqlite3 driver for
// x/perf/storage/db. It must be imported instead of go-sqlite3 to
// ensure foreign keys are properly honored.
package sqlite3

import (
	"database/sql"

	sqlite3 "github.com/mattn/go-sqlite3"
	"golang.org/x/perf/storage/db"
)

func init() {
	db.RegisterOpenHook("sqlite3", func(db *sql.DB) error {
		db.Driver().(*sqlite3.SQLiteDriver).ConnectHook = func(c *sqlite3.SQLiteConn) error {
			_, err := c.Exec("PRAGMA foreign_keys = ON;", nil)
			return err
		}
		return nil
	})
}
