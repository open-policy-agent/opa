package logs

import (
	"context"
	"sync"
)

type ctxKey struct{}

type extra struct {
	mu sync.RWMutex
	m  map[string]interface{}
}

func (e *extra) Set(key string, val interface{}) {
	e.mu.Lock()
	e.m[key] = val
	e.mu.Unlock()
}

func (e *extra) extra() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.m) == 0 {
		return nil
	}

	c := make(map[string]interface{}, len(e.m))
	for k, v := range e.m {
		c[k] = v
	}

	return c
}

func WithContext(ctx context.Context) context.Context {
	if _, ok := ctx.Value(ctxKey{}).(*extra); ok {
		return ctx
	}

	return context.WithValue(ctx, ctxKey{}, &extra{
		m: make(map[string]interface{}),
	})
}

func SetExtra(ctx context.Context, key string, val interface{}) {
	e, ok := ctx.Value(ctxKey{}).(*extra)
	if !ok {
		return
	}

	e.Set(key, val)
}

func getExtra(ctx context.Context) map[string]interface{} {
	e, ok := ctx.Value(ctxKey{}).(*extra)
	if !ok {
		return nil
	}
	return e.extra()
}
