// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"golang.org/x/net/context"
	"golang.org/x/perf/internal/diff"
	"golang.org/x/perf/storage/benchfmt"
)

func TestQueryError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid query", 500)
	}))
	defer ts.Close()

	c := &Client{BaseURL: ts.URL}

	q := c.Query(context.Background(), "invalid query")
	defer q.Close()

	if q.Next() {
		t.Error("Next = true, want false")
	}
	if q.Err() == nil {
		t.Error("Err = nil, want error")
	}
}

func TestQuery(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if have, want := r.URL.RequestURI(), "/search?q=key1%3Avalue+key2%3Avalue"; have != want {
			t.Errorf("RequestURI = %q, want %q", have, want)
		}
		fmt.Fprintf(w, "key: value\nBenchmarkOne 5 ns/op\nkey: value2\nBenchmarkTwo 10 ns/op\n")
	}))
	defer ts.Close()

	c := &Client{BaseURL: ts.URL}

	q := c.Query(context.Background(), "key1:value key2:value")
	defer q.Close()

	var buf bytes.Buffer
	bp := benchfmt.NewPrinter(&buf)

	for q.Next() {
		if err := bp.Print(q.Result()); err != nil {
			t.Fatalf("Print: %v", err)
		}
	}
	if err := q.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	want := "key: value\nBenchmarkOne 5 ns/op\nkey: value2\nBenchmarkTwo 10 ns/op\n"
	if diff := diff.Diff(buf.String(), want); diff != "" {
		t.Errorf("wrong results: (- have/+ want)\n%s", diff)
	}
}

func TestListUploads(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if have, want := r.URL.RequestURI(), "/uploads?extra_label=key1&extra_label=key2&limit=10&q=key1%3Avalue+key2%3Avalue"; have != want {
			t.Errorf("RequestURI = %q, want %q", have, want)
		}
		fmt.Fprintf(w, "%s\n", `{"UploadID": "id", "Count": 100, "LabelValues": {"key1": "value"}}`)
	}))
	defer ts.Close()

	c := &Client{BaseURL: ts.URL}

	r := c.ListUploads(context.Background(), "key1:value key2:value", []string{"key1", "key2"}, 10)
	defer r.Close()

	if !r.Next() {
		t.Errorf("Next = false, want true")
	}
	if have, want := r.Info(), (UploadInfo{Count: 100, UploadID: "id", LabelValues: benchfmt.Labels{"key1": "value"}}); !reflect.DeepEqual(have, want) {
		t.Errorf("Info = %#v, want %#v", have, want)
	}
	if err := r.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
}

func TestNewUpload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if have, want := r.URL.RequestURI(), "/upload"; have != want {
			t.Errorf("RequestURI = %q, want %q", have, want)
		}
		mr, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
		}
		i := 0
		for i = 0; ; i++ {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			name := p.FormName()
			if name == "commit" {
				continue
			}
			if name != "file" {
				t.Errorf("unexpected field %q, want file", name)
			}
			if have, want := p.FileName(), fmt.Sprintf("want%d.txt", i); have != want {
				t.Errorf("file name = %q, want %q", have, want)
			}
			content, _ := ioutil.ReadAll(p)
			if have, want := string(content), "content"; have != want {
				t.Errorf("unexpected content %q, want %q", have, want)
			}
		}
		if i != 3 {
			t.Errorf("number of files = %d, want %d", i, 3)
		}
		fmt.Fprintf(w, "%s\n", `{"uploadid": "id", "fileids": ["id/1", "id/2"]}`)
	}))
	defer ts.Close()

	c := &Client{BaseURL: ts.URL}

	u := c.NewUpload(context.Background())
	for i := 0; i < 2; i++ {
		w, err := u.CreateFile(fmt.Sprintf("want%d.txt", i))
		if err != nil {
			t.Fatalf("CreateFile = %v", err)
		}
		if _, err := fmt.Fprintf(w, "content"); err != nil {
			t.Fatalf("Write returned %v", err)
		}
	}
	status, err := u.Commit()
	if err != nil {
		t.Errorf("Commit = %v", err)
	}
	if status.UploadID != "id" {
		t.Errorf("status.UploadID = %q, want %q", status.UploadID, "id")
	}
}

func TestNewUploadAbort(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if have, want := r.URL.RequestURI(), "/upload"; have != want {
			t.Errorf("RequestURI = %q, want %q", have, want)
		}
		mr, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
		}
		i := 0
		for i = 0; ; i++ {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			name := p.FormName()
			if name == "abort" {
				continue
			}
			if name != "file" {
				t.Errorf("unexpected field %q, want file or abort", name)
			}
			if have, want := p.FileName(), fmt.Sprintf("want%d.txt", i); have != want {
				t.Errorf("file name = %q, want %q", have, want)
			}
			content, _ := ioutil.ReadAll(p)
			if have, want := string(content), "content"; have != want {
				t.Errorf("unexpected content %q, want %q", have, want)
			}
		}
		if i != 3 {
			t.Errorf("number of files = %d, want %d", i, 3)
		}
		fmt.Fprintf(w, "%s\n", `{"uploadid": "id", "fileids": ["id/1", "id/2"]}`)
	}))
	defer ts.Close()

	c := &Client{BaseURL: ts.URL}

	u := c.NewUpload(context.Background())
	for i := 0; i < 2; i++ {
		w, err := u.CreateFile(fmt.Sprintf("want%d.txt", i))
		if err != nil {
			t.Fatalf("CreateFile = %v", err)
		}
		if _, err := fmt.Fprintf(w, "content"); err != nil {
			t.Fatalf("Write returned %v", err)
		}
	}
	if err := u.Abort(); err != nil {
		t.Errorf("Abort = %v", err)
	}
}
