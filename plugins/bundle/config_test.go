// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import "testing"

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
			input:   `{"name": a/b/c", "service": "service2"}`,
			wantErr: false,
		},
	}

	for _, test := range tests {
		config, _ := ParseConfig([]byte(test.input), []string{"service1", "service2"})
		if config == nil {
			continue
		}
	}
}
