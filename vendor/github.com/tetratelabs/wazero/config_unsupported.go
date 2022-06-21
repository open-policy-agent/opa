// This is the opposite constraint of config_supported.go
//go:build !(amd64 || arm64) || !(darwin || linux || windows)

package wazero

func newRuntimeConfig() RuntimeConfig {
	return NewRuntimeConfigInterpreter()
}
