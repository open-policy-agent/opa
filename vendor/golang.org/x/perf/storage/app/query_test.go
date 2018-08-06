// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !plan9

package app

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"golang.org/x/perf/storage"
	"golang.org/x/perf/storage/benchfmt"
)

func TestQuery(t *testing.T) {
	app := createTestApp(t)
	defer app.Close()

	// Write 1024 test results to the database.  These results
	// have labels named label0, label1, etc. Each label's value
	// is an integer whose value is (record number) / (1 << label
	// number).  So 1 record has each value of label0, 2 records
	// have each value of label1, 4 records have each value of
	// label2, etc. This allows writing queries that match 2^n records.
	status := app.uploadFiles(t, func(mpw *multipart.Writer) {
		w, err := mpw.CreateFormFile("file", "path/1.txt")
		if err != nil {
			t.Errorf("CreateFormFile: %v", err)
		}
		bp := benchfmt.NewPrinter(w)
		for i := 0; i < 1024; i++ {
			r := &benchfmt.Result{Labels: make(map[string]string), NameLabels: make(map[string]string), Content: "BenchmarkName 1 ns/op"}
			for j := uint(0); j < 10; j++ {
				r.Labels[fmt.Sprintf("label%d", j)] = fmt.Sprintf("%d", i/(1<<j))
			}
			r.NameLabels["name"] = "Name"
			if err := bp.Print(r); err != nil {
				t.Fatalf("Print: %v", err)
			}
		}
	})

	tests := []struct {
		q    string
		want []int
	}{
		{"label0:0", []int{0}},
		{"label1:0", []int{0, 1}},
		{"label0:5 name:Name", []int{5}},
		{"label0:0 label0:5", nil},
	}
	for _, test := range tests {
		t.Run("query="+test.q, func(t *testing.T) {
			u := app.srv.URL + "/search?" + url.Values{"q": []string{test.q}}.Encode()
			resp, err := http.Get(u)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("get /search: %v", resp.Status)
			}
			br := benchfmt.NewReader(resp.Body)
			for i, num := range test.want {
				if !br.Next() {
					t.Fatalf("#%d: Next() = false, want true (Err() = %v)", i, br.Err())
				}
				r := br.Result()
				if r.Labels["upload"] != status.UploadID {
					t.Errorf("#%d: upload = %q, want %q", i, r.Labels["upload"], status.UploadID)
				}
				if r.Labels["upload-part"] != status.FileIDs[0] {
					t.Errorf("#%d: upload-part = %q, want %q", i, r.Labels["upload-part"], status.FileIDs[0])
				}
				if r.Labels["upload-file"] != "1.txt" {
					t.Errorf("#%d: upload-file = %q, want %q", i, r.Labels["upload-file"], "1.txt")
				}
				if r.Labels["label0"] != fmt.Sprintf("%d", num) {
					t.Errorf("#%d: label0 = %q, want %d", i, r.Labels["label0"], num)
				}
				if r.NameLabels["name"] != "Name" {
					t.Errorf("#%d: name = %q, want %q", i, r.NameLabels["name"], "Name")
				}
				if r.Labels["by"] != "user" {
					t.Errorf("#%d: by = %q, want %q", i, r.Labels["uploader"], "user")
				}
			}
			if br.Next() {
				t.Fatalf("Next() = true, want false")
			}
			if err := br.Err(); err != nil {
				t.Errorf("Err() = %v, want nil", err)
			}
		})
	}
}

func TestUploads(t *testing.T) {
	app := createTestApp(t)
	defer app.Close()

	// Write 9 uploads to the database. These uploads have 1-9
	// results each, a common label "i" set to the upload number,
	// and a label "j" set to the record number within the upload.
	var uploadIDs []string
	for i := 0; i < 9; i++ {
		status := app.uploadFiles(t, func(mpw *multipart.Writer) {
			w, err := mpw.CreateFormFile("file", "path/1.txt")
			if err != nil {
				t.Errorf("CreateFormFile: %v", err)
			}
			bp := benchfmt.NewPrinter(w)
			for j := 0; j <= i; j++ {
				r := &benchfmt.Result{Labels: map[string]string{"i": fmt.Sprintf("%d", i)}, NameLabels: make(map[string]string), Content: "BenchmarkName 1 ns/op"}
				r.Labels["j"] = fmt.Sprintf("%d", j)
				if err := bp.Print(r); err != nil {
					t.Fatalf("Print: %v", err)
				}
			}
		})
		uploadIDs = append(uploadIDs, status.UploadID)
	}

	tests := []struct {
		q           string
		extraLabels []string
		want        []storage.UploadInfo
	}{
		{"", nil, []storage.UploadInfo{
			{9, uploadIDs[8], nil}, {8, uploadIDs[7], nil}, {7, uploadIDs[6], nil}, {6, uploadIDs[5], nil}, {5, uploadIDs[4], nil}, {4, uploadIDs[3], nil}, {3, uploadIDs[2], nil}, {2, uploadIDs[1], nil}, {1, uploadIDs[0], nil},
		}},
		{"j:5", nil, []storage.UploadInfo{{1, uploadIDs[8], nil}, {1, uploadIDs[7], nil}, {1, uploadIDs[6], nil}, {1, uploadIDs[5], nil}}},
		{"i:5", []string{"i"}, []storage.UploadInfo{{6, uploadIDs[5], benchfmt.Labels{"i": "5"}}}},
		{"not:found", nil, nil},
	}
	for _, test := range tests {
		t.Run("query="+test.q, func(t *testing.T) {
			u := app.srv.URL + "/uploads"
			uv := url.Values{}
			if test.q != "" {
				uv["q"] = []string{test.q}
			}
			if test.extraLabels != nil {
				uv["extra_label"] = test.extraLabels
			}
			if len(uv) > 0 {
				u += "?" + uv.Encode()
			}

			resp, err := http.Get(u)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("get /uploads: %v", resp.Status)
			}
			dec := json.NewDecoder(resp.Body)
			i := 0
			for {
				var ui storage.UploadInfo
				if err := dec.Decode(&ui); err == io.EOF {
					break
				} else if err != nil {
					t.Fatalf("failed to parse UploadInfo: %v", err)
				}
				if i > len(test.want) {
					t.Fatalf("too many responses: have %d+ want %d", i, len(test.want))
				}
				if !reflect.DeepEqual(ui, test.want[i]) {
					t.Errorf("uploadinfo = %#v, want %#v", ui, test.want[i])
				}
				i++
			}
			if i < len(test.want) {
				t.Fatalf("missing responses: have %d want %d", i, len(test.want))
			}
		})
	}
}
