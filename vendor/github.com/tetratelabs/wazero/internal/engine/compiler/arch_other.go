//go:build !amd64 && !arm64

package compiler

import (
	"fmt"
	"runtime"

	"github.com/tetratelabs/wazero/internal/wazeroir"
)

const isSupported = false

// archContext is empty on an unsupported architecture.
type archContext struct{}

// newCompiler returns an unsupported error.
func newCompiler(ir *wazeroir.CompilationResult) (compiler, error) {
	return nil, fmt.Errorf("unsupported GOARCH %s", runtime.GOARCH)
}
