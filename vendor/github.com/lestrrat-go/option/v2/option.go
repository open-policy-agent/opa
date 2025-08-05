package option

import (
	"fmt"

	"github.com/lestrrat-go/blackmagic"
)

// Interface defines the minimum interface that an option must fulfill
type Interface interface {
	// Ident returns the "identity" of this option, a unique identifier that
	// can be used to differentiate between options
	Ident() any

	// Value assigns the stored value into the dst argument, which must be
	// a pointer to a variable that can store the value. If the assignment
	// is successful, it return nil, otherwise it returns an error.
	Value(dst any) error
}

type pair[T any] struct {
	ident any
	value T
}

// New creates a new Option
func New[T any](ident any, value T) Interface {
	return &pair[T]{
		ident: ident,
		value: value,
	}
}

func (p *pair[T]) Ident() any {
	return p.ident
}

func (p *pair[T]) Value(dst any) error {
	if err := blackmagic.AssignIfCompatible(dst, p.value); err != nil {
		return fmt.Errorf("failed to assign value %T to %T: %s", p.value, dst, err)
	}
	return nil
}

func (p *pair[T]) String() string {
	return fmt.Sprintf(`%v(%v)`, p.ident, p.value)
}
