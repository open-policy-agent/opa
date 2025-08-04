package transform

import "sync"

// FilterLogic is an interface that defines the logic for filtering objects.
type FilterLogic interface {
	Apply(key string, object any) bool
}

// FilterLogicFunc is a function type that implements the FilterLogic interface.
type FilterLogicFunc func(key string, object any) bool

func (f FilterLogicFunc) Apply(key string, object any) bool {
	return f(key, object)
}

// Filterable is an interface that must be implemented by objects that can be filtered.
type Filterable[T any] interface {
	// Keys returns the names of all fields in the object.
	Keys() []string

	// Clone returns a deep copy of the object.
	Clone() (T, error)

	// Remove removes a field from the object.
	Remove(string) error
}

// Apply is a standalone function that provides type-safe filtering based on
// specified filter logic.
//
// It returns a new object with only the fields that match the result of `logic.Apply`.
func Apply[T Filterable[T]](object T, logic FilterLogic) (T, error) {
	return filterWith(object, logic, true)
}

// Reject is a standalone function that provides type-safe filtering based on
// specified filter logic.
//
// It returns a new object with only the fields that DO NOT match the result
// of `logic.Apply`.
func Reject[T Filterable[T]](object T, logic FilterLogic) (T, error) {
	return filterWith(object, logic, false)
}

// filterWith is an internal function used by both Apply and Reject functions
// to apply the filtering logic to an object. If include is true, only fields
// matching the logic are included. If include is false, fields matching
// the logic are excluded.
func filterWith[T Filterable[T]](object T, logic FilterLogic, include bool) (T, error) {
	var zero T

	result, err := object.Clone()
	if err != nil {
		return zero, err
	}

	for _, k := range result.Keys() {
		if ok := logic.Apply(k, object); (include && ok) || (!include && !ok) {
			continue
		}

		if err := result.Remove(k); err != nil {
			return zero, err
		}
	}

	return result, nil
}

// NameBasedFilter is a filter that filters fields based on their field names.
type NameBasedFilter[T Filterable[T]] struct {
	names map[string]struct{}
	mu    sync.RWMutex
	logic FilterLogic
}

// NewNameBasedFilter creates a new NameBasedFilter with the specified field names.
//
// NameBasedFilter is the underlying implementation of the
// various filters in jwe, jwk, jws, and jwt packages. You normally do not
// need to use this directly.
func NewNameBasedFilter[T Filterable[T]](names ...string) *NameBasedFilter[T] {
	nameMap := make(map[string]struct{}, len(names))
	for _, name := range names {
		nameMap[name] = struct{}{}
	}

	nf := &NameBasedFilter[T]{
		names: nameMap,
	}

	nf.logic = FilterLogicFunc(nf.filter)
	return nf
}

func (nf *NameBasedFilter[T]) filter(k string, _ any) bool {
	_, ok := nf.names[k]
	return ok
}

// Filter returns a new object with only the fields that match the specified names.
func (nf *NameBasedFilter[T]) Filter(object T) (T, error) {
	nf.mu.RLock()
	defer nf.mu.RUnlock()

	return Apply(object, nf.logic)
}

// Reject returns a new object with only the fields that DO NOT match the specified names.
func (nf *NameBasedFilter[T]) Reject(object T) (T, error) {
	nf.mu.RLock()
	defer nf.mu.RUnlock()
	return Reject(object, nf.logic)
}
