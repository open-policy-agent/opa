// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPluginsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		conf     Config
		expected bool
	}{
		{
			name:     "empty config",
			conf:     Config{},
			expected: false,
		},
		{
			name: "bundle",
			conf: Config{
				Bundle: []byte(`{"bundle": {"name": "test-bundle"}}`),
			},
			expected: true,
		},
		{
			name: "bundles",
			conf: Config{
				Bundles: []byte(`{"bundles": {"test-bundle": {}}`),
			},
			expected: true,
		},
		{
			name: "decision_logs",
			conf: Config{
				DecisionLogs: []byte(`{decision_logs: {}}`),
			},
			expected: true,
		},
		{
			name: "status",
			conf: Config{
				Status: []byte(`{status: {}}`),
			},
			expected: true,
		},
		{
			name: "plugins",
			conf: Config{
				Plugins: map[string]json.RawMessage{
					"some-plugin": {},
				},
			},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := test.conf.PluginsEnabled()
			if actual != test.expected {
				t.Errorf("Expected %t but got %t", test.expected, actual)
			}
		})
	}
}

func TestPersistDirectory(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("%v", err)
	}

	c := Config{}
	persistDir, err := c.GetPersistenceDirectory()
	if err != nil {
		t.Fatalf("%v", err)
	}

	if persistDir != filepath.Join(pwd, ".opa") {
		t.Errorf("expected persistDir to be %v, got %v", filepath.Join(pwd, ".opa"), persistDir)
	}

	dir := "/var/opa"
	c.PersistenceDirectory = &dir
	persistDir, err = c.GetPersistenceDirectory()
	if err != nil {
		t.Fatalf("%v", err)
	}

	if persistDir != dir {
		t.Errorf("expected peristDir %v and dir %v to be equal", persistDir, dir)
	}
}
