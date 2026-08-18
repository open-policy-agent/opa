package ast

import (
	"encoding"
	"fmt"

	"github.com/open-policy-agent/opa/v1/util"
)

func (mod *Module) AppendText(buf []byte) ([]byte, error) {
	if mod == nil {
		return append(buf, "<nil module>"...), nil
	}

	var err error

	// NOTE(anderseknert): this DOES allocate still, and while that's unfortunate,
	// we'll be better off dealing with that when we have v2 JSON in the stdlib than
	// doing manual JSON marshalling (and string length calculations) here.
	for _, annotations := range mod.Annotations {
		// rule annotations are attached to rules, so only check for package scoped ones here
		if annotations.Scope == "package" || annotations.Scope == "subpackages" {
			buf = append(buf, "# METADATA\n# "...)
			buf = append(buf, annotations.String()...)
			buf = append(buf, '\n')
		}
	}

	if buf, err = mod.Package.AppendText(buf); err != nil {
		return nil, err
	}
	buf = append(buf, '\n')

	if len(mod.Imports) > 0 {
		for _, imp := range mod.Imports {
			buf = append(buf, '\n')
			if buf, err = imp.AppendText(buf); err != nil {
				return nil, err
			}
		}
		buf = append(buf, '\n')
	}

	if len(mod.Rules) > 0 {
		for _, rule := range mod.Rules {
			buf = append(buf, '\n')
			if buf, err = rule.appendWithOpts(toStringOpts{regoVersion: mod.regoVersion}, buf); err != nil {
				return nil, err
			}
		}
	}

	return buf, nil
}

func (pkg *Package) AppendText(buf []byte) ([]byte, error) {
	var err error
	if pkg == nil {
		return append(buf, "<illegal nil package>"...), nil
	}
	if len(pkg.Path) <= 1 {
		buf = append(buf, "package <illegal path \""...)
		if buf, err = pkg.Path.AppendText(buf); err != nil {
			return nil, err
		}
		return append(buf, "\">"...), nil
	}

	buf = append(buf, "package "...)

	path := pkg.Path[1:] // omit "data"

	return path.AppendText(buf)
}

func (imp *Import) AppendText(buf []byte) ([]byte, error) {
	buf = append(buf, "import "...)
	var err error
	if buf, err = imp.Path.AppendText(buf); err != nil {
		return nil, err
	}
	if imp.Alias != "" {
		buf = append(buf, ' ', 'a', 's', ' ')
		buf = append(buf, imp.Alias...)
	}
	return buf, nil
}

func (rule *Rule) AppendText(buf []byte) ([]byte, error) {
	regoVersion := DefaultRegoVersion
	if rule.Module != nil {
		regoVersion = rule.Module.RegoVersion()
	}
	return rule.appendWithOpts(toStringOpts{regoVersion: regoVersion}, buf)
}

func (rule *Rule) appendWithOpts(opts toStringOpts, buf []byte) ([]byte, error) {
	// See note in [Module.AppendText] regarding annotations.
	for _, annotations := range rule.Annotations {
		buf = append(buf, "# METADATA\n# "...)
		buf = append(buf, annotations.String()...)
		buf = append(buf, '\n')
	}

	if rule.Default {
		buf = append(buf, "default "...)
	}

	var err error
	if buf, err = rule.Head.appendWithOpts(opts, buf); err != nil {
		return nil, err
	}

	if !rule.Default {
		switch opts.RegoVersion() {
		case RegoV1, RegoV0CompatV1:
			buf = append(buf, " if { "...)
		default:
			buf = append(buf, " { "...)
		}
		if buf, err = rule.Body.AppendText(buf); err != nil {
			return nil, err
		}
		buf = append(buf, " }"...)
	}
	if rule.Else != nil {
		if buf, err = rule.Else.appendElse(opts, buf); err != nil {
			return nil, err
		}
	}

	return buf, nil
}

func (rule *Rule) appendElse(opts toStringOpts, buf []byte) ([]byte, error) {
	buf = append(buf, " else "...)

	var err error
	if rule.Head.Value != nil {
		buf = append(buf, "= "...)
		if buf, err = rule.Head.Value.AppendText(buf); err != nil {
			return nil, err
		}
	}

	if v := opts.RegoVersion(); v == RegoV1 || v == RegoV0CompatV1 {
		buf = append(buf, " if { "...)
	} else {
		buf = append(buf, " { "...)
	}
	if buf, err = rule.Body.AppendText(buf); err != nil {
		return nil, err
	}
	buf = append(buf, " }"...)

	if rule.Else != nil {
		if buf, err = rule.Else.appendElse(opts, buf); err != nil {
			return nil, err
		}
	}

	return buf, nil
}

func (head *Head) AppendText(buf []byte) ([]byte, error) {
	return head.appendWithOpts(toStringOpts{}, buf)
}

