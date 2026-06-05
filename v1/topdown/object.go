// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"cmp"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func builtinObjectUnion(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	objA, err := builtins.ObjectOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	objB, err := builtins.ObjectOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	if objA.Len() == 0 {
		return iter(operands[1])
	}
	if objB.Len() == 0 || objA.Compare(objB) == 0 {
		return iter(operands[0])
	}

	return iter(ast.NewTerm(mergeWithOverwrite(objA, objB)))
}

func builtinObjectUnionN(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	arr, err := builtins.ArrayOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	n := arr.Len()
	if n == 0 {
		return iter(ast.InternedEmptyObject)
	}

	first := arr.Elem(0)
	obj, ok := first.Value.(ast.Object)
	if !ok {
		return builtins.NewOperandElementErr(1, arr, first.Value, "object")
	}

	if n == 1 {
		return iter(first)
	}

	// Because we need merge-with-overwrite behavior, we can iterate
	// back-to-front, and get a mostly correct set of key assignments that
	// give us the "last assignment wins, with merges" behavior we want.
	// However, if a non-object overwrites an object value anywhere in the
	// chain of assignments for a key, we have to "freeze" that key to
	// prevent accidentally picking up nested objects that could merge with
	// it from earlier in the input array.
	// Example:
	//   Input: [{"a": {"b": 2}}, {"a": 4}, {"a": {"c": 3}}]
	//   Want Output: {"a": {"c": 3}}

	// First pass: count total keys for pre-allocation
	totalSize := obj.Len()
	for i := 1; i < n; i++ {
		elem := arr.Elem(i)
		o, ok := elem.Value.(ast.Object)
		if !ok {
			return builtins.NewOperandElementErr(1, arr, elem.Value, "object")
		}
		totalSize += o.Len()
	}

	result := ast.NewObjectWithCapacity(totalSize)
	frozenKeys := make(map[*ast.Term]struct{}, totalSize)

	for i := n - 1; i >= 0; i-- {
		if o := arr.Elem(i).Value.(ast.Object); o.Len() > 0 {
			mergewithOverwriteInPlace(result, o, frozenKeys)
		}
	}

	return iter(ast.NewTerm(result))
}

func builtinObjectRemove(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// Expect an object and an array/set/object of keys
	obj, err := builtins.ObjectOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	// Build a set of keys to remove
	keysToRemove, err := getObjectKeysParam(operands[1].Value)
	if err != nil {
		return err
	}

	// Pre-allocate with obj size (upper bound for result)
	r := ast.NewObjectWithCapacity(obj.Len())
	obj.Foreach(func(key *ast.Term, value *ast.Term) {
		if !keysToRemove.Contains(key) {
			r.Insert(key, value)
		}
	})

	return iter(ast.NewTerm(r))
}

func builtinObjectFilter(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// Expect an object and an array/set/object of keys
	obj, err := builtins.ObjectOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	// Build a new object from the supplied filter keys
	keys, err := getObjectKeysParam(operands[1].Value)
	if err != nil {
		return err
	}

	// Pre-allocate with keys size (upper bound for filter object)
	filterObj := ast.NewObjectWithCapacity(keys.Len())
	keys.Foreach(func(key *ast.Term) {
		filterObj.Insert(key, ast.InternedNullTerm)
	})

	// Actually do the filtering
	r, err := obj.Filter(filterObj)
	if err != nil {
		return err
	}

	return iter(ast.NewTerm(r))
}

func builtinObjectGet(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// silly micro optimization: initial ref to last item avoids
	// later bounds checks as 1 and 0 then known to be valid indices
	defaultValue, path, curr := operands[2], operands[1], operands[0]

	object, err := builtins.ObjectOperand(curr.Value, 1)
	if err != nil {
		return err
	}

	arr, ok := path.Value.(*ast.Array)
	if !ok {
		return iter(cmp.Or(object.Get(path), defaultValue))
	}

	for i := range arr.Len() {
		if curr = curr.Get(arr.Elem(i)); curr == nil {
			break
		}
	}

	return iter(cmp.Or(curr, defaultValue))
}

