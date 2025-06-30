package pool

import (
	"sync"
)

type Pool[T any] struct {
	pool       sync.Pool
	destructor func(T)
}

func New[T any](allocator func() interface{}, destructor func(T)) *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				return allocator()
			},
		},
		destructor: destructor,
	}
}

func (p *Pool[T]) Get() T {
	//nolint:forcetypeassert
	return p.pool.Get().(T)
}

func (p *Pool[T]) Put(item T) {
	p.destructor(item)
	p.pool.Put(item)
}
