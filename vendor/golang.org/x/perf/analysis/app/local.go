// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !appengine

package app

import (
	"log"
	"net/http"

	"golang.org/x/net/context"
)

// requestContext returns the Context object for a given request.
func requestContext(r *http.Request) context.Context {
	return r.Context()
}

func infof(_ context.Context, format string, args ...interface{}) {
	log.Printf(format, args...)
}

var errorf = infof
