package decoding

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
		{
			input:   `{"gzip": [1, 2, 3]}`,
			wantErr: true,
		},
		{
			input:   `{"gzip": "nope"}`,
			wantErr: true,
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
			input:   `{"gzip": [1, 2, 3]}`,
			wantErr: "invalid value for server.decoding.gzip field, should be an object",
		},
		{
			input:   `{"gzip": "nope"}`,
			wantErr: "invalid value for server.decoding.gzip field, should be an object",
		},
		{
			input:   `{"max_length": true}`,
			wantErr: "invalid value for server.decoding.max_length field, should be a positive number",
		},
		{
			input:   `{"max_length": "foobar"}`,
			wantErr: "invalid value for server.decoding.max_length field, should be a positive number",
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

// TestConfigNoWarningsForKnownDecodingOptions verifies the server.decoding spec
// registered by this package lets config.ParseConfig recognize the section's
// options. Without the registration the section is treated as open, so this also
// guards against the unknown-option warning below silently going missing.
func TestConfigNoWarningsForKnownDecodingOptions(t *testing.T) {
	raw := []byte(`{"server": {"decoding": {"max_length": 5, "gzip": {"max_length": 42}}}}`)
	conf, err := config.ParseConfig(raw, "id")
	if err != nil {
		t.Fatal(err)
	}
	if len(conf.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", conf.Warnings)
	}
}

func TestConfigWarnsOnUnknownDecodingOption(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{
			raw:  `{"server": {"decoding": {"typo": 5}}}`,
			want: `unknown configuration option "server.decoding.typo" encountered`,
		},
		{
			raw:  `{"server": {"decoding": {"gzip": {"typo": 5}}}}`,
			want: `unknown configuration option "server.decoding.gzip.typo" encountered`,
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
