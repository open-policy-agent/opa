package ast

import "context"

type regoCompileCtx struct{}

func WithCompiler(ctx context.Context, c *Compiler) context.Context {
	return context.WithValue(ctx, regoCompileCtx{}, c)
}

func CompilerFromContext(ctx context.Context) (*Compiler, bool) {
	if ctx == nil {
		return nil, false
	}
	v, ok := ctx.Value(regoCompileCtx{}).(*Compiler)
	return v, ok
}
