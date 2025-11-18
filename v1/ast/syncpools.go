package ast

import (
	"bytes"
	"strings"
	"sync"
)

var (
	TermPtrPool     = NewSyncPool[Term]()
	BytesReaderPool = NewSyncPool[bytes.Reader]()
	IndexResultPool = NewSyncPool[IndexResult]()
	// Needs custom pool because of custom Put logic.
	sbPool = &stringBuilderPool{
		pool: sync.Pool{
			New: func() any {
				return &strings.Builder{}
			},
		},
	}
	// Needs custom pool because of custom Put logic.
	varVisitorPool = &vvPool{
		pool: sync.Pool{
			New: func() any {
				return NewVarVisitor()
			},
		},
	}
)

type (
	syncPool[T any] struct {
		pool sync.Pool
	}
	stringBuilderPool struct {
		pool sync.Pool
	}
	vvPool struct {
		pool sync.Pool
	}
)

func NewSyncPool[T any]() *syncPool[T] {
	return &syncPool[T]{
		pool: sync.Pool{
			New: func() any {
				return new(T)
			},
		},
	}
}

func (p *syncPool[T]) Get() *T {
	return p.pool.Get().(*T)
}

func (p *syncPool[T]) Put(x *T) {
	if x != nil {
		p.pool.Put(x)
	}
}

func (p *stringBuilderPool) Get() *strings.Builder {
	return p.pool.Get().(*strings.Builder)
}

func (p *stringBuilderPool) Put(sb *strings.Builder) {
	sb.Reset()
	p.pool.Put(sb)
}

func (p *vvPool) Get() *VarVisitor {
	return p.pool.Get().(*VarVisitor)
}

func (p *vvPool) Put(vv *VarVisitor) {
	if vv != nil {
		vv.Clear()
		p.pool.Put(vv)
	}
}
