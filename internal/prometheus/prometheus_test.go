// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package prometheus

import (
	"encoding/json"
	"testing"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/metrics"
)

func TestJSONSerialization(t *testing.T) {
	inner := metrics.New()
	logger := func(logger logging.Logger) loggerFunc {
		return func(attrs map[string]interface{}, f string, a ...interface{}) {
			logger.WithFields(map[string]interface{}(attrs)).Error(f, a...)
		}
	}(logging.NewNoOpLogger())

	prom := New(inner, logger)

	m := prom.All()
	bs, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(bs))
}
