package ast

import (
	"bytes"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/util"
)

var (
	TermPtrPool     = util.NewSyncPool[Term]()
	BytesReaderPool = util.NewSyncPool[bytes.Reader]()
	IndexResultPool = util.NewSyncPool[IndexResult]()
	bbPool          = util.NewSyncPool[bytes.Buffer]()
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
	stringBuilderPool struct{ pool sync.Pool }
	vvPool            struct{ pool sync.Pool }
)

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
