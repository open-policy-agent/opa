package ast

import (
	"strings"
	"sync"
)

type termPtrPool struct {
	pool sync.Pool
}

type stringBuilderPool struct {
	pool sync.Pool
}

type indexResultPool struct {
	pool sync.Pool
}

type vvPool struct {
	pool sync.Pool
}

func (p *termPtrPool) Get() *Term {
	return p.pool.Get().(*Term)
}

func (p *termPtrPool) Put(t *Term) {
	p.pool.Put(t)
}

func (p *stringBuilderPool) Get() *strings.Builder {
	return p.pool.Get().(*strings.Builder)
}

func (p *stringBuilderPool) Put(sb *strings.Builder) {
	sb.Reset()
	p.pool.Put(sb)
}

func (p *indexResultPool) Get() *IndexResult {
	return p.pool.Get().(*IndexResult)
}

func (p *indexResultPool) Put(x *IndexResult) {
	if x != nil {
		p.pool.Put(x)
	}
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

var TermPtrPool = &termPtrPool{
	pool: sync.Pool{
		New: func() any {
			return &Term{}
		},
	},
}

var sbPool = &stringBuilderPool{
	pool: sync.Pool{
		New: func() any {
			return &strings.Builder{}
		},
	},
}

var varVisitorPool = &vvPool{
	pool: sync.Pool{
		New: func() any {
			return NewVarVisitor()
		},
	},
}

var IndexResultPool = &indexResultPool{
	pool: sync.Pool{
		New: func() any {
			return &IndexResult{}
		},
	},
}
