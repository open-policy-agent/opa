// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// NOTE(sr): Different go runtime metrics on 1.16.
// This can be removed when we drop support for go 1.16.

//go:build !go1.17
// +build !go1.17

package prometheus

import "github.com/prometheus/client_golang/prometheus"

func collector() prometheus.Collector {
	return prometheus.NewGoCollector()
}
