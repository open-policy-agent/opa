// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package metrics

import (
	"net/http"
)

// GlobalMetrics abstracts metric providers API
type GlobalMetrics interface {
	RegisterEndpoints(registrar func(path, method string, handler http.Handler))
	InstrumentHandler(handler http.Handler, label string) http.Handler
	Gather() (interface{}, error)
	Name() string
}
