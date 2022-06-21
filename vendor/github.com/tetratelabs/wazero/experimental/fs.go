package experimental

import (
	"context"
	"io/fs"

	"github.com/tetratelabs/wazero/api"
	internalfs "github.com/tetratelabs/wazero/internal/sys"
)

// WithFS overrides fs.FS in the context-based manner. Caller needs to take responsibility for closing the filesystem.
//
// Note: This has the same effect as the same function name on wazero.ModuleConfig.
func WithFS(ctx context.Context, fs fs.FS) (context.Context, api.Closer, error) {
	fsConfig := internalfs.NewFSConfig().WithFS(fs)
	preopens, err := fsConfig.Preopens()
	if err != nil {
		return nil, nil, err
	}

	fsCtx := internalfs.NewFSContext(preopens)
	return context.WithValue(ctx, internalfs.FSKey{}, fsCtx), fsCtx, nil
}
