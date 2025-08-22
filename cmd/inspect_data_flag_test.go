// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectDataFlag(t *testing.T) {
	tests := []struct {
		name        string
		files       map[string]string
		dataPaths   []string
		wantErr     bool
		errContains string
		wantOutput  []string
	}{
		{
			name: "inspect single json data file",
			files: map[string]string{
				"data.json": `{"users": {"alice": {"role": "admin"}}}`,
			},
			dataPaths: []string{"data.json"},
			wantOutput: []string{
				"NAMESPACES:",
				"data",
				"data.json",
			},
		},
		{
			name: "inspect single yaml data file",
			files: map[string]string{
				"config.yaml": `
users:
  alice:
    role: admin
`,
			},
			dataPaths: []string{"config.yaml"},
			wantOutput: []string{
				"NAMESPACES:",
				"data",
				"config.yaml",
			},
		},
		{
			name: "inspect multiple data files",
			files: map[string]string{
				"data1.json": `{"key1": "value1"}`,
				"data2.json": `{"key2": "value2"}`,
			},
			dataPaths: []string{"data1.json", "data2.json"},
			wantOutput: []string{
				"NAMESPACES:",
				"data",
				"data1.json",
				"data2.json",
			},
		},
		{
			name: "error on non-data file",
			files: map[string]string{
				"policy.rego": `package test`,
			},
			dataPaths:   []string{"policy.rego"},
			wantErr:     true,
			errContains: "is not a JSON or YAML data file",
		},
		{
			name:        "error on non-existent file",
			files:       map[string]string{},
			dataPaths:   []string{"missing.json"},
			wantErr:     true,
			errContains: "error accessing path",
		},
		{
			name: "inspect directory with --data flag",
			files: map[string]string{
				"bundle/data.json": `{"foo": "bar"}`,
				"bundle/.manifest": `{"revision": "test"}`,
			},
			dataPaths: []string{"bundle"},
			wantOutput: []string{
				"MANIFEST:",
				"Revision",
				"test",
				"NAMESPACES:",
				"data",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tempDir := t.TempDir()

			// Create test files
			for path, content := range tt.files {
				fullPath := filepath.Join(tempDir, path)
				dir := filepath.Dir(fullPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("failed to create directory: %v", err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatalf("failed to write file: %v", err)
				}
			}

			// Change to temp directory
			oldWd, _ := os.Getwd()
			os.Chdir(tempDir)
			defer os.Chdir(oldWd)

			// Create params with data paths
			params := newInspectCommandParams()
			for _, p := range tt.dataPaths {
				params.dataPaths.Set(p)
			}

			var out bytes.Buffer
			err := doInspect(params, "", &out)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Check output
			output := out.String()
			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("expected output to contain %q, got:\n%s", want, output)
				}
			}
		})
	}
}