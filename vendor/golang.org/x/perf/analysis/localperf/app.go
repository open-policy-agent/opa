// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Localperf runs an HTTP server for benchmark analysis.
//
// Usage:
//
//     localperf [-addr address] [-storage url] [-base_dir ../appengine]
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/perf/analysis/app"
	"golang.org/x/perf/internal/basedir"
	"golang.org/x/perf/storage"
)

var (
	addr       = flag.String("addr", "localhost:8080", "serve HTTP on `address`")
	storageURL = flag.String("storage", "https://perfdata.golang.org", "storage server base `url`")
	baseDir    = flag.String("base_dir", basedir.Find("golang.org/x/perf/analysis/appengine"), "base `directory` for templates")
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage of localperf:
	localperf [flags]
`)
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetPrefix("localperf: ")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
	}

	if *baseDir == "" {
		log.Print("base_dir is required and could not be automatically found")
		flag.Usage()
	}

	app := &app.App{
		StorageClient: &storage.Client{BaseURL: *storageURL},
		BaseDir:       *baseDir,
	}
	app.RegisterOnMux(http.DefaultServeMux)

	log.Printf("Listening on %s", *addr)

	log.Fatal(http.ListenAndServe(*addr, nil))
}
