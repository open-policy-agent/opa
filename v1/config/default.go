// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import "crypto/tls"

const (
	// DefaultMinTLSVersion is the minimum TLS version used by OPA server and REST clients
	DefaultMinTLSVersion = tls.VersionTLS12
)

var (
	// DefaultIncludeRuleMetadata controls whether custom metadata from rule annotations
	// is included in decision logs by default. When true, this also enables annotation
	// processing during policy parsing. Embedding projects can set this to true via an
	// init() function to change the default.
	DefaultIncludeRuleMetadata bool
)
