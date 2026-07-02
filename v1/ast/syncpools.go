package ast

import (
	"bytes"
	"sync"

	"github.com/open-policy-agent/opa/v1/util"
)

var (
	TermPtrPool     = util.NewSyncPool[Term]()
	BytesReaderPool = util.NewSyncPool[bytes.Reader]()
	IndexResultPool = util.NewSyncPool[IndexResult]()

	// Needs custom pool because of custom Put logic.
	varVisitorPool = &vvPool{
		pool: sync.Pool{
			New: func() any {
				return NewVarVisitor()
			},
		},
	}
)

type vvPool struct {
	pool sync.Pool
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
