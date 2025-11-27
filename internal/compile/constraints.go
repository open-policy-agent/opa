// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package compile

import (
	"errors"
	"fmt"
	"maps"
	"strings"
)

type Set[T comparable] map[T]struct{}

func NewSet[T comparable](strs ...T) Set[T] {
	return make(Set[T]).Add(strs...)
}

func (s Set[T]) Clone() Set[T] {
	return maps.Clone(s)
}

func (s Set[T]) Add(strs ...T) Set[T] {
	for i := range strs {
		s[strs[i]] = struct{}{}
	}
	return s
}

// Contains checks if a string exists in the set
func (s Set[T]) Contains(str T) bool {
	_, exists := s[str]
	return exists
}

// Intersection returns a new set containing elements present in both sets
func (s Set[T]) Intersection(other Set[T]) Set[T] {
	result := NewSet[T]()
	// Iterate through the smaller set for better performance
	if len(s) > len(other) {
		s, other = other, s
	}
	for elem := range s {
		if other.Contains(elem) {
			result.Add(elem)
		}
	}
	return result
}

// Constraint lets us limit the Set that are allowed in a translation.
// There are hardcoded sets of supported Set. The constraints become
// effective during the post-PE analysis (compile.Checks()).
type Constraint struct {
	Target   string
	Variant  string
	Builtins Set[string]
	Features Set[string]
}

type Constraints interface {
	Builtin(string) bool
	AssertBuiltin(string) error
	Supports(string) bool
	AssertFeature(string) error
}

// NewConstraints returns a new Constraint object based on the type
// requested, ucast or sql.
func NewConstraints(typ, variant string) (*Constraint, error) {
	c := Constraint{Target: strings.ToUpper(typ), Variant: variant, Features: NewSet[string]()}
	switch typ {
	case "sql":
		switch v := strings.ToLower(variant); v {
		case "sqlite":
			c.Builtins = sqlSQLiteBuiltins
		case "mysql", "postgresql", "sqlserver", "sqlite-internal":
			c.Builtins = sqlBuiltins
		default:
			return nil, fmt.Errorf("unsupported variant for %s: %s", typ, variant)
		}
		c.Features.Add("not", "field-ref", "existence-ref")
	case "ucast":
		switch v := strings.ToLower(variant); v {
		case "all":
			c.Variant = v
			c.Features.Add("not", "field-ref")
			c.Builtins = allBuiltins
		case "prisma":
			c.Variant = v
			c.Features.Add("not", "existence-ref")
			c.Builtins = allBuiltins
		case "linq":
			c.Variant = "LINQ" // normalize spelling
			c.Builtins = ucastLINQBuiltins
		default:
			c.Variant = ""
			c.Builtins = ucastBuiltins
		}
	default:
		return nil, fmt.Errorf("unknown target/dialect combination: %s/%s", typ, variant)
	}

	return &c, nil
}

// Builtin returns true if the builtin is supported by the constraint.
func (c *Constraint) Builtin(x string) bool {
	return c.Builtins.Contains(x)
}

func (c *Constraint) AssertBuiltin(x string) error {
	if !c.Builtin(x) {
		return fmt.Errorf("unsupported for %s", c)
	}
	return nil
}

// Supports allows us to encode more fluent constraints, like support for "not".
// It returns an error if the feature is not supported by the constraint.
func (c *Constraint) Supports(x string) bool {
	return c.Features.Contains(x)
}

func (c *Constraint) AssertFeature(x string) error {
	if !c.Supports(x) {
		return fmt.Errorf("unsupported feature %q for %s", x, c)
	}
	return nil
}

var _ fmt.Stringer = (*Constraint)(nil)

func (c *Constraint) String() string {
	if c.Variant == "" {
		return c.Target
	}
	return fmt.Sprintf("%s (%s)", c.Target, c.Variant)
}

type ConstraintSet struct {
	Constraints []*Constraint
}

func NewConstraintSet(cs ...*Constraint) *ConstraintSet {
	return &ConstraintSet{Constraints: cs}
}

// Builtin returns true if all the constraints in the set support the builtin.
func (cs *ConstraintSet) Builtin(x string) bool {
	for i := range cs.Constraints {
		if !cs.Constraints[i].Builtin(x) {
			return false
		}
	}
	return true
}

func (cs *ConstraintSet) AssertBuiltin(x string) error {
	var err error
	for i := range cs.Constraints {
		if e := cs.Constraints[i].AssertBuiltin(x); e != nil {
			err = errors.Join(err, e)
		}
	}
	return err
}

// Supports returns true if the feature is supported by all constraints.
func (cs *ConstraintSet) Supports(x string) bool {
	for i := range cs.Constraints {
		if !cs.Constraints[i].Supports(x) {
			return false
		}
	}
	return true
}

// AssertFeature returns an error if the feature is not supported by any
// of the constraints. The error contains the offending target/dialect pairs.
func (cs *ConstraintSet) AssertFeature(x string) error {
	var err error
	for i := range cs.Constraints {
		if e := cs.Constraints[i].AssertFeature(x); e != nil {
			err = errors.Join(err, e)
		}
	}
	return err
}

var _ fmt.Stringer = (*ConstraintSet)(nil)

func (cs *ConstraintSet) String() string {
	result := make([]string, 0, len(cs.Constraints)+1)
	for i := range cs.Constraints {
		result = append(result, cs.Constraints[i].String())
	}
	return "multi-constraint: " + strings.Join(result, ", ")
}

var (
	sqlBuiltins = allBuiltins

	// sqlite doesn't support startswith/endswith/contains
	sqlSQLiteBuiltins = ucastBuiltins.Clone().Add("internal.member_2")

	ucastBuiltins = NewSet(
		"eq",
		"neq",
		"lt",
		"lte",
		"gt",
		"gte",
	)
	allBuiltins = ucastBuiltins.Clone().Add(
		"internal.member_2",
		// "nin", // TODO: deal with NOT IN
		"startswith",
		"endswith",
		"contains",
	)
	ucastLINQBuiltins = ucastBuiltins.Clone().Add(
		"internal.member_2",
		// "nin", // TODO: deal with NOT IN
	)
)
