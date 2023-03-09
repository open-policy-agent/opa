package encoding

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
			input:   `{"gzip": {"min_length": "not-a-number"}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip": {min_length": 42}}`,
			wantErr: false,
		},
		{
			input:   `{"gzip":{"min_length": "42"}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip":{"min_length": 0}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip":{"min_length": -10}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip":{"random_key": 0}}`,
			wantErr: false,
		},
		{
			input:   `{"gzip": {"min_length": -10, "compression_level": 13}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip":{"compression_level": "not-an-number"}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip":{"compression_level": 1}}`,
			wantErr: false,
		},
		{
			input:   `{"gzip":{"compression_level": 13}}`,
			wantErr: true,
		},
		{
			input:   `{"gzip":{"min_length": 42, "compression_level": 9}}`,
			wantErr: false,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConfigValidation_case_%d", i), func(t *testing.T) {
			_, err := NewConfigBuilder().WithBytes([]byte(test.input)).Parse()
			if err != nil && !test.wantErr {
				t.Fail()
			}
			if err == nil && test.wantErr {
				t.Fail()
			}
		})
	}
}

func TestConfigValue(t *testing.T) {
	tests := []struct {
		input                         string
		minLengthExpectedValue        int
		compressionLevelExpectedValue int
	}{
		{
			input:                         `{}`,
			minLengthExpectedValue:        1024,
			compressionLevelExpectedValue: 9,
		},
		{
			input:                         `{"gzip":{"min_length": 42, "compression_level": 1}}`,
			minLengthExpectedValue:        42,
			compressionLevelExpectedValue: 1,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConfigValue_case_%d", i), func(t *testing.T) {
			config, err := NewConfigBuilder().WithBytes([]byte(test.input)).Parse()
			if err != nil {
				t.Fail()
			}
			if *config.Gzip.MinLength != test.minLengthExpectedValue || *config.Gzip.CompressionLevel != test.compressionLevelExpectedValue {
				t.Fail()
			}
		})
	}
}
