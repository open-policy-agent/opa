// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"fmt"
	"testing"
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
