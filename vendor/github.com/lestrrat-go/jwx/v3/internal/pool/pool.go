package pool

import (
	"sync"
)

type Pool[T any] struct {
	pool       sync.Pool
	destructor func(T) T
}

// New creates a new Pool instance for the type T.
// The allocator function is used to create new instances of T when the pool is empty.
// The destructor function is used to clean up instances of T before they are returned to the pool.
// The destructor should reset the state of T to a clean state, so it can be reused, and
// return the modified instance of T. This is required for cases when you reset operations
// can modify the underlying data structure, such as slices or maps.
func New[T any](allocator func() T, destructor func(T) T) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any {
				return allocator()
			},
		},
		destructor: destructor,
	}
}

// Get retrieves an item of type T from the pool.
func (p *Pool[T]) Get() T {
	//nolint:forcetypeassert
	return p.pool.Get().(T)
}

// Put returns an item of type T to the pool.
// The item is first processed by the destructor function to ensure it is in a clean state.
func (p *Pool[T]) Put(item T) {
	p.pool.Put(p.destructor(item))
}

// SlicePool is a specialized pool for slices of type T. It is identical to Pool[T] but
// provides additional functionality to get slices with a specific capacity.
type SlicePool[T any] struct {
	pool *Pool[[]T]
}

func NewSlicePool[T any](allocator func() []T, destructor func([]T) []T) SlicePool[T] {
	return SlicePool[T]{
		pool: New(allocator, destructor),
	}
}

func (p SlicePool[T]) Get() []T {
	return p.pool.Get()
}

func (p SlicePool[T]) GetCapacity(capacity int) []T {
	if capacity <= 0 {
		return p.Get()
	}
	s := p.Get()
	if cap(s) < capacity {
		s = make([]T, 0, capacity)
	}
	return s
}

func (p SlicePool[T]) Put(s []T) {
	p.pool.Put(s)
}
