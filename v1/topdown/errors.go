// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
)

var cancelErr = &Error{Code: CancelErr}

// Halt is a special error type that built-in function implementations return to indicate
// that policy evaluation should stop immediately.
type Halt struct {
	Err error
}

func (h Halt) Error() string {
	return h.Err.Error()
}

func (h Halt) Unwrap() error { return h.Err }

// Error is the error type returned by the Eval and Query functions when
// an evaluation error occurs.
type Error struct {
	Code     string        `json:"code"`
	Message  string        `json:"message"`
	Location *ast.Location `json:"location,omitempty"`

	// StackTrace is the stack of queries being evaluated when the error occurred.
	// Only populated when enabled (see Query.WithStackTraces), and left out of
	// Error() so enabling it doesn't change the messages callers display.
	StackTrace StackTrace `json:"stack_trace,omitempty"`

	err error `json:"-"`
}

const (
	// InternalErr represents an unknown evaluation error.
	InternalErr string = "eval_internal_error"

	// CancelErr indicates the evaluation process was cancelled.
	CancelErr string = "eval_cancel_error"

	// ConflictErr indicates a conflict was encountered during evaluation. For
	// instance, a conflict occurs if a rule produces multiple, differing values
	// for the same key in an object. Conflict errors indicate the policy does
	// not account for the data loaded into the policy engine.
	ConflictErr string = "eval_conflict_error"

	// TypeErr indicates evaluation stopped because an expression was applied to
	// a value of an inappropriate type.
	TypeErr string = "eval_type_error"

	// BuiltinErr indicates a built-in function received a semantically invalid
	// input or encountered some kind of runtime error, e.g., connection
	// timeout, connection refused, etc.
	BuiltinErr string = "eval_builtin_error"

	// WithMergeErr indicates that the real and replacement data could not be merged.
	WithMergeErr string = "eval_with_merge_error"
)

// IsError returns true if the err is an Error.
func IsError(err error) bool {
	_, ok := errors.AsType[*Error](err)
	return ok
}

// IsCancel returns true if err was caused by cancellation.
func IsCancel(err error) bool {
	return errors.Is(err, cancelErr)
}

// Is allows matching topdown errors using errors.Is (see IsCancel).
func (e *Error) Is(target error) bool {
	if t, ok := errors.AsType[*Error](target); ok {
		return (t.Code == "" || e.Code == t.Code) &&
			(t.Message == "" || e.Message == t.Message) &&
			(t.Location == nil || t.Location.Equal(e.Location))
	}
	return false
}

func (e *Error) Error() string {
	buf, _ := e.AppendText(make([]byte, 0, e.StringLength()))
	return util.ByteSliceToString(buf)
}

func (e *Error) AppendText(buf []byte) ([]byte, error) {
	if e.Location != nil {
		buf, _ := e.Location.AppendText(buf)
		buf = append(append(buf, ": "...), e.Code...)
		buf = append(append(buf, ": "...), e.Message...)
		return buf, nil
	}

	return append(append(append(buf, e.Code...), ": "...), e.Message...), nil
}

func (e *Error) StringLength() int {
	l := len(e.Code) + 2 + len(e.Message)
	if e.Location != nil {
		l += e.Location.StringLength() + 2
	}
	return l
}

func (e *Error) Wrap(err error) *Error {
	e.err = err
	return e
}

func (e *Error) Unwrap() error {
	return e.err
}

func functionConflictMsg(path string) string {
	return "function " + path + " produced conflicting values for the same inputs"
}

func completeDocConflictMsg(path string) string {
	return "rule " + path + " produced conflicting values"
}

// rulePath returns the ref of the document produced by rule, falling back to
// the rule's head ref when it is not contained in a module. Complete rules and
// functions always have ground refs.
func rulePath(rule *ast.Rule) string {
	if rule.Module == nil {
		return rule.Head.Ref().String()
	}
	return rule.Ref().String()
}

// maxConflictingRules bounds how many rules a conflict error lists, and with
// that, how much evaluation continues after the first conflict is detected.
const maxConflictingRules = 10

