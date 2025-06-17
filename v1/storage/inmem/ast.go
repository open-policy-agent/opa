package inmem

import (
	"fmt"
	"strconv"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
)

type updateAST struct {
	path   storage.Path // data path modified by update
	remove bool         // indicates whether update removes the value at path
	value  ast.Value    // value to add/replace at path (ignored if remove is true)
}

func (u *updateAST) Path() storage.Path {
	return u.path
}

func (u *updateAST) Remove() bool {
	return u.remove
}

func (u *updateAST) Set(v any) {
	if v, ok := v.(ast.Value); ok {
		u.value = v
	} else {
		panic("illegal value type") // FIXME: do conversion?
	}
}

func (u *updateAST) Value() any {
	return u.value
}

func (u *updateAST) Relative(path storage.Path) dataUpdate {
	cpy := *u
	cpy.path = cpy.path[len(path):]
	return &cpy
}

func (u *updateAST) Apply(v any) any {
	if len(u.path) == 0 {
		return u.value
	}

	data, ok := v.(ast.Value)
	if !ok {
		panic(fmt.Errorf("illegal value type %T, expected ast.Value", v))
	}

	if u.remove {
		newV, err := removeInAst(data, u.path)
		if err != nil {
			panic(err)
		}
		return newV
	}

	// If we're not removing, we're replacing (adds are turned into replaces during updateAST creation).
	newV, err := setInAst(data, u.path, u.value)
	if err != nil {
		panic(err)
	}
	return newV
}

func newUpdateAST(data any, op storage.PatchOp, path storage.Path, idx int, value ast.Value) (*updateAST, error) {

	switch data.(type) {
	case ast.Null, ast.Boolean, ast.Number, ast.String:
		return nil, NewNotFoundError(path)
	}

	switch data := data.(type) {
	case ast.Object:
		return newUpdateObjectAST(data, op, path, idx, value)

	case *ast.Array:
		return newUpdateArrayAST(data, op, path, idx, value)
	}

	return nil, &storage.Error{
		Code:    storage.InternalErr,
		Message: "invalid data value encountered",
	}
}

// Pool for reusing term slices
var termSlicePool = sync.Pool{
	New: func() interface{} {
		s := make([]*ast.Term, 0, 16)
		return &s
	},
}

func newUpdateArrayAST(data *ast.Array, op storage.PatchOp, path storage.Path, idx int, value ast.Value) (*updateAST, error) {
	if idx == len(path)-1 {
		if path[idx] == "-" || path[idx] == strconv.Itoa(data.Len()) {
			if op != storage.AddOp {
				return nil, invalidPatchError("%v: invalid patch path", path)
			}
			cpy := data.Append(ast.NewTerm(value))
			return &updateAST{path[:len(path)-1], false, cpy}, nil
		}

		pos, err := ValidateASTArrayIndex(data, path[idx], path)
		if err != nil {
			return nil, err
		}

		switch op {
		case storage.AddOp:
			resultsPtr := termSlicePool.Get().(*[]*ast.Term)
			*resultsPtr = (*resultsPtr)[:0] // Reset length

			desiredCap := data.Len() + 1
			if cap(*resultsPtr) < desiredCap {
				*resultsPtr = make([]*ast.Term, 0, desiredCap)
			}
			results := *resultsPtr


			for i := 0; i < data.Len(); i++ {
				if i == pos {
					results = append(results, ast.NewTerm(value))
				}
				results = append(results, data.Elem(i))
			}
			*resultsPtr = results

			// Create final slice of required size
			finalResults := make([]*ast.Term, len(*resultsPtr))
			copy(finalResults, *resultsPtr)
			termSlicePool.Put(resultsPtr)

			return &updateAST{path[:len(path)-1], false, ast.NewArray(finalResults...)}, nil

		case storage.RemoveOp:
			if data.Len() <= 1 {
				return &updateAST{path[:len(path)-1], false, ast.NewArray()}, nil
			}

			resultsPtr := termSlicePool.Get().(*[]*ast.Term)
			*resultsPtr = (*resultsPtr)[:0] // Reset length

			desiredCap := data.Len() - 1
			if cap(*resultsPtr) < desiredCap {
				*resultsPtr = make([]*ast.Term, 0, desiredCap)
			}
			results := *resultsPtr

			for i := 0; i < data.Len(); i++ {
				if i != pos {
					results = append(results, data.Elem(i))
				}
			}
			*resultsPtr = results

			finalResults := make([]*ast.Term, len(*resultsPtr))
			copy(finalResults, *resultsPtr)
			termSlicePool.Put(resultsPtr)

			return &updateAST{path[:len(path)-1], false, ast.NewArray(finalResults...)}, nil

		default: // ReplaceOp
			resultsPtr := termSlicePool.Get().(*[]*ast.Term)
			*resultsPtr = (*resultsPtr)[:0] // Reset length

			desiredCap := data.Len()
			if cap(*resultsPtr) < desiredCap {
				*resultsPtr = make([]*ast.Term, 0, desiredCap)
			}
			results := *resultsPtr

			for i := 0; i < data.Len(); i++ {
				if i == pos {
					results = append(results, ast.NewTerm(value))
				} else {
					results = append(results, data.Elem(i))
				}
			}
			*resultsPtr = results

			finalResults := make([]*ast.Term, len(*resultsPtr))
			copy(finalResults, *resultsPtr)
			termSlicePool.Put(resultsPtr)

			return &updateAST{path[:len(path)-1], false, ast.NewArray(finalResults...)}, nil
		}
	}

	pos, err := ValidateASTArrayIndex(data, path[idx], path)
	if err != nil {
		return nil, err
	}

	return newUpdateAST(data.Elem(pos).Value, op, path, idx+1, value)
}

