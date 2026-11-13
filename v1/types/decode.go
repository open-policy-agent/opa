// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package types

import (
	"errors"
	"fmt"

	"github.com/open-policy-agent/opa/v1/util"
)

const (
	typeNull      = "null"
	typeBoolean   = "boolean"
	typeNumber    = "number"
	typeString    = "string"
	typeArray     = "array"
	typeSet       = "set"
	typeObject    = "object"
	typeAny       = "any"
	typeFunction  = "function"
	typeRecursive = "recursive"
)

// Unmarshal deserializes bs and returns the resulting type.
func Unmarshal[T byte, BS ~[]byte](bs BS) (result Type, err error) {
	var x any
	if err := util.UnmarshalJSON(bs, &x); err != nil {
		return nil, err
	}
	// Decode once and walk the result: re-decoding the raw bytes of each nested
	// type would be quadratic in the nesting depth.
	return unmarshalType(x)
}

func unmarshalType(x any) (Type, error) {
	if x == nil {
		return nil, errors.New("unsupported type ''")
	}
	m, ok := x.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid type: expected object, got %T", x)
	}
	tpe, ok := m["type"].(string)
	if !ok && m["type"] != nil {
		return nil, fmt.Errorf("invalid type field: expected string, got %T", m["type"])
	}

	switch tpe {
	case typeNull:
		return Nl, nil
	case typeBoolean:
		return B, nil
	case typeNumber:
		return N, nil
	case typeString:
		return S, nil
	case typeArray:
		static, err := unmarshalTypeSlice(m, "static")
		if err != nil {
			return nil, err
		}
		var dynamic Type
		if d, ok := m["dynamic"]; ok {
			if dynamic, err = unmarshalType(d); err != nil {
				return nil, err
			}
		}
		return NewArray(static, dynamic), nil
	case typeObject:
		static, err := unmarshalStaticPropertySlice(m["static"])
		if err != nil {
			return nil, err
		}
		dynamic, err := unmarshalDynamicProperty(m["dynamic"])
		if err != nil {
			return nil, err
		}
		return NewObject(static, dynamic), nil
	case typeSet:
		// A set of unknown elements, like the type of set(), is written
		// without "of".
		var of Type
		if x, ok := m["of"]; ok {
			var err error
			if of, err = unmarshalType(x); err != nil {
				return nil, err
			}
		}
		return NewSet(of), nil
	case typeAny:
		of, err := unmarshalTypeSlice(m, "of")
		if err != nil {
			return nil, err
		}
		return NewAny(of...), nil
	case typeFunction:
		args, err := unmarshalTypeSlice(m, "args")
		if err != nil {
			return nil, err
		}
		var ret Type
		if r, ok := m["result"]; ok {
			if ret, err = unmarshalType(r); err != nil {
				return nil, err
			}
		}
		if v, ok := m["variadic"]; ok {
			varargs, err := unmarshalType(v)
			if err != nil {
				return nil, err
			}
			return NewVariadicFunction(args, varargs, ret), nil
		}
		return NewFunction(args, ret), nil
	default:
		return nil, fmt.Errorf("unsupported type '%v'", tpe)
	}
}

func unmarshalRequiredType(m map[string]any, key string) (Type, error) {
	x, ok := m[key]
	if !ok {
		return nil, fmt.Errorf("missing %q field", key)
	}
	return unmarshalType(x)
}

func unmarshalTypeSlice(m map[string]any, key string) ([]Type, error) {
	if m[key] == nil {
		return nil, nil
	}
	elems, ok := m[key].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid %s field: expected array, got %T", key, m[key])
	}
	return util.TryMap(elems, unmarshalType)
}

func unmarshalStaticPropertySlice(x any) ([]*StaticProperty, error) {
	if x == nil {
		return []*StaticProperty{}, nil
	}
	elems, ok := x.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid static field: expected array, got %T", x)
	}
	result := make([]*StaticProperty, len(elems))
	for i := range elems {
		elem, ok := elems[i].(map[string]any)
		if !ok {
			if elems[i] != nil {
				return nil, fmt.Errorf("invalid static property: expected object, got %T", elems[i])
			}
			elem = map[string]any{}
		}
		value, err := unmarshalRequiredType(elem, "value")
		if err != nil {
			return nil, err
		}
		result[i] = NewStaticProperty(elem["key"], value)
	}
	return result, nil
}

func unmarshalDynamicProperty(x any) (*DynamicProperty, error) {
	if x == nil {
		return nil, nil
	}
	m, ok := x.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid dynamic field: expected object, got %T", x)
	}
	k, ok := m["key"]
	if !ok {
		return nil, nil
	}
	key, err := unmarshalType(k)
	if err != nil {
		return nil, err
	}
	value, err := unmarshalRequiredType(m, "value")
	if err != nil {
		return nil, err
	}
	return NewDynamicProperty(key, value), nil
}
