// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Reindex repopulates the perfdata SQL database from the original data files in Google Cloud Storage.
//
// Usage:
//
//	reindex [-v] [-db foo.bar/baz] [-bucket name] prefix...
//
// Reindex reindexes all the uploads with IDs starting with the given prefixes.

// +build cloud

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"cloud.google.com/go/storage"
	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/mysql"
	"golang.org/x/perf/storage/benchfmt"
	"golang.org/x/perf/storage/db"
	"google.golang.org/api/iterator"
)

var (
	dbName  = flag.String("db", "root:@cloudsql(golang-org:us-central1:golang-org)/perfdata?interpolateParams=true", "connect to MySQL `database`")
	bucket  = flag.String("bucket", "golang-perfdata", "read from Google Cloud Storage `bucket`")
	verbose = flag.Bool("v", false, "verbose")
)

func usage() {
	fmt.Fprintf(os.Stderr, `Usage of reindex:
	reindex [flags] prefix...
`)
	flag.PrintDefaults()
	os.Exit(2)
}

func main() {
	log.SetPrefix("reindex: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	if *verbose {
		log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	}

	ctx := context.Background()

	prefixes := flag.Args()
	if len(prefixes) == 0 {
		log.Fatal("no prefixes to reindex")
	}

	d, err := db.OpenSQL("mysql", *dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	bucket := client.Bucket(*bucket)

	for _, prefix := range prefixes {
		if strings.Index(prefix, "/") >= 0 {
			log.Fatalf("prefix %q cannot contain /", prefix)
		}
		it := bucket.Objects(ctx, &storage.Query{Prefix: "uploads/" + prefix})
		var lastUploadId string
		var files []string
		for {
			objAttrs, err := it.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Fatal(err)
			}
			name := strings.TrimPrefix(objAttrs.Name, "uploads/")
			slash := strings.Index(name, "/")
			if slash < 0 {
				log.Printf("ignoring file %q", objAttrs.Name)
			}
			uploadID := name[:slash]
			if lastUploadId != "" && uploadID != lastUploadId {
				if err := reindex(ctx, d, bucket, lastUploadId, files); err != nil {
					log.Fatal(err)
				}
				files = nil
			}
			files = append(files, objAttrs.Name)
			lastUploadId = uploadID
		}
		if len(files) > 0 {
			if err := reindex(ctx, d, bucket, lastUploadId, files); err != nil {
				log.Fatal(err)
			}
		}
	}
}

func reindex(ctx context.Context, db *db.DB, bucket *storage.BucketHandle, uploadID string, files []string) error {
	if *verbose {
		log.Printf("reindexing %q", uploadID)
	}
	u, err := db.ReplaceUpload(uploadID)
	if err != nil {
		return err
	}
	for _, name := range files {
		if err := reindexOne(ctx, u, bucket, name); err != nil {
			return err
		}
	}
	return u.Commit()
}

func reindexOne(ctx context.Context, u *db.Upload, bucket *storage.BucketHandle, name string) error {
	r, err := bucket.Object(name).NewReader(ctx)
	if err != nil {
		return err
	}
	defer r.Close()
	br := benchfmt.NewReader(r)
	for br.Next() {
		if err := u.InsertRecord(br.Result()); err != nil {
			return err
		}
	}
	if err := br.Err(); err != nil {
		return err
	}
	return nil
}
