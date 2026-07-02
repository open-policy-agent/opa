// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package config

import "crypto/tls"

const (
	// DefaultMinTLSVersion is the minimum TLS version used by OPA server and REST clients
	DefaultMinTLSVersion = tls.VersionTLS12
)
