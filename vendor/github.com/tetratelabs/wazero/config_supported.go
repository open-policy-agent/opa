// Note: The build constraints here are about the compiler, which is more
// narrow than the architectures supported by the assembler.
//
// Constraints here must match platform.IsSupported.
//
// Meanwhile, users who know their runtime.GOOS can operate with the compiler
// may choose to use NewRuntimeConfigCompiler explicitly.
//go:build (amd64 || arm64) && (darwin || linux || windows)

package wazero

func newRuntimeConfig() RuntimeConfig {
	return NewRuntimeConfigCompiler()
}
