// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package discovery

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/keys"
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
			input:    `{"name": "a/b/c", "decision": "query", "signing": {"keyid": "foo", "scope": "write"}}`,
			services: []string{"s1"},
			wantErr:  false,
		},
		{
			input:    `{"name": "a/b/c", "decision": "query", "signing": {"keyid": "bar", "scope": "write"}}`,
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

func TestConfigSignatureRequired(t *testing.T) {
	tests := []struct {
		note             string
		input            string
		keys             map[string]*keys.Config
		expSigning       bool
		expSignatureReqd bool
		expErr           string
	}{
		{
			note:             "no signing block, top-level keys injected",
			input:            `{"name": "a/b/c"}`,
			keys:             map[string]*keys.Config{"foo": {Key: "secret", Algorithm: "HS256"}},
			expSigning:       true,
			expSignatureReqd: false,
		},
		{
			note:       "no signing block, no keys",
			input:      `{"name": "a/b/c"}`,
			keys:       map[string]*keys.Config{},
			expSigning: false,
		},
		{
			note:             "empty signing block requires a signature",
			input:            `{"name": "a/b/c", "signing": {}}`,
			keys:             map[string]*keys.Config{"foo": {Key: "secret", Algorithm: "HS256"}},
			expSigning:       true,
			expSignatureReqd: true,
		},
		{
			note:             "scope only requires a signature",
			input:            `{"name": "a/b/c", "signing": {"scope": "write"}}`,
			keys:             map[string]*keys.Config{"foo": {Key: "secret", Algorithm: "HS256"}},
			expSigning:       true,
			expSignatureReqd: true,
		},
		{
			note:             "exclude_files only requires a signature",
			input:            `{"name": "a/b/c", "signing": {"exclude_files": ["data.json"]}}`,
			keys:             map[string]*keys.Config{"foo": {Key: "secret", Algorithm: "HS256"}},
			expSigning:       true,
			expSignatureReqd: true,
		},
		{
			note:             "keyid requires a signature",
			input:            `{"name": "a/b/c", "signing": {"keyid": "foo"}}`,
			keys:             map[string]*keys.Config{"foo": {Key: "secret", Algorithm: "HS256"}},
			expSigning:       true,
			expSignatureReqd: true,
		},
		{
			note:   "signing block with no keys is a config error",
			input:  `{"name": "a/b/c", "signing": {}}`,
			keys:   map[string]*keys.Config{},
			expErr: "invalid configuration for discovery service: no verification keys configured",
		},
		{
			note:   "unknown keyid is a config error",
			input:  `{"name": "a/b/c", "signing": {"keyid": "bar"}}`,
			keys:   map[string]*keys.Config{"foo": {Key: "secret", Algorithm: "HS256"}},
			expErr: "invalid configuration for discovery service: key id bar not found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			conf, err := NewConfigBuilder().WithBytes([]byte(tc.input)).WithServices([]string{"s1"}).
				WithKeyConfigs(tc.keys).Parse()

			if tc.expErr != "" {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if err.Error() != tc.expErr {
					t.Fatalf("expected error %q but got %v", tc.expErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			if tc.expSigning != (conf.Signing != nil) {
				t.Fatalf("expected Signing != nil to be %v but got %v", tc.expSigning, conf.Signing != nil)
			}

			if conf.Signing == nil {
				return
			}

			if act := conf.Signing.SignatureRequired(); act != tc.expSignatureReqd {
				t.Fatalf("expected SignatureRequired() to be %v but got %v", tc.expSignatureReqd, act)
			}
		})
	}
}

// TestConfigAllowUnsignedDeltaBundles covers the
// `discovery.signing.allow_unsigned_delta_bundles` opt-out.
func TestConfigAllowUnsignedDeltaBundles(t *testing.T) {
	keyConfigs := map[string]*keys.Config{"foo": {Key: "secret", Algorithm: "HS256"}}

	tests := []struct {
		note                    string
		input                   string
		keys                    map[string]*keys.Config
		expSignatureReqd        bool
		expUnsignedDeltaAllowed bool
	}{
		{
			note:                    "unset",
			input:                   `{"name": "a/b/c", "signing": {"keyid": "foo"}}`,
			keys:                    keyConfigs,
			expSignatureReqd:        true,
			expUnsignedDeltaAllowed: false,
		},
		{
			note:                    "set to true",
			input:                   `{"name": "a/b/c", "signing": {"keyid": "foo", "allow_unsigned_delta_bundles": true}}`,
			keys:                    keyConfigs,
			expSignatureReqd:        true,
			expUnsignedDeltaAllowed: true,
		},
		{
			note:                    "set to false",
			input:                   `{"name": "a/b/c", "signing": {"keyid": "foo", "allow_unsigned_delta_bundles": false}}`,
			keys:                    keyConfigs,
			expSignatureReqd:        true,
			expUnsignedDeltaAllowed: false,
		},
		{
			note:                    "no signing block, top-level keys injected",
			input:                   `{"name": "a/b/c"}`,
			keys:                    keyConfigs,
			expSignatureReqd:        false,
			expUnsignedDeltaAllowed: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			conf, err := NewConfigBuilder().WithBytes([]byte(tc.input)).WithServices([]string{"s1"}).
				WithKeyConfigs(tc.keys).Parse()
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}

			if conf.Signing == nil {
				t.Fatal("expected a verification config but got nil")
			}

			if act := conf.Signing.SignatureRequired(); act != tc.expSignatureReqd {
				t.Fatalf("expected SignatureRequired() to be %v but got %v", tc.expSignatureReqd, act)
			}

			if act := conf.Signing.UnsignedDeltaBundlesAllowed(); act != tc.expUnsignedDeltaAllowed {
				t.Fatalf("expected UnsignedDeltaBundlesAllowed() to be %v but got %v", tc.expUnsignedDeltaAllowed, act)
			}
		})
	}
}
