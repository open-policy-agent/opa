// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Benchsave uploads benchmark results to a storage server.
//
// Usage:
//
//	benchsave [-v] [-header file] [-server url] file...
//
// Each input file should contain the output from one or more runs of
// ``go test -bench'', or another tool which uses the same format.
//
// Benchsave will upload the input files to the specified server and
// print a URL where they can be viewed.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/perf/storage"
)

var (
	server  = flag.String("server", "https://perfdata.golang.org", "upload benchmarks to server at `url`")
	verbose = flag.Bool("v", false, "print verbose log messages")
	header  = flag.String("header", "", "insert `file` at the beginning of each uploaded file")
)

const userAgent = "Benchsave/1.0"

// writeOneFile reads name and writes it to u.
func writeOneFile(u *storage.Upload, name string, header []byte) error {
	w, err := u.CreateFile(filepath.Base(name))
	if err != nil {
		return err
	}
	if len(header) > 0 {
		if _, err := w.Write(header); err != nil {
			return err
		}
	}
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(w, f); err != nil {
		return err
	}
	return nil
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage of benchsave:
	benchsave [flags] file...
`)
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetPrefix("benchsave: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		log.Fatal("no files to upload")
	}

	var headerData []byte
	if *header != "" {
		var err error
		headerData, err = ioutil.ReadFile(*header)
		if err != nil {
			log.Fatal(err)
		}
		headerData = append(bytes.TrimRight(headerData, "\n"), '\n', '\n')
	}

	// TODO(quentin): Some servers might not need authentication.
	// We should somehow detect this and not force the user to get a token.
	// Or they might need non-Google authentication.
	hc := oauth2.NewClient(context.Background(), newTokenSource())

	client := &storage.Client{BaseURL: *server, HTTPClient: hc}

	start := time.Now()

	u := client.NewUpload(context.Background())

	for _, name := range files {
		if err := writeOneFile(u, name, headerData); err != nil {
			log.Print(err)
			u.Abort()
			return
		}
	}

	status, err := u.Commit()
	if err != nil {
		log.Fatalf("upload failed: %v\n", err)
	}

	if *verbose {
		s := ""
		if len(files) != 1 {
			s = "s"
		}
		log.Printf("%d file%s uploaded in %.2f seconds.\n", len(files), s, time.Since(start).Seconds())
	}
	if status.ViewURL != "" {
		// New servers will serve a text/plain response to the view URL when given these headers.
		// Old servers will not, so only show the response if it is a 200 and text/plain.
		req, err := http.NewRequest("GET", status.ViewURL, nil)
		if err == nil {
			req.Header.Set("User-Agent", userAgent)
			req.Header.Set("Accept", "text/plain")
			req.Header.Set("X-Benchsave", "1")
			resp, err := hc.Do(req)
			if err == nil {
				defer resp.Body.Close()
				mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
				if resp.StatusCode == http.StatusOK && err == nil && mt == "text/plain" {
					io.Copy(os.Stdout, resp.Body)
					fmt.Println()
				}
			}
		}
		fmt.Printf("%s\n", status.ViewURL)
	}
}