// errConflictLimit stops evaluation once maxConflictingRules is exceeded. It
// never escapes the evaluator; the collected ruleConflict is returned instead.
var errConflictLimit = errors.New("conflicting rules limit exceeded")

// ruleConflict collects the rules of a complete rule or function whose output
// differs from the first output produced.
type ruleConflict struct {
	loc   *ast.Location // location of the first conflicting rule
	rules []*ast.Rule
	more  bool

	// builtinErrs is the number of built-in errors recorded when the conflict
	// was detected. Built-in errors raised while collecting further conflicts
	// are discarded, as they would otherwise take precedence over the conflict
	// in strict mode.
	builtinErrs int

	// stack is the evaluation stack when the conflict was detected, nil when
	// stack traces are disabled.
	stack StackTrace
}

func newRuleConflict(rule, prev *ast.Rule, builtinErrs int) *ruleConflict {
	c := &ruleConflict{loc: rule.Location, rules: []*ast.Rule{prev}, builtinErrs: builtinErrs}
	c.add(rule)
	return c
}

// add records rule, and returns false when the limit is exceeded and
// evaluation should stop.
func (c *ruleConflict) add(rule *ast.Rule) bool {
	if slices.Contains(c.rules, rule) {
		return true
	}
	if len(c.rules) == maxConflictingRules {
		c.more = true
		return false
	}
	c.rules = append(c.rules, rule)
	return true
}

// error lists the locations of the conflicting rules in source order. The list
// is omitted when the conflict originates from a single rule, as the error
// location already points at it.
func (c *ruleConflict) error(msg string) error {
	locs := make([]*ast.Location, 0, len(c.rules))
	for _, rule := range c.rules {
		if rule.Location != nil {
			locs = append(locs, rule.Location)
		}
	}

	if len(locs) > 1 {
		slices.SortFunc(locs, (*ast.Location).Compare)

		s := strings.Builder{}
		s.WriteString(msg)
		s.WriteString(":")
		for _, loc := range locs {
			s.WriteString("\n  rule at ")
			// Location.String falls back to the full rule text when no file is
			// set, which is too verbose for a listing.
			if loc.File != "" {
				s.WriteString(loc.File)
				s.WriteByte(':')
				s.WriteString(strconv.Itoa(loc.Row))
			} else {
				s.WriteString(strconv.Itoa(loc.Row))
				s.WriteByte(':')
				s.WriteString(strconv.Itoa(loc.Col))
			}
		}
		if c.more {
			s.WriteString("\n  ...")
		}
		msg = s.String()
	}

	return &Error{
		Code:       ConflictErr,
		Location:   c.loc,
		Message:    msg,
		StackTrace: c.stack,
	}
}

func objectDocKeyConflictErr(loc *ast.Location) error {
	return &Error{
		Code:     ConflictErr,
		Location: loc,
		Message:  "object keys must be unique",
	}
}

func unsupportedBuiltinErr(loc *ast.Location, name string) error {
	return &Error{
		Code:     InternalErr,
		Location: loc,
		Message:  "unsupported built-in: " + name,
	}
}

func mergeConflictErr(loc *ast.Location) error {
	return &Error{
		Code:     WithMergeErr,
		Location: loc,
		Message:  "real and replacement data could not be merged",
	}
}

// unevaluatedOperandErr is returned when a built-in function would have been
// called with an operand that requires evaluation, which indicates a bug in OPA
// rather than in the policy being evaluated.
func unevaluatedOperandErr(loc *ast.Location, name string, pos int, operand *ast.Term) error {
	return &Error{
		Code:     InternalErr,
		Location: loc,
		Message: "built-in function " + name + " called with operand " + strconv.Itoa(pos) +
			" that requires evaluation: " + operand.String(),
	}
}

func internalErr(loc *ast.Location, msg string) error {
	return &Error{
		Code:     InternalErr,
		Location: loc,
		Message:  msg,
	}
}
