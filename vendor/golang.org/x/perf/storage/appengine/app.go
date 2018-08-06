// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build appengine

// Package appengine contains an AppEngine app for perfdata.golang.org
package appengine

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/net/context"
	"golang.org/x/perf/storage/app"
	"golang.org/x/perf/storage/db"
	"golang.org/x/perf/storage/fs/gcs"
	oauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/appengine"
	aelog "google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
	"google.golang.org/appengine/user"
)

// connectDB returns a DB initialized from the environment variables set in app.yaml. CLOUDSQL_CONNECTION_NAME, CLOUDSQL_USER, and CLOUDSQL_DATABASE must be set to point to the Cloud SQL instance. CLOUDSQL_PASSWORD can be set if needed.
func connectDB() (*db.DB, error) {
	var (
		connectionName = mustGetenv("CLOUDSQL_CONNECTION_NAME")
		user           = mustGetenv("CLOUDSQL_USER")
		password       = os.Getenv("CLOUDSQL_PASSWORD") // NOTE: password may be empty
		dbName         = mustGetenv("CLOUDSQL_DATABASE")
	)

	return db.OpenSQL("mysql", fmt.Sprintf("%s:%s@cloudsql(%s)/%s?interpolateParams=true", user, password, connectionName, dbName))
}

func mustGetenv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Panicf("%s environment variable not set.", k)
	}
	return v
}

func auth(w http.ResponseWriter, r *http.Request) (string, error) {
	ctx := appengine.NewContext(r)
	u, err := reqUser(ctx, r)
	if err != nil {
		return "", err
	}
	if u == "" {
		url, err := user.LoginURL(ctx, r.URL.String())
		if err != nil {
			return "", err
		}
		http.Redirect(w, r, url, http.StatusFound)
		return "", app.ErrResponseWritten
	}
	return u, nil
}

// reqUser gets the username from the request, trying AE user authentication, AE OAuth authentication, and Google OAuth authentication, in that order.
// If the request contains no authentication, "", nil is returned.
// If the request contains bogus authentication, an error is returned.
func reqUser(ctx context.Context, r *http.Request) (string, error) {
	u := user.Current(ctx)
	if u != nil {
		return u.Email, nil
	}
	if r.Header.Get("Authorization") == "" {
		return "", nil
	}
	u, err := user.CurrentOAuth(ctx, "https://www.googleapis.com/auth/userinfo.email")
	if err == nil {
		return u.Email, nil
	}
	return oauthServiceUser(ctx, r)
}

// oauthServiceUser authenticates the OAuth token from r's headers.
// This is necessary because user.CurrentOAuth does not work if the token is for a service account.
func oauthServiceUser(ctx context.Context, r *http.Request) (string, error) {
	tok := r.Header.Get("Authorization")
	if !strings.HasPrefix(tok, "Bearer ") {
		return "", errors.New("unknown Authorization header")
	}
	tok = tok[len("Bearer "):]

	hc := urlfetch.Client(ctx)
	service, err := oauth2.New(hc)
	if err != nil {
		return "", err
	}
	info, err := service.Tokeninfo().AccessToken(tok).Do()
	if err != nil {
		return "", err
	}

	if !info.VerifiedEmail || info.Email == "" {
		return "", errors.New("token does not contain verified e-mail address")
	}
	return info.Email, nil
}

// appHandler is the default handler, registered to serve "/".
// It creates a new App instance using the appengine Context and then
// dispatches the request to the App. The environment variable
// GCS_BUCKET must be set in app.yaml with the name of the bucket to
// write to. PERFDATA_VIEW_URL_BASE may be set to the URL that should
// be supplied in /upload responses.
func appHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	// App Engine does not return a context with a deadline set,
	// even though the request does have a deadline. urlfetch uses
	// a 5s default timeout if the context does not have a
	// deadline, so explicitly set a deadline to match the App
	// Engine timeout.
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	// GCS clients need to be constructed with an AppEngine
	// context, so we can't actually make the App until the
	// request comes in.
	// TODO(quentin): Figure out if there's a way to construct the
	// app and clients once, in init(), instead of on every request.
	db, err := connectDB()
	if err != nil {
		aelog.Errorf(ctx, "connectDB: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	defer db.Close()

	fs, err := gcs.NewFS(ctx, mustGetenv("GCS_BUCKET"))
	if err != nil {
		aelog.Errorf(ctx, "gcs.NewFS: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	mux := http.NewServeMux()
	app := &app.App{DB: db, FS: fs, Auth: auth, ViewURLBase: os.Getenv("PERFDATA_VIEW_URL_BASE")}
	app.RegisterOnMux(mux)
	mux.ServeHTTP(w, r)
}

func init() {
	http.HandleFunc("/", appHandler)
}
