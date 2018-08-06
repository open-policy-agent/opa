// Copyright 2017 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build appengine

package app

import (
	"net/http"

	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
)

// requestContext returns the Context object for a given request.
func requestContext(r *http.Request) context.Context {
	return appengine.NewContext(r)
}

var infof = log.Infof
var errorf = log.Errorf
