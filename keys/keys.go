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
