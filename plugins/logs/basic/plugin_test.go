// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package basic

import (
	"context"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/plugins/logs"
)

func TestFactory(t *testing.T) {
	f := DecisionLoggerFactory{}

	rawCfg := []byte("{}")
	cfg, err := f.Validate(nil, rawCfg)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	p := f.New(nil, cfg)
	if p == nil {
		t.Fatalf("Plugin should not be nil")
	}
}

func TestLog(t *testing.T) {
	p := DecisionLogger{}
	entry := logs.EventV1{
		Labels: map[string]string{
			"id":      "test-instance-id",
			"app":     "example-app",
			"version": "1.2.3",
		},
		Revision:    "399",
		DecisionID:  "399",
		Path:        "tda/bar",
		Result:      nil,
		RequestedBy: "test",
		Timestamp:   time.Now(),
		Metrics:     metrics.New().All(),
	}

	err := p.Log(context.Background(), entry)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}
}
