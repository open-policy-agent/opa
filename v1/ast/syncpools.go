package ast

import (
	"bytes"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/util"
)

var (
	TermPtrPool      = util.NewSyncPool[Term]()
	BytesReaderPool  = util.NewSyncPool[bytes.Reader]()
	StringReaderPool = util.NewSyncPool[strings.Reader]()
	IndexResultPool  = util.NewSyncPool[IndexResult]()

	// keysIteratorPool accessed via BorrowKeysIterator and ReturnKeysIterator functions.
	keysIteratorPool = util.NewResettablePool[objectKeysIterator]()

	// Needs custom pool because of custom Put logic.
	varVisitorPool = &vvPool{
		pool: sync.Pool{
			New: func() any {
				return NewVarVisitor()
			},
		},
	}
)

// BorrowKeysIterator retrieves an ObjectKeysIterator from the pool and initializes it with
// the given object, without the allocation cost that comes with the [Object.KeysIterator] method.
// Callers must return the iterator to the pool using [ReturnKeysIterator] when done.
func BorrowKeysIterator(obj Object) ObjectKeysIterator {
	ki := keysIteratorPool.Get()
	ki.index = 0
	ki.obj = objectFromObject(obj)
	ki.numKeys = len(ki.obj.keys)

	return ki
}

// ReturnKeysIterator returns provided ObjectKeysIterator to the pool.
func ReturnKeysIterator(it ObjectKeysIterator) {
	if oki, ok := it.(*objectKeysIterator); ok {
		keysIteratorPool.Put(oki)
	}
}

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
