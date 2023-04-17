// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package discovery

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/keys"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		input    string
		services []string
		wantErr  bool
	}{
		{
			input:    `{}`,
			services: []string{"service1"},
			wantErr:  true,
		},
		{
			input:    `{"name": "a/b/c", "service": "service1"}`,
			services: []string{"service2"},
			wantErr:  true,
		},
		{
			input:    `{"name": "a/b/c", "service": "service1"}`,
			services: []string{"service1", "service2"},
			wantErr:  false,
		},
		{
			input:    `{"name": "a/b/c"}`,
			services: []string{"service1", "service2"},
			wantErr:  true,
		},
		{
			input:    `{"name": "a/b/c"}`,
			services: []string{},
			wantErr:  true,
		},
		{
			input:    `{"name": "a/b/c"}`,
			services: []string{"service1"},
			wantErr:  false,
		},
		{
			input:    `{"name": "a/b/c", "prefix": "dummy", "decision": "query"}`,
			services: []string{"service1"},
			wantErr:  false,
		},
		{
			input:    `{"name": "a/b/c", "decision": "query", "signing": {"keyid": "foo", "scope": "write"}}}`,
			services: []string{"s1"},
			wantErr:  false,
		},
		{
			input:    `{"name": "a/b/c", "decision": "query", "signing": {"keyid": "bar", "scope": "write"}}}`,
			services: []string{"s1"},
			wantErr:  true,
		},
	}

	keys := map[string]*keys.Config{"foo": {Key: "secret"}}
	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConfigValidation_case_%d", i), func(t *testing.T) {
			_, err := NewConfigBuilder().WithBytes([]byte(test.input)).WithServices(test.services).WithKeyConfigs(keys).Parse()
			if err != nil && !test.wantErr {
				t.Fatalf("unexpected error while parsing config: %s", err.Error())
			}
			if err == nil && test.wantErr {
				t.Fatal("expected error while parsing config, but got none")
			}
		})
	}
}

func TestConfigDecision(t *testing.T) {
	tests := []struct {
		input    string
		decision string
	}{
		{
			input:    `{"name": "a/b/c", "decision": "query"}`,
			decision: "data.query",
		},
		{
			input:    `{"name": "a/b/c"}`,
			decision: "data.a.b.c",
		},
		{
			input:    `{"resource": "discovery.tar.gz"}`,
			decision: `data`,
		},
		{
			input:    `{"resource": "discovery.tar.gz", "decision": "foo/bar"}`,
			decision: "data.foo.bar",
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConfigDecision_case_%d", i), func(t *testing.T) {
			c, err := NewConfigBuilder().WithBytes([]byte(test.input)).WithServices([]string{"service1"}).Parse()
			if err != nil {
				t.Fatal("unexpected error while parsing config")
			}

			if c.query != test.decision {
				t.Fail()
			}
		})
	}
}

func TestConfigService(t *testing.T) {
	tests := []struct {
		input    string
		services []string
		service  string
	}{
		{
			input:    `{"name": "a/b/c"}`,
			services: []string{"service1"},
			service:  "service1",
		},
		{
			input:    `{"name": "a/b/c", "service": "service1"}`,
			services: []string{"service1", "service2"},
			service:  "service1",
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConfigService_case_%d", i), func(t *testing.T) {
			c, err := NewConfigBuilder().WithBytes([]byte(test.input)).WithServices(test.services).Parse()
			if err != nil {
				t.Fatalf("unexpected error while parsing config: %s", err)
			}

			if c.service != test.service {
				t.Fail()
			}
		})
	}
}

func TestConfigPath(t *testing.T) {
	tests := []struct {
		input string
		path  string
	}{
		{
			input: `{"name": "a/b/c", "prefix": "dummy"}`,
			path:  "dummy/a/b/c",
		},
		{
			input: `{"name": "a/b/c"}`,
			path:  "bundles/a/b/c",
		},
		{
			input: `{"name": "a/b/c/", "resource": "x/y/z"}`,
			path:  "x/y/z",
		},
		{
			input: `{"name": "a/b/c", "prefix": "/bundles2", resource: "x/y/z"}`,
			path:  "x/y/z",
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConfigDecision_case_%d", i), func(t *testing.T) {
			c, err := NewConfigBuilder().WithBytes([]byte(test.input)).WithServices([]string{"service1"}).Parse()
			if err != nil {
				t.Fatalf("unexpected error while parsing config: %s", err.Error())
			}

			if c.path != test.path {
				t.Fail()
			}
		})
	}
}
