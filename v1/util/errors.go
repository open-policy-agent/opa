package util

import "errors"

// ErrorIs is a shorthand for [errors.AsType] returning only the boolean result.
// This is typically cheaper than [errors.Is], but it's not a drop-in replacement
// for scenario where e.g. custom `Is` methods need to be taken into account.
func ErrorIs[E error](err error) bool {
	_, ok := errors.AsType[E](err)
	return ok
}
