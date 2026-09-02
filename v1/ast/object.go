package ast

import (
	"slices"
	"sync"

	"github.com/open-policy-agent/opa/v1/util"
)

var ObjectBuilderPool = &util.SyncPool[ObjectBuilder]{
	Pool: sync.Pool{
		New: func() any {
			return newObjectBuilder(32)
		},
	},
}

// ObjectBuilder is a builder to help building ast.Object values without having to step through intermediate
// formats and conversions, and with things like resource pooling and interning of keys handled conveniently.
// While ast.Object's support other key types than strings, this builder currently doesn't.
//
// NOTE that this is a helper intended for internal use. While anyone is welcome to use it, the API is not
// considered part of the public API contract, and can change without notice.
type ObjectBuilder struct {
	pairs    [][2]*Term
	keyMapFn func(string) string
	valMapFn func(*Term) *Term
}

func newObjectBuilder(size int) *ObjectBuilder {
	return &ObjectBuilder{pairs: make([][2]*Term, 0, size)}
}

// MapToObject returns an Object mapped from m where an optional key mapper is used to transform
// the keys before they're made to (interned) terms, and a mandatory value mapper to transform
// generic values to terms.
func MapToObject[V any](m map[string]V, keyFn func(string) string, valueFn func(V) *Term) Object {
	ob := ObjectBuilderPool.Get().Reset().WithKeyMapper(keyFn)
	defer ObjectBuilderPool.Put(ob)

	for k, v := range m {
		ob.Item(k, valueFn(v))
	}
	return ob.AsObject()
}

func TryMapToObject[V any](m map[string]V, keyFn func(string) string, valueFn func(V) (*Term, error)) (Object, error) {
	ob := ObjectBuilderPool.Get().Reset().WithKeyMapper(keyFn)
	defer ObjectBuilderPool.Put(ob)

	for k, v := range m {
		t, err := valueFn(v)
		if err != nil {
			return nil, err
		}
		ob.Item(k, t)
	}
	return ob.AsObject(), nil
}

// WithKeyMapper sets a key mapping function to be used when adding items to the builder.
func (b *ObjectBuilder) WithKeyMapper(fn func(string) string) *ObjectBuilder {
	b.keyMapFn = fn
	return b
}

// WithValueMapper sets a value mapping function to be used when adding items to the builder.
func (b *ObjectBuilder) WithValueMapper(fn func(*Term) *Term) *ObjectBuilder {
	b.valMapFn = fn
	return b
}

// Grow ensures that the builder has capacity for at least n additional items, and returns the builder.
func (b *ObjectBuilder) Grow(n int) *ObjectBuilder {
	if b.pairs == nil {
		b.pairs = make([][2]*Term, 0, n)
	} else if cap(b.pairs)-len(b.pairs) < n {
		b.pairs = slices.Grow(b.pairs, n)
	}
	return b
}

// Item adds a key-value pair to the builder, where the key is a string (interned as term) and the value as term.
func (b *ObjectBuilder) Item(key string, value *Term) *ObjectBuilder {
	k, v := b.mapKey(key), b.mapValue(value)
	if k == nil || v == nil {
		return b
	}
	b.pairs = append(b.pairs, [2]*Term{k, v})
	return b
}

// AsObject builds and returns an ast.Object.
func (b *ObjectBuilder) AsObject() Object {
	return NewObject(b.pairs...)
}

// AsTerm builds and returns an ast.Object contained in an ast.Term for convenience.
func (b *ObjectBuilder) AsTerm() *Term {
	return NewTerm(b.AsObject())
}

// Reset resets the builder to an empty state, but keeps the underlying slice for reuse.
func (b *ObjectBuilder) Reset() *ObjectBuilder {
	b.pairs = b.pairs[:0]
	b.keyMapFn = nil
	b.valMapFn = nil
	return b
}

func (b *ObjectBuilder) mapKey(key string) *Term {
	if b.keyMapFn != nil {
		key = b.keyMapFn(key)
	}
	return InternedTerm(key)
}

func (b *ObjectBuilder) mapValue(value *Term) *Term {
	if b.valMapFn != nil {
		value = b.valMapFn(value)
	}
	return value
}

// InterfaceToTermMapper works similarly to [InterfaceToValue], but returns a term instead of a value,
// and tries to find an interned version of the term if possible.
func InterfaceToTermMapper(x any) (*Term, error) {
	switch v := x.(type) {
	case *Term:
		return v, nil
	case Value:
		// TODO: Find interned term from value
		return NewTerm(v), nil
	case bool:
		return InternedTerm(v), nil
	case string:
		return InternedTerm(v), nil
	case int:
		return InternedTerm(v), nil
	case int8:
		return InternedTerm(v), nil
	case int16:
		return InternedTerm(v), nil
	case int32:
		return InternedTerm(v), nil
	case int64:
		return InternedTerm(v), nil
	case uint:
		return InternedTerm(v), nil
	case uint8:
		return InternedTerm(v), nil
	case uint16:
		return InternedTerm(v), nil
	case uint32:
		return InternedTerm(v), nil
	case uint64:
		return InternedTerm(v), nil
	default:
		x, err := InterfaceToValue(v)
		if err != nil {
			return nil, err
		}
		return NewTerm(x), nil // TODO: InternedTerm from value
	}

}
