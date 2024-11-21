// Package decoding implements the configuration side of the upgraded gzip
// decompression framework. The original work only enabled gzip decoding for
// a few endpoints-- here we enable if for all of OPA. Additionally, we provide
// some new defensive configuration options: max_length, and gzip.max_length.
// These allow rejecting requests that indicate their contents are larger than
// the size limits.
//
// The request handling pipeline now looks roughly like this:
//
// Request -> MaxBytesReader(Config.MaxLength) -> ir.CopyN(dest, req, Gzip.MaxLength)
//
// The intent behind this design is to improve how OPA handles large and/or
// malicious requests, compressed or otherwise. The benefit of being a little
// more strict in what we allow is that we can now use "riskier", but
// dramatically more performant techniques, like preallocating content buffers
// for gzipped data. This also should help OPAs in limited memory situations.
package decoding

import (
	v1 "github.com/open-policy-agent/opa/v1/plugins/server/decoding"
)

// Config represents the configuration for the Server.Decoding settings
type Config = v1.Config

// Gzip represents the configuration for the Server.Decoding.Gzip settings
type Gzip = v1.Gzip

// ConfigBuilder assists in the construction of the plugin configuration.
type ConfigBuilder = v1.ConfigBuilder

// NewConfigBuilder returns a new ConfigBuilder to build and parse the server config
func NewConfigBuilder() *ConfigBuilder {
	return v1.NewConfigBuilder()
}
