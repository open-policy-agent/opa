// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package app implements the performance data analysis server.
package app

import (
	"net/http"

	"golang.org/x/perf/storage"
)

// App manages the analysis server logic.
// Construct an App instance and call RegisterOnMux to connect it with an HTTP server.
type App struct {
	// StorageClient is used to talk to the storage server.
	StorageClient *storage.Client

	// BaseDir is the directory containing the "template" directory.
	// If empty, the current directory will be used.
	BaseDir string
}

// RegisterOnMux registers the app's URLs on mux.
func (a *App) RegisterOnMux(mux *http.ServeMux) {
	mux.HandleFunc("/", a.index)
	mux.HandleFunc("/search", a.search)
	mux.HandleFunc("/compare", a.compare)
	mux.HandleFunc("/trend", a.trend)
}

// search handles /search.
// This currently just runs the compare handler, until more analysis methods are implemented.
func (a *App) search(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if r.Header.Get("Accept") == "text/plain" || r.Header.Get("X-Benchsave") == "1" {
		// TODO(quentin): Switch to real Accept negotiation when golang/go#19307 is resolved.
		// Benchsave sends both of these headers.
		a.textCompare(w, r)
		return
	}
	// TODO(quentin): Intelligently choose an analysis method
	// based on the results from the query, once there is more
	// than one analysis method.
	//q := r.Form.Get("q")
	a.compare(w, r)
}
