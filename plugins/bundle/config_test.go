// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"fmt"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestConfigValidation(t *testing.T) {

	tests := []struct {
		input   string
		wantErr bool
	}{
		{
			input:   `{}`,
			wantErr: true,
		},
		{
			input:   `{"name": "a/b/c", "service": "invalid"}`,
			wantErr: true,
		},
		{
			input:   `{"name": "a/b/c", "service": "service2"}`,
			wantErr: false,
		},
		{
			input:   `{"name": "a/b/c", "service": "service2", "prefix": "mybundle"}`,
			wantErr: false,
		},
		{
			input:   `{"name": "a/b/c", "service": "service2", "prefix": "/"}`,
			wantErr: false,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConfigValidation_case_%d", i), func(t *testing.T) {
			_, err := ParseConfig([]byte(test.input), []string{"service1", "service2"})
			if err != nil && !test.wantErr {
				t.Fail()
			}
			if err == nil && test.wantErr {
				t.Fail()
			}
		})
	}
}

func TestConfigValid(t *testing.T) {

	in := `{"name": "a/b/c", "service": "service2", "prefix": "mybundle"}`

	config, err := ParseConfig([]byte(in), []string{"service1", "service2"})
	if err != nil {
		t.Fail()
	}

	if config.Name != "a/b/c" {
		t.Fatalf("want %v got %v", "a/b/c", config.Name)
	}
	if config.Service != "service2" {
		t.Fatalf("want %v got %v", "service2", config.Name)
	}
	if *(config.Prefix) != "mybundle" {
		t.Fatalf("want %v got %v", "mybundle", *(config.Prefix))
	}
}

func TestConfigCorrupted(t *testing.T) {

	in := `{"name": "a/b/c", "service": "service2", "prefix: mybundle"}`

	config, err := ParseConfig([]byte(in), []string{"service1", "service2"})
	if err != nil {
		t.Fail()
	}

	if config.Name != "a/b/c" {
		t.Fatalf("want %v got %v", "a/b/c", config.Name)
	}
	if config.Service != "service2" {
		t.Fatalf("want %v got %v", "service2", config.Name)
	}
	if *(config.Prefix) != "bundles" {
		t.Fatalf("want %v got %v", "bundles", *(config.Prefix))
	}
}

func TestLegacyDownloadPath(t *testing.T) {
	testCases := []struct {
		prefix string
		name   string
		result string
	}{
		{
			prefix: "/",
			name:   "bundles/bundles.tar.gz",
			result: "bundles/bundles.tar.gz",
		},
		{
			prefix: "bundles",
			name:   "bundles.tar.gz",
			result: "bundles/bundles.tar.gz",
		},
		{
			prefix: "",
			name:   "bundles/bundles.tar.gz",
			result: "bundles/bundles.tar.gz",
		},
		{
			prefix: "",
			name:   "/bundles.tar.gz",
			result: "bundles.tar.gz",
		},
	}
	for i, test := range testCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			config := Config{
				Name:   test.name,
				Prefix: &test.prefix,
			}

			bs, err := yaml.Marshal(&config)
			if err != nil {
				t.Fatalf("Unexpected error marshalling config: %s", err)
			}

			parsed, err := ParseConfig(bs, []string{"service1"})
			if err != nil {
				t.Fatalf("Unexpected error parsing config: %s", err)
			}

			b, ok := parsed.Bundles[test.name]
			if !ok {
				t.Fatalf("Expected resource %q on bundle with name %q", test.result, test.name)
			}

			if b.Resource != test.result {
				t.Errorf("Expected resource %q on bundle with name %q, actual: %s", test.result, test.name, b.Resource)
			}
		})
	}
}

