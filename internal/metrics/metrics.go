// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package metrics

import (
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/open-policy-agent/opa/internal/metrics/prometheus"
	"github.com/open-policy-agent/opa/metrics"
)

// NewGlobalMetrics creates a metrics provider instance given its name and config
func NewGlobalMetrics(name string, config json.RawMessage) (metrics.GlobalMetrics, error) {
	switch name {
	case "":
		return &dummyProvider{}, nil
	case prometheus.ProviderName:
		return prometheus.NewPrometheusProvider(), nil
	default:
		return nil, errors.Errorf("Invalid metrics provider %s.", name)
	}
}
