// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !plan9

package app

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"golang.org/x/perf/storage/db"
	"golang.org/x/perf/storage/db/dbtest"
	_ "golang.org/x/perf/storage/db/sqlite3"
	"golang.org/x/perf/storage/fs"
)

type testApp struct {
	db        *db.DB
	dbCleanup func()
	fs        *fs.MemFS
	app       *App
	srv       *httptest.Server
}

func (app *testApp) Close() {
	app.dbCleanup()
	app.srv.Close()
}

// createTestApp returns a testApp corresponding to a new app
// serving from an in-memory database and file system on an
// isolated test HTTP server.
//
// When finished with app, the caller must call app.Close().
func createTestApp(t *testing.T) *testApp {
	db, cleanup := dbtest.NewDB(t)

	fs := fs.NewMemFS()

	app := &App{
		DB:          db,
		FS:          fs,
		Auth:        func(http.ResponseWriter, *http.Request) (string, error) { return "user", nil },
		ViewURLBase: "view:",
	}

	mux := http.NewServeMux()
	app.RegisterOnMux(mux)

	srv := httptest.NewServer(mux)

	return &testApp{db, cleanup, fs, app, srv}
}

// uploadFiles calls the /upload endpoint and executes f in a new
// goroutine to write files to the POST request.
func (app *testApp) uploadFiles(t *testing.T, f func(*multipart.Writer)) *uploadStatus {
	pr, pw := io.Pipe()
	mpw := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer mpw.Close()
		f(mpw)
	}()

	resp, err := http.Post(app.srv.URL+"/upload", mpw.FormDataContentType(), pr)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("post /upload: %v", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading /upload response: %v", err)
	}
	t.Logf("/upload response:\n%s", body)

	status := &uploadStatus{}
	if err := json.Unmarshal(body, status); err != nil {
		t.Fatalf("unmarshaling /upload response: %v", err)
	}
	return status
}

func TestUpload(t *testing.T) {
	app := createTestApp(t)
	defer app.Close()

	wantID := time.Now().UTC().Format("20060102.") + "1"

	status := app.uploadFiles(t, func(mpw *multipart.Writer) {
		w, err := mpw.CreateFormFile("file", "1.txt")
		if err != nil {
			t.Errorf("CreateFormFile: %v", err)
		}
		fmt.Fprintf(w, "key: value\nBenchmarkOne 5 ns/op\nkey:value2\nBenchmarkTwo 10 ns/op\n")
	})

	if status.UploadID != wantID {
		t.Errorf("uploadid = %q, want %q", status.UploadID, wantID)
	}
	if have, want := status.FileIDs, []string{wantID + "/0"}; !reflect.DeepEqual(have, want) {
		t.Errorf("fileids = %v, want %v", have, want)
	}
	if want := "view:" + wantID; status.ViewURL != want {
		t.Errorf("viewurl = %q, want %q", status.ViewURL, want)
	}

	if len(app.fs.Files()) != 1 {
		t.Errorf("/upload wrote %d files, want 1", len(app.fs.Files()))
	}
}