func TestParseAndValidateBundlesConfig(t *testing.T) {
	tests := []struct {
		conf      string
		services  []string
		wantError bool
	}{
		{
			conf:      "",
			services:  []string{},
			wantError: false,
		},
		{
			conf:      "{{{",
			services:  []string{},
			wantError: true,
		},
		{
			conf:      `{"b1":{"service": "s1"}}`,
			services:  []string{},
			wantError: true,
		},
		{
			conf:      `{"b1":{"service": "s1"}}`,
			services:  []string{"s1"},
			wantError: false,
		},
		{
			conf:      `{"b1":{"service": "s1"}, "b2":{"service": "s1"}}`,
			services:  []string{"s1"},
			wantError: false,
		},
		{
			conf:      `{"b1":{"service": "s1"}, "b2":{"service": "s2"}}`,
			services:  []string{"s1"},
			wantError: true,
		},
		{
			conf:      `{"b1":{"service": "s1"}, "b2":{"service": "s2"}}`,
			services:  []string{"s1", "s2"},
			wantError: false,
		},
		{
			conf:      `{"b1":{"service": "s1", "polling": {"min_delay_seconds": 1, "max_delay_seconds": 5}}}`,
			services:  []string{"s1"},
			wantError: false,
		},
		{
			conf:      `{"b1":{"service": "s1", "polling": {"min_delay_seconds": 5, "max_delay_seconds": 1}}}`,
			services:  []string{"s1"},
			wantError: true,
		},
	}

	for i := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			_, err := ParseBundlesConfig([]byte(tests[i].conf), tests[i].services)
			if err != nil && !tests[i].wantError {
				t.Fatalf("Unexpected error: %s", err)
			}
			if err == nil && tests[i].wantError {
				t.Fatalf("Expected an error but didn't get one")
			}
		})
	}
}

func TestParseBundlesConfig(t *testing.T) {
	conf := []byte(`
bundle.tar.gz:
  service: s1
b2:
  service: s1
  resource: /b2/path/
b3:
  service: s3
  resource: /some/longer/path/bundle.tar.gz
`)
	services := []string{"s1", "s3"}
	parsedConfig, err := ParseBundlesConfig(conf, services)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if parsedConfig.Name != "" {
		t.Fatalf("Expected config `Name` to be empty, actual: %s", parsedConfig.Name)
	}

	if len(parsedConfig.Bundles) != 3 {
		t.Fatalf("Expected 3 bundles in parsed config, got: %+v", parsedConfig.Bundles)
	}

	expectedSources := map[string]struct {
		service  string
		resource string
	}{
		"bundle.tar.gz": {
			service:  "s1",
			resource: "bundles/bundle.tar.gz",
		},
		"b2": {
			service:  "s1",
			resource: "/b2/path/",
		},
		"b3": {
			service:  "s3",
			resource: "/some/longer/path/bundle.tar.gz",
		},
	}

	for name, expected := range expectedSources {
		actual, ok := parsedConfig.Bundles[name]
		if !ok {
			t.Fatalf("Expected to have bundle with name %s configured, actual: %+v", name, parsedConfig.Bundles)
		}
		if expected.resource != actual.Resource {
			t.Errorf("Expected resource '%s', found '%s'", expected.resource, actual.Resource)
		}
		if expected.service != actual.Service {
			t.Errorf("Expected service '%s', found '%s'", expected.service, actual.Service)
		}
	}
}

func TestConfigIsMultiBundle(t *testing.T) {
	tests := []struct {
		conf     Config
		expected bool
	}{
		{
			conf:     Config{},
			expected: true,
		},
		{
			conf:     Config{Name: "bundle.tar.gz"},
			expected: false,
		},
		{
			conf: Config{
				Name: "bundle.tar.gz",
				Bundles: map[string]*Source{
					"bundle.tar.gz": &Source{},
				},
			},
			expected: false,
		},
		{
			conf: Config{
				Name: "",
				Bundles: map[string]*Source{
					"bundle.tar.gz": &Source{},
				},
			},
			expected: true,
		},
	}

	for i := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			actual := tests[i].conf.IsMultiBundle()
			if actual != tests[i].expected {
				t.Errorf("expected %t but got %t", tests[i].expected, actual)
			}
		})
	}

}
