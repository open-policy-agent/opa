// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
)

// RootPath refers to the root document in storage.
var RootPath = Path{}

// Path refers to a document in storage.
type Path []string

// ParsePath returns a new path for the given str.
func ParsePath(str string) (path Path, ok bool) {
	if len(str) == 0 || str[0] != '/' {
		return nil, false
	}
	if len(str) == 1 {
		return Path{}, true
	}

	return strings.Split(str[1:], "/"), true
}

// ParsePathEscaped returns a new path for the given escaped str.
func ParsePathEscaped(str string) (path Path, ok bool) {
	if path, ok = ParsePath(str); ok {
		for i := range path {
			if segment, err := url.PathUnescape(path[i]); err == nil {
				path[i] = segment
			} else {
				return nil, false
			}
		}
	}
	return
}

// NewPathForRef returns a new path for the given ref.
func NewPathForRef(ref ast.Ref) (path Path, err error) {
	if len(ref) == 0 {
		return nil, errors.New("empty reference (indicates error in caller)")
	}

	if len(ref) == 1 {
		return Path{}, nil
	}

	path = make(Path, 0, len(ref)-1)

	for _, term := range ref[1:] {
		switch v := term.Value.(type) {
		case ast.String:
			path = append(path, string(v))
		case ast.Number:
			path = append(path, v.String())
		case ast.Boolean, ast.Null:
			return nil, &Error{
				Code:    NotFoundErr,
				Message: fmt.Sprintf("%v: does not exist", ref),
			}
		case *ast.Array, ast.Object, ast.Set:
			return nil, fmt.Errorf("composites cannot be base document keys: %v", ref)
		default:
			return nil, fmt.Errorf("unresolved reference (indicates error in caller): %v", ref)
		}
	}

	return path, nil
}

// Compare performs lexigraphical comparison on p and other and returns -1 if p
// is less than other, 0 if p is equal to other, or 1 if p is greater than
// other.
func (p Path) Compare(other Path) (cmp int) {
	return slices.Compare(p, other)
}

// Equal returns true if p is the same as other.
func (p Path) Equal(other Path) bool {
	return slices.Equal(p, other)
}

// HasPrefix returns true if p starts with other.
func (p Path) HasPrefix(other Path) bool {
	return len(other) <= len(p) && p[:len(other)].Equal(other)
}

// Ref returns a ref that represents p rooted at head.
func (p Path) Ref(head *ast.Term) (ref ast.Ref) {
	ref = make(ast.Ref, len(p)+1)
	ref[0] = head
	for i := range p {
		idx, err := strconv.ParseInt(p[i], 10, 64)
		if err == nil {
			ref[i+1] = ast.UIntNumberTerm(uint64(idx))
		} else {
			ref[i+1] = ast.StringTerm(p[i])
		}
	}
	return ref
}

func (p Path) String() string {
	if len(p) == 0 {
		return "/"
	}

	l := 0
	for i := range p {
		l += len(p[i]) + 1
	}

	sb := strings.Builder{}
	sb.Grow(l)
	for i := range p {
		sb.WriteByte('/')
		sb.WriteString(url.PathEscape(p[i]))
	}
	return sb.String()
}

// MustParsePath returns a new Path for s. If s cannot be parsed, this function
// will panic. This is mostly for test purposes.
func MustParsePath(s string) Path {
	path, ok := ParsePath(s)
	if !ok {
		panic(s)
	}
	return path
}
