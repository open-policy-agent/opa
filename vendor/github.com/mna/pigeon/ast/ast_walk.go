package ast

import "fmt"

// A Visitor implements a Visit method, which is invoked for each Expression
// encountered by Walk.
// If the result visitor w is not nil, Walk visits each of the children
// of Expression with the visitor w, followed by a call of w.Visit(nil).
type Visitor interface {
	Visit(expr Expression) (w Visitor)
}

// Walk traverses an AST in depth-first order: It starts by calling
// v.Visit(expr); Expression must not be nil. If the visitor w returned by
// v.Visit(expr) is not nil, Walk is invoked recursively with visitor
// w for each of the non-nil children of Expression, followed by a call of
// w.Visit(nil).
//
func Walk(v Visitor, expr Expression) {
	if v = v.Visit(expr); v == nil {
		return
	}

	switch expr := expr.(type) {
	case *ActionExpr:
		Walk(v, expr.Expr)
	case *AndCodeExpr:
		// Nothing to do
	case *AndExpr:
		Walk(v, expr.Expr)
	case *AnyMatcher:
		// Nothing to do
	case *CharClassMatcher:
		// Nothing to do
	case *ChoiceExpr:
		for _, e := range expr.Alternatives {
			Walk(v, e)
		}
	case *Grammar:
		for _, e := range expr.Rules {
			Walk(v, e)
		}
	case *LabeledExpr:
		Walk(v, expr.Expr)
	case *LitMatcher:
		// Nothing to do
	case *NotCodeExpr:
		// Nothing to do
	case *NotExpr:
		Walk(v, expr.Expr)
	case *OneOrMoreExpr:
		Walk(v, expr.Expr)
	case *Rule:
		Walk(v, expr.Expr)
	case *RuleRefExpr:
		// Nothing to do
	case *SeqExpr:
		for _, e := range expr.Exprs {
			Walk(v, e)
		}
	case *StateCodeExpr:
		// Nothing to do
	case *ZeroOrMoreExpr:
		Walk(v, expr.Expr)
	case *ZeroOrOneExpr:
		Walk(v, expr.Expr)
	default:
		panic(fmt.Sprintf("unknown expression type %T", expr))
	}
}

type inspector func(Expression) bool

func (f inspector) Visit(expr Expression) Visitor {
	if f(expr) {
		return f
	}
	return nil
}

// Inspect traverses an AST in depth-first order: It starts by calling
// f(expr); expr must not be nil. If f returns true, Inspect invokes f
// recursively for each of the non-nil children of expr, followed by a
// call of f(nil).
func Inspect(expr Expression, f func(Expression) bool) {
	Walk(inspector(f), expr)
}
