// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
)

// maxConflicts bounds how many values a conflict error lists, and with that,
// how much evaluation continues after a conflict is detected.
const maxConflicts = 10

var (
	// errConflictFound ends the evaluation of a rule once it has produced a
	// conflicting value. One per rule is enough to point at it.
	errConflictFound = errors.New("conflicting value found")

	// errConflictLimit ends the collection once more than maxConflicts values
	// conflict.
	errConflictLimit = errors.New("conflicting values limit exceeded")
)

// Evaluation stops at the first conflict, which only tells us about two of the
// rules involved. Once a conflict is confirmed, the rules are evaluated again
// to find every rule producing a conflicting value, so that the error can list
// them. This keeps the cost of listing them off the path of evaluations that
// don't conflict.

// conflictValue is a value produced by one of the rules of a complete rule or
// function, and the location of that rule.
type conflictValue struct {
	loc   *ast.Location
	value *ast.Term
}

// conflicts collects the values of a complete rule or function that differ
// from the first value produced.
type conflicts struct {
	values []conflictValue // values[0] is the first value produced
	more   bool            // more conflicting values than maxConflicts
}

// add records value when it is the first, or conflicts with the first. It
// returns errConflictFound when value conflicts, and errConflictLimit when it
// would exceed maxConflicts.
func (c *conflicts) add(rule *ast.Rule, value *ast.Term) error {
	if len(c.values) > 0 && c.values[0].value.Equal(value) {
		return nil
	}
	if len(c.values) == maxConflicts {
		c.more = true
		return errConflictLimit
	}
	c.values = append(c.values, conflictValue{loc: rule.Location, value: value})
	if len(c.values) == 1 {
		return nil
	}
	return errConflictFound
}

// ownConflict returns err when it is the conflict raised by rule, rather than
// one raised by a document that rule's body depends on. Conflict errors carry
// the location of the rule raising them, and as rules can't be recursive, no
// other conflict can carry the same one.
func ownConflict(err error, rule *ast.Rule) (*Error, bool) {
	tdErr, ok := err.(*Error)
	return tdErr, ok && rule.Location != nil && tdErr.Code == ConflictErr && tdErr.Location == rule.Location
}

// conflictErr returns err listing every conflicting value when it is the
// conflict raised by rule, and err unchanged otherwise.
func (e evalVirtualComplete) conflictErr(err error, rule *ast.Rule, findOne bool) error {
	tdErr, ok := ownConflict(err, rule)
	// Partial evaluation may save the rule bodies rather than produce values.
	if !ok || e.e.partial() {
		return err
	}
	return e.e.collectConflicts(tdErr, e.ir, func(rule *ast.Rule, c *conflicts) (bool, error) {
		return e.conflictingValues(rule, c, findOne)
	})
}

// conflictingValues evaluates rule, adding the values it produces to c. The
// values are not passed on, nor cached: the evaluation ends in a conflict error.
func (e evalVirtualComplete) conflictingValues(rule *ast.Rule, c *conflicts, findOne bool) (bool, error) {
	child := evalPool.Get()
	defer evalPool.Put(child)

	e.e.childWithBindingSizeHint(rule.Body, child, ast.EstimateBodyBindingCount(rule.Body))
	child.findOne = findOne
	child.traceEnter(rule)

	var produced bool
	err := child.eval(func(child *eval) error {
		child.traceExit(rule)
		produced = true
		if err := c.add(rule, child.bindings.Plug(rule.Head.Value)); err != nil {
			return err
		}
		child.traceRedo(rule)
		return nil
	})

	return produced, ruleDone(err)
}

// conflictErr returns err listing every conflicting value when it is the
// conflict raised by rule, and err unchanged otherwise.
func (e *evalFunc) conflictErr(err error, rule *ast.Rule, findOne bool) error {
	tdErr, ok := ownConflict(err, rule)
	// Partial evaluation may save the rule bodies rather than produce values.
	if !ok || e.e.partial() {
		return err
	}
	return e.e.collectConflicts(tdErr, e.ir, func(rule *ast.Rule, c *conflicts) (bool, error) {
		return e.conflictingValues(rule, c, findOne)
	})
}

