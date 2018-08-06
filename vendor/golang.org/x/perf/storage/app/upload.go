// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/perf/storage/benchfmt"
	"golang.org/x/perf/storage/db"
)

// upload is the handler for the /upload endpoint. It serves a form on
// GET requests and processes files in a multipart/x-form-data POST
// request.
func (a *App) upload(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)

	user, err := a.Auth(w, r)
	switch {
	case err == ErrResponseWritten:
		return
	case err != nil:
		errorf(ctx, "%v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	if r.Method == http.MethodGet {
		http.ServeFile(w, r, filepath.Join(a.BaseDir, "static/upload.html"))
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "/upload must be called as a POST request", http.StatusMethodNotAllowed)
		return
	}

	// We use r.MultipartReader instead of r.ParseForm to avoid
	// storing uploaded data in memory.
	mr, err := r.MultipartReader()
	if err != nil {
		errorf(ctx, "%v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	result, err := a.processUpload(ctx, user, mr)
	if err != nil {
		errorf(ctx, "%v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		errorf(ctx, "%v", err)
		http.Error(w, err.Error(), 500)
		return
	}
}

// uploadStatus is the response to an /upload POST served as JSON.
type uploadStatus struct {
	// UploadID is the upload ID assigned to the upload.
	UploadID string `json:"uploadid"`
	// FileIDs is the list of file IDs assigned to the files in the upload.
	FileIDs []string `json:"fileids"`
	// ViewURL is a URL that can be used to interactively view the upload.
	ViewURL string `json:"viewurl,omitempty"`
}

// processUpload takes one or more files from a multipart.Reader,
// writes them to the filesystem, and indexes their content.
func (a *App) processUpload(ctx context.Context, user string, mr *multipart.Reader) (*uploadStatus, error) {
	var upload *db.Upload
	var fileids []string

	uploadtime := time.Now().UTC().Format(time.RFC3339)

	for i := 0; ; i++ {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		name := p.FormName()
		if name == "commit" {
			continue
		}
		if name != "file" {
			return nil, fmt.Errorf("unexpected field %q", name)
		}

		if upload == nil {
			var err error
			upload, err = a.DB.NewUpload(ctx)
			if err != nil {
				return nil, err
			}
			defer func() {
				if upload != nil {
					upload.Abort()
				}
			}()
		}

		// The incoming file needs to be stored in Cloud
		// Storage and it also needs to be indexed. If the file
		// is invalid (contains no valid records) it needs to
		// be rejected and the Cloud Storage upload aborted.

		meta := map[string]string{
			"upload":      upload.ID,
			"upload-part": fmt.Sprintf("%s/%d", upload.ID, i),
			"upload-time": uploadtime,
		}
		name = p.FileName()
		if slash := strings.LastIndexAny(name, `/\`); slash >= 0 {
			name = name[slash+1:]
		}
		if name != "" {
			meta["upload-file"] = name
		}
		if user != "" {
			meta["by"] = user
		}

		// We need to do two things with the incoming data:
		// - Write it to permanent storage via a.FS
		// - Write index records to a.DB
		// AND if anything fails, attempt to clean up both the
		// FS and the index records.

		if err := a.indexFile(ctx, upload, p, meta); err != nil {
			return nil, err
		}

		fileids = append(fileids, meta["upload-part"])
	}

	if upload == nil {
		return nil, errors.New("no files processed")
	}
	if err := upload.Commit(); err != nil {
		return nil, err
	}

	status := &uploadStatus{UploadID: upload.ID, FileIDs: fileids}
	if a.ViewURLBase != "" {
		status.ViewURL = a.ViewURLBase + url.QueryEscape(upload.ID)
	}

	upload = nil

	return status, nil
}

func (a *App) indexFile(ctx context.Context, upload *db.Upload, p io.Reader, meta map[string]string) (err error) {
	path := fmt.Sprintf("uploads/%s.txt", meta["upload-part"])
	fw, err := a.FS.NewWriter(ctx, path, meta)
	if err != nil {
		return err
	}
	defer func() {
		start := time.Now()
		if err != nil {
			fw.CloseWithError(err)
		} else {
			err = fw.Close()
		}
		infof(ctx, "Close(%q) took %.2f seconds", path, time.Since(start).Seconds())
	}()
	var keys []string
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(fw, "%s: %s\n", k, meta[k]); err != nil {
			return err
		}
	}
	// Write a blank line to separate metadata from user-generated content.
	fmt.Fprintf(fw, "\n")

	// TODO(quentin): Add a separate goroutine and buffer for writes to fw?
	tr := io.TeeReader(p, fw)
	br := benchfmt.NewReader(tr)
	br.AddLabels(meta)
	i := 0
	for br.Next() {
		i++
		if err := upload.InsertRecord(br.Result()); err != nil {
			return err
		}
	}
	if err := br.Err(); err != nil {
		return err
	}
	if i == 0 {
		return errors.New("no valid benchmark lines found")
	}
	return nil
}
