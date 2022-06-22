// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package glog is used as a replacement for github.com/golang/glog. The latter
// is used only through badger (ristretto), and comes with costly current user
// lookups that we don't need: it's only used for printing fatal errors before
// giving up.
package glog

import "log"

// Fatal outputs its arguments and does an os.Exit(1)
func Fatal(args ...interface{}) {
	log.Fatal(args...)
}

// Fatalf formats and outputs its arguments and does an os.Exit(1)
func Fatalf(format string, args ...interface{}) {
	log.Fatal(append([]interface{}{format}, args...))
}
