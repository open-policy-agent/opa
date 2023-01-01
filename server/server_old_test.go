//go:build usegorillamux

// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package server

import (
	"github.com/gorilla/mux"
)

func getRouter() *mux.Router {
	return mux.NewRouter()
}
