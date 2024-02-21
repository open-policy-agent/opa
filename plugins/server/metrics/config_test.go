package metrics

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
			input:   `{"prom": {}}`,
			wantErr: false,
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": {}}}`,
			wantErr: false,
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": {"buckets": []}}}`,
			wantErr: false,
		},

		{
			input:   `{"prom": {"http_request_duration_seconds": {"buckets": ["not-a-array"]}}}`,
			wantErr: true,
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": {"buckets": [1]}}}`,
			wantErr: false,
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": {"buckets": "1"}}}`,
			wantErr: true,
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": {"buckets": [0.001, "1", "2"]}}}`,
			wantErr: true,
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": {"buckets": ["one", "two", "three"]}}}`,
			wantErr: true,
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": {"buckets": ["0.1", "0.2", "0.3", "4"]}}}`,
			wantErr: true,
		},
		{
			input:   `{"prom": {"random_key": 0}}`,
			wantErr: false,
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": {"random_key": 0}}}`,
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
		input         string
		expectedValue []float64
	}{
		{
			input:         `{}`,
			expectedValue: defaultHTTPRequestBuckets,
		},
		{
			input:         `{"prom": {}}`,
			expectedValue: defaultHTTPRequestBuckets,
		},
		{
			input:         `{"prom": {"http_request_duration_seconds": {}}}`,
			expectedValue: defaultHTTPRequestBuckets,
		},
		{
			input:         `{"prom": {"http_request_duration_seconds": {"buckets": []}}}`,
			expectedValue: []float64{},
		},
		{
			input:         `{"prom": {"http_request_duration_seconds": {"buckets":[0.1, 0.2, 0.3, 4]}}}`,
			expectedValue: []float64{0.1, 0.2, 0.3, 4},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("TestConfigValue_case_%d", i), func(t *testing.T) {
			config, err := NewConfigBuilder().WithBytes([]byte(test.input)).Parse()
			if err != nil {
				t.Fail()
			}
			if !valuesAreEqual(config.Prom.HTTPRequestDurationSeconds.Buckets, test.expectedValue) {
				t.Fail()
			}
		})
	}
}

func valuesAreEqual(a []float64, b []float64) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}

	return true
}