// conflictingValues evaluates rule for the arguments of the call, adding the
// values it produces to c. The values are not passed on, nor cached: the
// evaluation ends in a conflict error.
func (e *evalFunc) conflictingValues(rule *ast.Rule, c *conflicts, findOne bool) (bool, error) {
	numArgs := len(e.args)
	copy(e.args, rule.Head.Args)
	if numArgs == len(rule.Head.Args)+1 {
		e.args[numArgs-1] = rule.Head.Value
	}

	child := evalPool.Get()
	defer evalPool.Put(child)

	e.e.childWithBindingSizeHint(rule.Body, child, numArgs)
	child.findOne = findOne
	child.traceEnter(rule)

	var produced bool
	err := child.biunifyTerms(e.terms[1:], e.args, e.e.bindings, child.bindings, func() error {
		return child.eval(func(child *eval) error {
			child.traceExit(rule)
			produced = true
			if err := c.add(rule, child.bindings.Plug(rule.Head.Value)); err != nil {
				return err
			}
			child.traceRedo(rule)
			return nil
		})
	})

	return produced, ruleDone(err)
}

// ruleDone returns nil for the errors that only end the evaluation of a rule.
func ruleDone(err error) error {
	if _, ok := err.(*deferredEarlyExitError); ok || errors.Is(err, errConflictFound) {
		return nil
	}
	return err
}

// collectConflicts evaluates the rules in ir the way evalValue does, using
// eval to evaluate each, and returns err listing the values that conflict.
func (e *eval) collectConflicts(err *Error, ir *ast.IndexResult, eval func(*ast.Rule, *conflicts) (bool, error)) error {
	// Built-in errors raised here were either raised by the evaluation that
	// found the conflict already, or would not have been raised by it. Keeping
	// them would let them take precedence over the conflict in strict mode.
	numBuiltinErrs := len(e.builtinErrors.errs)
	defer func() {
		e.builtinErrors.errs = e.builtinErrors.errs[:numBuiltinErrs]
	}()

	var c conflicts
	for _, rule := range ir.Rules {
		produced, evalErr := eval(rule, &c)
		for _, erule := range ir.Else[rule] {
			if produced || evalErr != nil {
				break
			}
			produced, evalErr = eval(erule, &c)
		}
		// Anything else going wrong here was not what ended the evaluation, so
		// report the conflict with the values found so far.
		if evalErr != nil {
			break
		}
	}

	// Evaluating the rules again may not reproduce the conflict, if they
	// depend on non-deterministic built-ins.
	if len(c.values) < 2 {
		return err
	}

	return c.error(err)
}

// error returns err listing the conflicting values in source order.
func (c *conflicts) error(err *Error) error {
	values := slices.Clone(c.values)
	slices.SortStableFunc(values, func(a, b conflictValue) int {
		return a.loc.Compare(b.loc)
	})

	s := strings.Builder{}
	s.WriteString(err.Message)
	s.WriteByte(':')
	for _, v := range values {
		s.WriteString("\n  ")
		s.WriteString(frameText(v.value.String()))
		s.WriteString(" at ")
		writeConflictLocation(&s, v.loc)
	}
	if c.more {
		s.WriteString("\n  ...")
	}

	cpy := *err
	cpy.Message = s.String()
	return &cpy
}

// writeConflictLocation writes loc as file:row. Location.String falls back to
// the full rule text when no file is set, which is too verbose for a listing.
func writeConflictLocation(s *strings.Builder, loc *ast.Location) {
	switch {
	case loc == nil:
		s.WriteString("<unknown>")
	case loc.File != "":
		s.WriteString(loc.File)
		s.WriteByte(':')
		s.WriteString(strconv.Itoa(loc.Row))
	default:
		s.WriteString(strconv.Itoa(loc.Row))
		s.WriteByte(':')
		s.WriteString(strconv.Itoa(loc.Col))
	}
}