func builtinObjectKeys(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	object, err := builtins.ObjectOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}
	if object.Len() == 0 {
		return iter(ast.InternedEmptySet)
	}

	return iter(ast.SetTerm(object.Keys()...))
}

// getObjectKeysParam returns a set of key values
// from a supplied ast array, object, set value.
// The returned set must not be mutated. For Set
// inputs, it may be the original.
func getObjectKeysParam(arrayOrSet ast.Value) (ast.Set, error) {
	switch v := arrayOrSet.(type) {
	case *ast.Array:
		keys := ast.NewSetWithCapacity(v.Len())
		v.Foreach(keys.Add)
		return keys, nil
	case ast.Set:
		// Return directly. Callers only use this for Contains() checks
		// without mutating the set.
		return v, nil
	case ast.Object:
		return ast.NewSet(v.Keys()...), nil
	}

	return nil, builtins.NewOperandTypeErr(2, arrayOrSet, "object", "set", "array")
}

func mergeWithOverwrite(objA, objB ast.Object) ast.Object {
	merged, _ := objA.MergeWith(objB, func(v1, v2 *ast.Term) (*ast.Term, bool) {
		originalValueObj, ok2 := v1.Value.(ast.Object)
		updateValueObj, ok1 := v2.Value.(ast.Object)
		if !ok1 || !ok2 {
			// If we can't merge, stick with the right-hand value
			return v2, false
		}

		// Recursively update the existing value
		merged := mergeWithOverwrite(originalValueObj, updateValueObj)
		return ast.NewTerm(merged), false
	})
	return merged
}

// Modifies obj with any new keys from other, and recursively
// merges any keys where the values are both objects.
func mergewithOverwriteInPlace(dst, src ast.Object, frozenKeys map[*ast.Term]struct{}) {
	if src.Len() == 0 {
		return
	}

	src.Foreach(func(k, v *ast.Term) {
		if v2 := dst.Get(k); v2 == nil {
			// key not in dst, insert from src
			dst.Insert(k, copyIfObject(v))
		} else {
			// key in both, merge or reject change
			srcObj, ok2 := v.Value.(ast.Object)
			dstObj, ok1 := v2.Value.(ast.Object)
			// both are objects? Merge recursively.
			if ok1 && ok2 {
				// Check to make sure that this key isn't frozen before merging.
				if _, ok := frozenKeys[v2]; !ok {
					mergewithOverwriteInPlace(dstObj, srcObj, frozenKeys)
				}
			} else {
				// Else, original value wins. Freeze the key.
				frozenKeys[v2] = struct{}{}
			}
		}
	})
}

// copyIfObject returns term in which objects are copied recursively
// other values are returned as-is. This is much cheaper than .Copy()
// and sufficient for the use case of merging, as sets and arrays are
// overwritten rather than merged.
func copyIfObject(term *ast.Term) *ast.Term {
	switch val := term.Value.(type) {
	case ast.Object:
		cpy, _ := val.Map(func(k, v *ast.Term) (*ast.Term, *ast.Term, error) {
			return k, copyIfObject(v), nil
		})
		return ast.NewTerm(cpy)
	default:
		return term
	}
}

func init() {
	RegisterBuiltinFunc(ast.ObjectUnion.Name, builtinObjectUnion)
	RegisterBuiltinFunc(ast.ObjectUnionN.Name, builtinObjectUnionN)
	RegisterBuiltinFunc(ast.ObjectRemove.Name, builtinObjectRemove)
	RegisterBuiltinFunc(ast.ObjectFilter.Name, builtinObjectFilter)
	RegisterBuiltinFunc(ast.ObjectGet.Name, builtinObjectGet)
	RegisterBuiltinFunc(ast.ObjectKeys.Name, builtinObjectKeys)
}
