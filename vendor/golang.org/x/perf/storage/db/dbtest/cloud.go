// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build cloud,!plan9

package dbtest

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"flag"
	"fmt"
	"testing"

	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql"
)

var cloud = flag.Bool("cloud", false, "connect to Cloud SQL database instead of in-memory SQLite")
var cloudsql = flag.String("cloudsql", "golang-org:us-central1:golang-org", "name of Cloud SQL instance to run tests on")

// createEmptyDB makes a new, empty database for the test.
func createEmptyDB(t *testing.T) (driver, dsn string, cleanup func()) {
	if !*cloud {
		return "sqlite3", ":memory:", nil
	}
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}

	name := "perfdata-test-" + base64.RawURLEncoding.EncodeToString(buf)

	prefix := fmt.Sprintf("root:@cloudsql(%s)/", *cloudsql)

	db, err := sql.Open("mysql", prefix)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(fmt.Sprintf("CREATE DATABASE `%s`", name)); err != nil {
		db.Close()
		t.Fatal(err)
	}

	t.Logf("Using database %q", name)

	return "mysql", prefix + name + "?interpolateParams=true", func() {
		if _, err := db.Exec(fmt.Sprintf("DROP DATABASE `%s`", name)); err != nil {
			t.Error(err)
		}
		db.Close()
	}
}