func newUpdateObjectAST(data ast.Object, op storage.PatchOp, path storage.Path, idx int, value ast.Value) (*updateAST, error) {
	key := ast.InternedStringTerm(path[idx])
	val := data.Get(key)

	if idx == len(path)-1 {
		switch op {
		case storage.ReplaceOp, storage.RemoveOp:
			if val == nil {
				return nil, NewNotFoundError(path)
			}
		}
		return &updateAST{path, op == storage.RemoveOp, value}, nil
	}

	if val != nil {
		return newUpdateAST(val.Value, op, path, idx+1, value)
	}

	return nil, NewNotFoundError(path)
}

func interfaceToValue(v any) (ast.Value, error) {
	if v, ok := v.(ast.Value); ok {
		return v, nil
	}
	return ast.InterfaceToValue(v)
}

// setInAst updates the value in the AST at the given path with the given value.
// Values can only be replaced in arrays, not added.
// Values for new keys can be added to objects
func setInAst(data ast.Value, path storage.Path, value ast.Value) (ast.Value, error) {
	if len(path) == 0 {
		return data, nil
	}

	switch data := data.(type) {
	case ast.Object:
		return setInAstObject(data, path, value)
	case *ast.Array:
		return setInAstArray(data, path, value)
	default:
		return nil, fmt.Errorf("illegal value type %T, expected ast.Object or ast.Array", data)
	}
}

func setInAstObject(obj ast.Object, path storage.Path, value ast.Value) (ast.Value, error) {
	key := ast.InternedStringTerm(path[0])

	if len(path) == 1 {
		obj.Insert(key, ast.NewTerm(value))
		return obj, nil
	}

	child := obj.Get(key)
	newChild, err := setInAst(child.Value, path[1:], value)
	if err != nil {
		return nil, err
	}
	obj.Insert(key, ast.NewTerm(newChild))
	return obj, nil
}

func setInAstArray(arr *ast.Array, path storage.Path, value ast.Value) (ast.Value, error) {
	idx, err := strconv.Atoi(path[0])
	if err != nil {
		return nil, fmt.Errorf("illegal array index %v: %v", path[0], err)
	}

	if idx < 0 || idx >= arr.Len() {
		return arr, nil
	}

	if len(path) == 1 {
		arr.Set(idx, ast.NewTerm(value))
		return arr, nil
	}

	child := arr.Elem(idx)
	newChild, err := setInAst(child.Value, path[1:], value)
	if err != nil {
		return nil, err
	}
	arr.Set(idx, ast.NewTerm(newChild))
	return arr, nil
}

func removeInAst(value ast.Value, path storage.Path) (ast.Value, error) {
	if len(path) == 0 {
		return value, nil
	}

	switch value := value.(type) {
	case ast.Object:
		return removeInAstObject(value, path)
	case *ast.Array:
		return removeInAstArray(value, path)
	default:
		return nil, fmt.Errorf("illegal value type %T, expected ast.Object or ast.Array", value)
	}
}

func removeInAstObject(obj ast.Object, path storage.Path) (ast.Value, error) {
	key := ast.InternedStringTerm(path[0])

	if len(path) == 1 {
		if obj.Len() <= 1 {
			return ast.NewObject(), nil
		}

		// Optimized deletion with size pre-estimation
		items := make([][2]*ast.Term, 0, obj.Len()-1)
		obj.Foreach(func(k *ast.Term, v *ast.Term) {
			if !k.Equal(key) {
				items = append(items, [2]*ast.Term{k, v})
			}
		})
		return ast.NewObject(items...), nil
	}

	if child := obj.Get(key); child != nil {
		updatedChild, err := removeInAst(child.Value, path[1:])
		if err != nil {
			return nil, err
		}
		obj.Insert(key, ast.NewTerm(updatedChild))
	}

	return obj, nil
}

func removeInAstArray(arr *ast.Array, path storage.Path) (ast.Value, error) {
	idx, err := strconv.Atoi(path[0])
	if err != nil {
		return arr, nil // Path component is not an int, cannot be an array index.
	}

	if idx < 0 || idx >= arr.Len() {
		return arr, nil // Index out of bounds.
	}

	if len(path) == 1 {
		if arr.Len() <= 1 {
			return ast.NewArray(), nil
		}

		elemsPtr := termSlicePool.Get().(*[]*ast.Term)
		*elemsPtr = (*elemsPtr)[:0] // Reset length

		desiredCap := arr.Len() - 1
		if cap(*elemsPtr) < desiredCap {
			*elemsPtr = make([]*ast.Term, 0, desiredCap)
		}
		elems := *elemsPtr

		for i := 0; i < arr.Len(); i++ {
			if i != idx {
				elems = append(elems, arr.Elem(i))
			}
		}
		*elemsPtr = elems

		finalElems := make([]*ast.Term, len(*elemsPtr))
		copy(finalElems, *elemsPtr)
		termSlicePool.Put(elemsPtr)

		return ast.NewArray(finalElems...), nil
	}

	updatedChild, err := removeInAst(arr.Elem(idx).Value, path[1:])
	if err != nil {
		return nil, err
	}
	arr.Set(idx, ast.NewTerm(updatedChild))
	return arr, nil
}
