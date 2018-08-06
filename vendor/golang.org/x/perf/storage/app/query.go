// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package app

import (
	"encoding/json"
	"net/http"
	"strconv"

	"golang.org/x/perf/storage/benchfmt"
)

func (a *App) search(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	q := r.Form.Get("q")
	if q == "" {
		http.Error(w, "missing q parameter", 400)
		return
	}

	query := a.DB.Query(q)
	defer query.Close()

	infof(ctx, "query: %s", query.Debug())

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	bw := benchfmt.NewPrinter(w)
	for query.Next() {
		if err := bw.Print(query.Result()); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	if err := query.Err(); err != nil {
		errorf(ctx, "query returned error: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
}

// uploads serves a list of upload IDs on /uploads.
// If the query parameter q is provided, only uploads containing matching records are returned.
// The format of the result is "<count> <uploadid>\n" where count is the number of matching records.
// The lines are sorted in order from most to least recent.
// If the query parameter limit is provided, only the most recent limit upload IDs are returned.
// If limit is not provided, the most recent 1000 upload IDs are returned.
func (a *App) uploads(w http.ResponseWriter, r *http.Request) {
	ctx := requestContext(r)

	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	q := r.Form.Get("q")

	limit := 1000
	limitStr := r.Form.Get("limit")
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, "invalid limit parameter", 400)
			return
		}
	}

	res := a.DB.ListUploads(q, r.Form["extra_label"], limit)
	defer res.Close()

	infof(ctx, "query: %s", res.Debug())

	w.Header().Set("Content-Type", "application/json")
	e := json.NewEncoder(w)
	for res.Next() {
		ui := res.Info()
		if err := e.Encode(&ui); err != nil {
			errorf(ctx, "failed to encode JSON: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
	}
	if err := res.Err(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}
