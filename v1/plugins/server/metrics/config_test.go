package metrics

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/config"
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
		{
			input:   `{"prom": [1, 2, 3]}`,
			wantErr: true,
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": [1]}}`,
			wantErr: true,
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

// TestConfigRejectsWrongShapedOptions covers options the validation policy has to
// reject itself. Defaults are merged over the raw config, so an option that
// should hold an object would otherwise be silently replaced by the defaults
// instead of reaching the Go unmarshal that used to report the type error.
func TestConfigRejectsWrongShapedOptions(t *testing.T) {
	tests := []struct {
		input   string
		wantErr string
	}{
		{
			input:   `{"prom": [1, 2, 3]}`,
			wantErr: "invalid value for server.metrics.prom field, should be an object",
		},
		{
			input:   `{"prom": "nope"}`,
			wantErr: "invalid value for server.metrics.prom field, should be an object",
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": [1]}}`,
			wantErr: "invalid value for server.metrics.prom.http_request_duration_seconds field, should be an object",
		},
		{
			input:   `{"prom": {"http_request_duration_seconds": {"buckets": ["a"]}}}`,
			wantErr: "buckets field, should be an array of numbers",
		},
		{
			input:   `[1, 2, 3]`,
			wantErr: "config must be an object",
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			_, err := NewConfigBuilder().WithBytes([]byte(test.input)).Parse()
			if err == nil {
				t.Fatalf("expected error containing %q, got none", test.wantErr)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("expected error containing %q, got %q", test.wantErr, err.Error())
			}
		})
	}
}

// TestConfigNoWarningsForKnownMetricsOptions verifies the server.metrics spec
// registered by this package lets config.ParseConfig recognize the section's
// options. Without the registration the section is treated as open, so this also
// guards against the unknown-option warning below silently going missing.
func TestConfigNoWarningsForKnownMetricsOptions(t *testing.T) {
	raw := []byte(`{"server": {"metrics": {"prom": {"http_request_duration_seconds": {"buckets": [0.1, 1]}}}}}`)
	conf, err := config.ParseConfig(raw, "id")
	if err != nil {
		t.Fatal(err)
	}
	if len(conf.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", conf.Warnings)
	}
}

func TestConfigWarnsOnUnknownMetricsOption(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{
			raw:  `{"server": {"metrics": {"typo": 5}}}`,
			want: `unknown configuration option "server.metrics.typo" encountered`,
		},
		{
			raw:  `{"server": {"metrics": {"prom": {"typo": 5}}}}`,
			want: `unknown configuration option "server.metrics.prom.typo" encountered`,
		},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			conf, err := config.ParseConfig([]byte(tc.raw), "id")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(conf.Warnings, tc.want) {
				t.Fatalf("expected warning %q, got %v", tc.want, conf.Warnings)
			}
		})
	}
}
