// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package download

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {

	tests := []struct {
		note         string
		input        string
		wantErr      bool
		expMin       *int64
		expMax       *int64
		expParsedMin time.Duration
		expParsedMax time.Duration
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
			note:         "empty",
			input:        `{}`,
			expMin:       nil,
			expMax:       nil,
			expParsedMin: time.Second * time.Duration(defaultMinDelaySeconds),
			expParsedMax: time.Second * time.Duration(defaultMaxDelaySeconds),
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
			expMin: func() *int64 {
				min := int64(10)
				return &min
			}(),
			expMax: func() *int64 {
				max := int64(30)
				return &max
			}(),
			expParsedMin: time.Second * 10,
			expParsedMax: time.Second * 30,
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
		t.Run(test.note, func(t *testing.T) {
			var config Config

			if err := json.Unmarshal([]byte(test.input), &config); err != nil {
				t.Fatal(err)
			}

			// no matter how many calls to ValidateAndInjectDefaults, the values should stay the same
			for range 3 {
				err := config.ValidateAndInjectDefaults()
				if err != nil && !test.wantErr {
					t.Errorf("Unexpected error on: %v, err: %v", test.input, err)
				} else if err != nil && test.wantErr {
					return
				}

				if config.Polling.MinDelaySeconds == nil && test.expMin != nil {
					t.Fatal("Expected min delay seconds to be set")
				}
				if config.Polling.MinDelaySeconds != nil && *config.Polling.MinDelaySeconds != *test.expMin {
					t.Errorf("For %q expected min %v but got %v", test.note, test.expMin, time.Duration(*config.Polling.MinDelaySeconds))
				}
				if config.Polling.MaxDelaySeconds == nil && test.expMax != nil {
					t.Fatal("Expected min delay seconds to be set")
				}
				if config.Polling.MaxDelaySeconds != nil && *config.Polling.MaxDelaySeconds != *test.expMax {
					t.Errorf("For %q expected max %v but got %v", test.note, test.expMax, time.Duration(*config.Polling.MaxDelaySeconds))
				}

				if time.Duration(*config.Polling.parsedMinDelaySeconds) != test.expParsedMin {
					t.Errorf("For %q expected min %v but got %v", test.note, test.expParsedMin, time.Duration(*config.Polling.MinDelaySeconds))
				}
				if time.Duration(*config.Polling.parsedMaxDelaySeconds) != test.expParsedMax {
					t.Errorf("For %q expected max %v but got %v", test.note, test.expParsedMax, time.Duration(*config.Polling.MaxDelaySeconds))
				}
			}
		})
	}
}

func TestConfigValidationUpdate(t *testing.T) {
	expMin := int64(10)
	expMax := int64(30)
	expParsedMin := time.Second * time.Duration(expMin)
	expParsedMax := time.Second * time.Duration(expMax)
	var config Config

	if err := json.Unmarshal([]byte(fmt.Sprintf(`{
				"polling": {
					"min_delay_seconds": %d,
					"max_delay_seconds": %d
				}
			}`, expMin, expMax)), &config); err != nil {
		t.Fatal(err)
	}

	err := config.ValidateAndInjectDefaults()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if *config.Polling.MinDelaySeconds != expMin {
		t.Errorf("expected min %v but got %v", expMin, time.Duration(*config.Polling.MinDelaySeconds))
	}

	if config.Polling.MaxDelaySeconds != nil && *config.Polling.MaxDelaySeconds != expMax {
		t.Errorf("expected max %v but got %v", expMax, time.Duration(*config.Polling.MaxDelaySeconds))
	}

	if time.Duration(*config.Polling.parsedMinDelaySeconds) != expParsedMin {
		t.Errorf("expected min %v but got %v", expParsedMin, time.Duration(*config.Polling.MinDelaySeconds))
	}
	if time.Duration(*config.Polling.parsedMaxDelaySeconds) != expParsedMax {
		t.Errorf("expected max %v but got %v", expParsedMax, time.Duration(*config.Polling.MaxDelaySeconds))
	}

	expMin = int64(50)
	expMax = int64(100)
	expParsedMin = time.Second * time.Duration(expMin)
	expParsedMax = time.Second * time.Duration(expMax)

	config.Polling.MinDelaySeconds = &expMin
	config.Polling.MaxDelaySeconds = &expMax

	err = config.ValidateAndInjectDefaults()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if *config.Polling.MinDelaySeconds != expMin {
		t.Errorf("expected min %v but got %v", expMin, time.Duration(*config.Polling.MinDelaySeconds))
	}

	if config.Polling.MaxDelaySeconds != nil && *config.Polling.MaxDelaySeconds != expMax {
		t.Errorf("expected max %v but got %v", expMax, time.Duration(*config.Polling.MaxDelaySeconds))
	}

	if time.Duration(*config.Polling.parsedMinDelaySeconds) != expParsedMin {
		t.Errorf("expected min %v but got %v", expParsedMin, time.Duration(*config.Polling.MinDelaySeconds))
	}
	if time.Duration(*config.Polling.parsedMaxDelaySeconds) != expParsedMax {
		t.Errorf("expected max %v but got %v", expParsedMax, time.Duration(*config.Polling.MaxDelaySeconds))
	}
}
