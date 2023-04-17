// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package download

import (
	"encoding/json"
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {

	tests := []struct {
		note    string
		input   string
		wantErr bool
		expMin  time.Duration
		expMax  time.Duration
	}{
		{
			note: "min > max",
			input: `{
				"polling": {
					"min_delay_seconds": 10,
					"max_delay_seconds": 1
				}
			}`,
			wantErr: true,
		},
		{
			note:   "empty",
			input:  `{}`,
			expMin: time.Second * time.Duration(defaultMinDelaySeconds),
			expMax: time.Second * time.Duration(defaultMaxDelaySeconds),
		},
		{
			note: "min missing",
			input: `{
				"polling": {
					"max_delay_seconds": 10
				}
			}`,
			wantErr: true,
		},
		{
			note: "max missing",
			input: `{
				"polling": {
					"min_delay_seconds": 1
				}
			}`,
			wantErr: true,
		},
		{
			note: "user supplied",
			input: `{
				"polling": {
					"min_delay_seconds": 10,
					"max_delay_seconds": 30
				}
			}`,
			expMin: time.Second * 10,
			expMax: time.Second * 30,
		},
		{
			note: "long polling timeout < 1",
			input: `{
				"polling": {
					"long_polling_timeout_seconds": 0
				}
			}`,
			wantErr: true,
		},
	}

	for _, test := range tests {

		var config Config

		if err := json.Unmarshal([]byte(test.input), &config); err != nil {
			t.Fatal(err)
		}

		err := config.ValidateAndInjectDefaults()
		if err != nil && !test.wantErr {
			t.Errorf("Unexpected error on: %v, err: %v", test.input, err)
		}

		if err == nil {
			if time.Duration(*config.Polling.MinDelaySeconds) != test.expMin {
				t.Errorf("For %q expected min %v but got %v", test.note, test.expMin, time.Duration(*config.Polling.MinDelaySeconds))
			}
			if time.Duration(*config.Polling.MaxDelaySeconds) != test.expMax {
				t.Errorf("For %q expected min %v but got %v", test.note, test.expMax, time.Duration(*config.Polling.MaxDelaySeconds))
			}
		}
	}
}
