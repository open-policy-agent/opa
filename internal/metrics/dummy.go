// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package metrics

import (
	"net/http"
)

type dummyProvider struct{}

func (dummyProvider) RegisterEndpoints(registrar func(path, method string, handler http.Handler)) {}

func (dummyProvider) InstrumentHandler(handler http.Handler, label string) http.Handler {
	return handler
}

func (dummyProvider) Gather() (interface{}, error) {
	return nil, nil
}

func (dummyProvider) Name() string {
	return ""
}
