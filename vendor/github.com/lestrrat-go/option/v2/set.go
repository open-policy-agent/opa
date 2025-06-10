package option

import (
	"sync"
)

// Set is a container to store multiple options. Because options are
// usually used all over the place to configure various aspects of
// a system, it is often useful to be able to collect multiple options
// together and pass them around as a single entity.
//
// Note that Set is meant to be add-only; You usually do not remove
// options from a Set.
//
// The intention is to create a set using a sync.Pool; we would like
// to provide a centralized pool of Sets so that you don't need to
// instantiate a new pool for every type of option you want to
// store, but that is not quite possible because of the limitations
// of parameterized types in Go. Instead create a `*option.SetPool`
// with an appropriate type parameter and allocator.
type Set[T Interface] struct {
	mu      sync.RWMutex
	options []T
}

func NewSet[T Interface]() *Set[T] {
	return &Set[T]{
		options: make([]T, 0, 1),
	}
}

func (s *Set[T]) Add(opt T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.options = append(s.options, opt)
}

func (s *Set[T]) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.options = s.options[:0] // Reset the options slice to avoid memory leaks
}

func (s *Set[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.options)
}

func (s *Set[T]) Option(i int) T {
	var zero T
	s.mu.RLock()
	defer s.mu.RUnlock()
	if i < 0 || i >= len(s.options) {
		return zero
	}
	return s.options[i]
}

// List returns a slice of all options stored in the Set.
// Note that the slice is the same slice that is used internally, so
// you should not modify the contents of the slice directly.
// This to avoid unnecessary allocations and copying of the slice for
// performance reasons.
func (s *Set[T]) List() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.options
}

// SetPool is a pool of Sets that can be used to efficiently manage
// the lifecycle of Sets. It uses a sync.Pool to store and retrieve
// Sets, allowing for efficient reuse of memory and reducing the
// number of allocations required when creating new Sets.
type SetPool[T Interface] struct {
	pool *sync.Pool // sync.Pool that contains *Set[T]
}

func NewSetPool[T Interface](pool *sync.Pool) *SetPool[T] {
	return &SetPool[T]{
		pool: pool,
	}
}

func (p *SetPool[T]) Get() *Set[T] {
	return p.pool.Get().(*Set[T])
}

func (p *SetPool[T]) Put(s *Set[T]) {
	s.Reset()
	p.pool.Put(s)
}
