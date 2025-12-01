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

	// Slice pools for MarshalJSON operations
	// Custom pool for []map[string]any to properly clear maps before reuse
	mapStringAnySlicePool = &mapStringAnySlicePoolType{
		pool: sync.Pool{
			New: func() any {
				s := make([]map[string]any, 8)
				return &s
			},
		},
	}
	// Standard pool for [][2]*Term - no need for custom cleanup
	termPair2SlicePool = util.NewSlicePool[[2]*Term](8)

	// Pools for buildRequiredCapabilities temporary maps
	stringStructMapPool = &stringStructMapPoolType{
		pool: sync.Pool{
			New: func() any {
				m := make(map[string]struct{}, 16)
				return &m
			},
		},
	}

	// Pool for map[*Rule]struct{} used in GetRules
	ruleSetMapPool = &ruleSetMapPoolType{
		pool: sync.Pool{
			New: func() any {
				m := make(map[*Rule]struct{}, 16)
				return &m
			},
		},
	}

	// Pool for map[Var]*usedRef used in getGlobals
	varUsedRefMapPool = &varUsedRefMapPoolType{
		pool: sync.Pool{
			New: func() any {
				m := make(map[Var]*usedRef, 32)
				return &m
			},
		},
	}

	// Pool for unsafeVars (map[*Expr]VarSet) used in reorderBodyForSafety
	unsafeVarsMapPool = &unsafeVarsMapPoolType{
		pool: sync.Pool{
			New: func() any {
				m := make(unsafeVars, 16)
				return &m
			},
		},
	}

	// Pool for map[string]any used in MarshalJSON methods (Annotations, AnnotationsRef, etc.)
	mapStringAnyPool = &mapStringAnyPoolType{
		pool: sync.Pool{
			New: func() any {
				m := make(map[string]any, 8)
				return &m
			},
		},
	}
)

type (
	stringBuilderPool         struct{ pool sync.Pool }
	vvPool                    struct{ pool sync.Pool }
	mapStringAnySlicePoolType struct{ pool sync.Pool }
	stringStructMapPoolType   struct{ pool sync.Pool }
	ruleSetMapPoolType        struct{ pool sync.Pool }
	varUsedRefMapPoolType     struct{ pool sync.Pool }
	unsafeVarsMapPoolType     struct{ pool sync.Pool }
	mapStringAnyPoolType      struct{ pool sync.Pool }
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

func (p *mapStringAnySlicePoolType) Get(length int) *[]map[string]any {
	s := p.pool.Get().(*[]map[string]any)
	d := *s

	// Grow capacity if needed
	if cap(d) < length {
		// Need to allocate new slice with more capacity
		newSlice := make([]map[string]any, length)
		d = newSlice
	} else {
		d = d[:length]
	}

	// Clear each map in the slice and ensure we have fresh maps
	for i := range d {
		if d[i] == nil {
			d[i] = make(map[string]any, 4) // Pre-allocate small capacity
		} else {
			clear(d[i]) // Clear existing map
		}
	}

	*s = d
	return s
}

func (p *mapStringAnySlicePoolType) Put(s *[]map[string]any) {
	if s != nil {
		p.pool.Put(s)
	}
}

func (p *stringStructMapPoolType) Get() *map[string]struct{} {
	m := p.pool.Get().(*map[string]struct{})
	// Clear the map before returning
	clear(*m)
	return m
}

func (p *stringStructMapPoolType) Put(m *map[string]struct{}) {
	if m != nil {
		p.pool.Put(m)
	}
}

func (p *ruleSetMapPoolType) Get() *map[*Rule]struct{} {
	m := p.pool.Get().(*map[*Rule]struct{})
	clear(*m)
	return m
}

func (p *ruleSetMapPoolType) Put(m *map[*Rule]struct{}) {
	if m != nil {
		p.pool.Put(m)
	}
}

func (p *varUsedRefMapPoolType) Get() *map[Var]*usedRef {
	m := p.pool.Get().(*map[Var]*usedRef)
	clear(*m)
	return m
}

func (p *varUsedRefMapPoolType) Put(m *map[Var]*usedRef) {
	if m != nil {
		p.pool.Put(m)
	}
}

func (p *unsafeVarsMapPoolType) Get() *unsafeVars {
	m := p.pool.Get().(*unsafeVars)
	clear(*m)
	return m
}

func (p *unsafeVarsMapPoolType) Put(m *unsafeVars) {
	if m != nil {
		p.pool.Put(m)
	}
}

func (p *mapStringAnyPoolType) Get() *map[string]any {
	m := p.pool.Get().(*map[string]any)
	clear(*m)
	return m
}

func (p *mapStringAnyPoolType) Put(m *map[string]any) {
	if m != nil {
		p.pool.Put(m)
	}
}
