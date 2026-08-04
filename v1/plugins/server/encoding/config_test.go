package encoding

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
			wantErr: "invalid value for server.encoding.gzip field, should be an object",
		},
		{
			input:   `{"gzip": 7}`,
			wantErr: "invalid value for server.encoding.gzip field, should be an object",
		},
		{
			input:   `{"gzip": {"min_length": true}}`,
			wantErr: "invalid value for server.encoding.gzip.min_length field, should be a positive number",
		},
		{
			input:   `{"gzip": {"min_length": "foobar"}}`,
			wantErr: "invalid value for server.encoding.gzip.min_length field, should be a positive number",
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

// TestConfigNoWarningsForKnownEncodingOptions verifies the server.encoding spec
// registered by this package lets config.ParseConfig recognize the section's
// options. Without the registration the section is treated as open, so this also
// guards against the unknown-option warning below silently going missing.
func TestConfigNoWarningsForKnownEncodingOptions(t *testing.T) {
	raw := []byte(`{"server": {"encoding": {"gzip": {"min_length": 42, "compression_level": 1}}}}`)
	conf, err := config.ParseConfig(raw, "id")
	if err != nil {
		t.Fatal(err)
	}
	if len(conf.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", conf.Warnings)
	}
}

func TestConfigWarnsOnUnknownEncodingOption(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{
			raw:  `{"server": {"encoding": {"typo": 5}}}`,
			want: `unknown configuration option "server.encoding.typo" encountered`,
		},
		{
			raw:  `{"server": {"encoding": {"gzip": {"typo": 5}}}}`,
			want: `unknown configuration option "server.encoding.gzip.typo" encountered`,
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
