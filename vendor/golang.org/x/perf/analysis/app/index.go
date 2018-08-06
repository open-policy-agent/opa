// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"

	"golang.org/x/perf/storage"
)

// index redirects / to /search.
func (a *App) index(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)

	tmpl, err := ioutil.ReadFile(filepath.Join(a.BaseDir, "template/index.html"))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	t, err := template.New("main").Parse(string(tmpl))
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var uploads []storage.UploadInfo
	ul := a.StorageClient.ListUploads(ctx, "", []string{"by", "upload-time"}, 16)
	defer ul.Close()
	for ul.Next() {
		uploads = append(uploads, ul.Info())
	}
	if err := ul.Err(); err != nil {
		errorf(ctx, "failed to fetch recent uploads: %v", err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, struct{ RecentUploads []storage.UploadInfo }{uploads}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}
