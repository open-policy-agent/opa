// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build appengine

// Package appengine contains an AppEngine app for perf.golang.org
package appengine

import (
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/perf/analysis/app"
	"golang.org/x/perf/storage"
	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
)

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Panicf("%s environment variable not set.", k)
	}
	return v
}

// appHandler is the default handler, registered to serve "/".
// It creates a new App instance using the appengine Context and then
// dispatches the request to the App. The environment variable
// STORAGE_URL_BASE must be set in app.yaml with the name of the bucket to
// write to.
func appHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	// urlfetch defaults to 5s timeout if the context has no timeout.
	// The underlying request has a 60 second timeout, so we might as well propagate that here.
	// (Why doesn't appengine do that for us?)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	app := &app.App{
		StorageClient: &storage.Client{
			BaseURL:    mustGetenv("STORAGE_URL_BASE"),
			HTTPClient: urlfetch.Client(ctx),
		},
	}
	mux := http.NewServeMux()
	app.RegisterOnMux(mux)
	mux.ServeHTTP(w, r)
}

func init() {
	http.HandleFunc("/", appHandler)
}
