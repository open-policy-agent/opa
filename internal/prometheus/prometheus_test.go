// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package prometheus_test

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/internal/prometheus"
	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/metrics"
)

func TestJSONSerialization(t *testing.T) {
	inner := metrics.New()
	logger := func(logger logging.Logger) func(attrs map[string]interface{}, f string, a ...interface{}) {
		return func(attrs map[string]interface{}, f string, a ...interface{}) {
			logger.WithFields(map[string]interface{}(attrs)).Error(f, a...)
		}
	}(logging.NewNoOpLogger())

	prom := prometheus.New(inner, logger)

	m := prom.All()
	bs, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(bs))
}
