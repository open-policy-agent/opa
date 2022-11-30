//go:build includebuild
// +build includebuild

package wasmtime

// This file is not built and not included in BUILD.bazel;
// it is only used to prevent "go mod vendor" to prune the
// build directory.

import (
	// Import these build directories in order to have them
	// included in vendored dependencies.
	// Cf. https://github.com/golang/go/issues/26366

	_ "github.com/bytecodealliance/wasmtime-go/v3/build/include"
	_ "github.com/bytecodealliance/wasmtime-go/v3/build/include/wasmtime"
	_ "github.com/bytecodealliance/wasmtime-go/v3/build/linux-aarch64"
	_ "github.com/bytecodealliance/wasmtime-go/v3/build/linux-x86_64"
	_ "github.com/bytecodealliance/wasmtime-go/v3/build/macos-aarch64"
	_ "github.com/bytecodealliance/wasmtime-go/v3/build/macos-x86_64"
	_ "github.com/bytecodealliance/wasmtime-go/v3/build/windows-x86_64"
)
