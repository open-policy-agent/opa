package inmem

import (
	"strconv"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
)

func Ptr(data any, path storage.Path) (any, error) {
	node := data
	for i := range path {
		key := path[i]
		switch curr := node.(type) {
		case map[string]any:
			var ok bool
			if node, ok = curr[key]; !ok {
				return nil, NewNotFoundError(path)
			}
		case []any:
			pos, err := ValidateArrayIndex(curr, key, path)
			if err != nil {
				return nil, err
			}
			node = curr[pos]
		default:
			return nil, NewNotFoundError(path)
		}
	}

	return node, nil
}

func ValuePtr(data ast.Value, path storage.Path) (ast.Value, error) {
	node := data
	for i := range path {
		key := path[i]
		switch curr := node.(type) {
		case ast.Object:
			// Use InternedStringTerm to leverage OPA's string interning.
			keyTerm := ast.InternedStringTerm(key)

			val := curr.Get(keyTerm)
			if val == nil {
				return nil, NewNotFoundError(path)
			}
			node = val.Value
		case *ast.Array:
			pos, err := ValidateASTArrayIndex(curr, key, path)
			if err != nil {
				return nil, err
			}
			node = curr.Elem(pos).Value
		default:
			return nil, NewNotFoundError(path)
		}
	}

	return node, nil
}

func ValidateArrayIndex(arr []any, s string, path storage.Path) (int, error) {
	idx, ok := isInt(s)
	if !ok {
		return 0, NewNotFoundErrorWithHint(path, ArrayIndexTypeMsg)
	}
	return inRange(idx, arr, path)
}

func ValidateASTArrayIndex(arr *ast.Array, s string, path storage.Path) (int, error) {
	idx, ok := isInt(s)
	if !ok {
		return 0, NewNotFoundErrorWithHint(path, ArrayIndexTypeMsg)
	}
	return inRange(idx, arr, path)
}

// ValidateArrayIndexForWrite also checks that `s` is a valid way to address an
// array element like `ValidateArrayIndex`, but returns a `resource_conflict` error
// if it is not.
func ValidateArrayIndexForWrite(arr []any, s string, i int, path storage.Path) (int, error) {
	idx, ok := isInt(s)
	if !ok {
		return 0, NewWriteConflictError(path[:i-1])
	}
	return inRange(idx, arr, path)
}

func isInt(s string) (int, bool) {
	idx, err := strconv.Atoi(s)
	return idx, err == nil
}

func inRange(i int, arr any, path storage.Path) (int, error) {

	var arrLen int

	switch v := arr.(type) {
	case []any:
		arrLen = len(v)
	case *ast.Array:
		arrLen = v.Len()
	}

	if i < 0 || i >= arrLen {
		return 0, NewNotFoundErrorWithHint(path, OutOfRangeMsg)
	}
	return i, nil
}
