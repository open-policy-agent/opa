// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"strings"
	"testing"
)

func TestValidateInspectParams(t *testing.T) {
	tests := []struct {
		name        string
		params      inspectCommandParams
		args        []string
		wantErr     bool
		errContains string
	}{
		{
			name:   "valid single path",
			params: newInspectCommandParams(),
			args:   []string{"bundle.tar.gz"},
		},
		{
			name:   "error on no arguments and no data flag",
			params: newInspectCommandParams(),
			args:   []string{},
			wantErr: true,
			errContains: "specify exactly one OPA bundle or path",
		},
		{
			name:   "error on multiple path arguments",
			params: newInspectCommandParams(),
			args:   []string{"path1", "path2"},
			wantErr: true,
			errContains: "specify exactly one OPA bundle or path",
		},
		{
			name: "valid data flag with no args",
			params: func() inspectCommandParams {
				p := newInspectCommandParams()
				p.dataPaths.Set("data.json")
				return p
			}(),
			args: []string{},
		},
		{
			name: "error on mixing data flag with path arg",
			params: func() inspectCommandParams {
				p := newInspectCommandParams()
				p.dataPaths.Set("data.json")
				return p
			}(),
			args: []string{"bundle.tar.gz"},
			wantErr: true,
			errContains: "specify either a bundle/path argument or --data flag, not both",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInspectParams(&tt.params, tt.args)
			
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}