package decoding

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
			wantErr: false,
		},
		{
			input:   `{"gzip": {"max_length": "not-a-number"}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip": {max_length": 42}}`,
			wantErr: false,
		},
		{
			input:   `{"gzip":{"max_length": "42"}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip":{"max_length": 0}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip":{"max_length": -10}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip":{"random_key": 0}}`,
			wantErr: false,
		},
		{
			input:   `{"gzip": {"max_length": -10}}`,
			wantErr: true,
		},
		{
			input:   `{"max_length": "not-a-number"}`,
			wantErr: true,
		},
		{
			input:   `{"gzip":{}}`,
			wantErr: false,
		},
		{
			input:   `{"max_length": "not-a-number", "gzip":{}}`,
			wantErr: true,
		},
		{
			input:   `{"max_length": 42, "gzip":{"max_length": 42}}`,
			wantErr: false,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConfigValidation_case_%d", i), func(t *testing.T) {
			_, err := NewConfigBuilder().WithBytes([]byte(test.input)).Parse()
			if err != nil && !test.wantErr {
				t.Fatalf("Unexpected error: %s", err.Error())
			}
			if err == nil && test.wantErr {
				t.Fail()
			}
		})
	}
}

func TestConfigValue(t *testing.T) {
	tests := []struct {
		input                      string
		maxLengthExpectedValue     int64
		gzipMaxLengthExpectedValue int64
	}{
		{
			input:                      `{}`,
			maxLengthExpectedValue:     268435456,
			gzipMaxLengthExpectedValue: 536870912,
		},
		{
			input:                      `{"max_length": 5, "gzip":{"max_length": 42}}`,
			maxLengthExpectedValue:     5,
			gzipMaxLengthExpectedValue: 42,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConfigValue_case_%d", i), func(t *testing.T) {
			config, err := NewConfigBuilder().WithBytes([]byte(test.input)).Parse()
			if err != nil {
				t.Fatalf("Error building configuration: %s", err.Error())
			}
			if *config.MaxLength != test.maxLengthExpectedValue {
				t.Fatalf("Unexpected config value for max_length (exp/actual): %d, %d", test.maxLengthExpectedValue, *config.MaxLength)
			}
			if *config.Gzip.MaxLength != test.gzipMaxLengthExpectedValue {
				t.Fatalf("Unexpected config value for gzip.max_length (exp/actual): %d, %d", test.gzipMaxLengthExpectedValue, *config.Gzip.MaxLength)
			}
		})
	}
}
