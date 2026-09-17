// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Deprecated: This package is intended for older projects transitioning from OPA v0.x and will remain for the lifetime of OPA v1.x, but its use is not recommended.
// For newer features and behaviours, such as defaulting to the Rego v1 syntax, use the corresponding components in the [github.com/open-policy-agent/opa/v1] package instead.
// See https://www.openpolicyagent.org/docs/latest/v0-compatibility/ for more information.
package keys

import (
	"encoding/json"

	v1 "github.com/open-policy-agent/opa/v1/keys"
)

// IsSupportedAlgorithm true if provided alg is supported
func IsSupportedAlgorithm(alg string) bool {
	return v1.IsSupportedAlgorithm(alg)
}

// Config holds the keys used to sign or verify bundles and tokens
type Config = v1.Config

// NewKeyConfig return a new Config
func NewKeyConfig(key, alg, scope string) (*Config, error) {
	return v1.NewKeyConfig(key, alg, scope)
}

// ParseKeysConfig returns a map containing the key and the signing algorithm
func ParseKeysConfig(raw json.RawMessage) (map[string]*Config, error) {
	return v1.ParseKeysConfig(raw)
}