func (head *Head) appendWithOpts(opts toStringOpts, buf []byte) ([]byte, error) {
	var err error
	if head.Reference == nil {
		buf = append(buf, head.Name...)
	} else {
		if buf, err = head.Reference.AppendText(buf); err != nil {
			return nil, err
		}
	}

	containsAdded := false
	switch {
	case len(head.Args) != 0:
		if buf, err = head.Args.AppendText(buf); err != nil {
			return nil, err
		}
	case len(head.Reference) == 1 && head.Key != nil:
		switch opts.RegoVersion() {
		case RegoV0:
			buf = append(buf, '[')
			if buf, err = head.Key.AppendText(buf); err != nil {
				return nil, err
			}
			buf = append(buf, ']')
		default:
			if buf, err = head.Key.AppendText(append(buf, " contains "...)); err != nil {
				return nil, err
			}
			containsAdded = true
		}
	}
	if head.Value != nil {
		if head.Assign {
			buf = append(buf, " := "...)
		} else {
			buf = append(buf, " = "...)
		}
		if buf, err = head.Value.AppendText(buf); err != nil {
			return nil, err
		}
	} else if !containsAdded && head.Name == "" && head.Key != nil {
		if buf, err = head.Key.AppendText(append(buf, " contains "...)); err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func (a Args) AppendText(buf []byte) ([]byte, error) {
	var err error
	buf = append(buf, '(')
	if buf, err = AppendDelimeted(buf, a, ", "); err != nil {
		return nil, err
	}
	return append(buf, ')'), nil
}

func (body Body) AppendText(buf []byte) ([]byte, error) {
	return AppendDelimeted(buf, body, "; ")
}

func (expr *Expr) AppendText(buf []byte) ([]byte, error) {
	if expr.Negated {
		buf = append(buf, "not "...)
	}

	var err error

	switch t := expr.Terms.(type) {
	case []*Term:
		if expr.IsEquality() && validEqAssignArgCount(expr) {
			if buf, err = t[1].AppendText(buf); err != nil {
				return nil, err
			}
			buf = append(append(append(buf, ' '), Equality.Infix...), ' ')
			if buf, err = t[2].AppendText(buf); err != nil {
				return nil, err
			}
		} else if buf, err = Call(t).AppendText(buf); err != nil {
			return nil, err
		}
	case encoding.TextAppender:
		if buf, err = t.AppendText(buf); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported expr terms type: %T", expr.Terms)
	}

	if len(expr.With) > 0 {
		buf = append(buf, ' ')
	}

	return AppendDelimeted(buf, expr.With, " ")
}

func (w *With) AppendText(buf []byte) ([]byte, error) {
	buf = append(buf, "with "...)
	var err error
	if buf, err = w.Target.AppendText(buf); err != nil {
		return nil, err
	}
	buf = append(buf, " as "...)
	if buf, err = w.Value.AppendText(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (q *Every) AppendText(buf []byte) ([]byte, error) {
	buf = append(buf, "every "...)
	var err error
	if q.Key != nil {
		if buf, err = q.Key.AppendText(buf); err != nil {
			return nil, err
		}
		buf = append(buf, ", "...)
	}
	if buf, err = q.Value.AppendText(buf); err == nil {
		buf = append(buf, " in "...)
		if buf, err = q.Domain.AppendText(buf); err == nil {
			buf = append(buf, " { "...)
			if buf, err = q.Body.AppendText(buf); err == nil {
				buf = append(buf, " }"...)
			}
		}
	}
	return buf, err
}

func (d *SomeDecl) AppendText(buf []byte) ([]byte, error) {
	var err error
	buf = append(buf, "some "...)
	if call, ok := d.Symbols[0].Value.(Call); ok {
		if buf, err = call[1].AppendText(buf); err != nil {
			return nil, err
		}
		if len(call) == 3 {
			buf = append(buf, " in "...)
		} else {
			buf = append(buf, ", "...)
		}
		if buf, err = call[2].AppendText(buf); err != nil {
			return nil, err
		}
		if len(call) == 4 {
			buf = append(buf, " in "...)
			if buf, err = call[3].AppendText(buf); err != nil {
				return nil, err
			}
		}
		return buf, nil
	}

	buf, err = AppendDelimeted(buf, d.Symbols, ", ")

	return buf, err
}

func (c *Comment) AppendText(buf []byte) ([]byte, error) {
	return append(append(buf, '#'), c.Text...), nil
}

func (a *LogicalAnd) AppendText(buf []byte) ([]byte, error) {
	return appendLogical(buf, "and", a.Lhs, a.Rhs, a.ExplicitLhs, a.ExplicitRhs)
}

func (o *LogicalOr) AppendText(buf []byte) ([]byte, error) {
	return appendLogical(buf, "or", o.Lhs, o.Rhs, o.ExplicitLhs, o.ExplicitRhs)
}

func appendLogical(buf []byte, op string, lhs, rhs Body, explicitLhs, explicitRhs bool) ([]byte, error) {
	var err error
	if buf, err = appendLogicalOperand(buf, lhs, explicitLhs, op, false); err != nil {
		return nil, err
	}
	buf = append(buf, ' ')
	buf = append(buf, op...)
	buf = append(buf, ' ')
	return appendLogicalOperand(buf, rhs, explicitRhs, op, true)
}

func appendLogicalOperand(buf []byte, b Body, explicit bool, parentOp string, rhs bool) ([]byte, error) {
	if !explicit && len(b) == 1 {
		if logicalOperandNeedsParens(b, parentOp, rhs) {
			buf = append(buf, '(')
			var err error
			if buf, err = b.AppendText(buf); err != nil {
				return nil, err
			}
			return append(buf, ')'), nil
		}
		return b.AppendText(buf)
	}

	buf = append(buf, "{ "...)
	var err error
	if buf, err = b.AppendText(buf); err != nil {
		return nil, err
	}
	return append(buf, " }"...), nil
}

// RulePath returns the string representation of the rule's path, i.e. its package path followed by the rule head ref.
func RulePath(r *Rule) string {
	if r == nil {
		return "<nil rule>"
	}
	if r.Module == nil {
		return "<rule " + r.Head.Reference.String() + " without module>"
	}
	buf := make([]byte, 0, r.Module.Package.Path.StringLength()+r.Head.Ref().StringLength()+1)
	buf, _ = r.Module.Package.Path.AppendText(buf)
	buf = append(buf, '.')
	buf, _ = r.Head.Ref().AppendText(buf)

	return util.ByteSliceToString(buf)
}
