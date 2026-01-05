package ast

import (
	"encoding"
	"fmt"
)

func (m *Module) AppendText(buf []byte) ([]byte, error) {
	if m == nil {
		return append(buf, "<nil module>"...), nil
	}

	var err error

	// NOTE(anderseknert): this DOES allocate still, and while that's unfortunate,
	// we'll be better off dealing with that when we have v2 JSON in the stdlib than
	// doing manual JSON marshalling (and string length calculations) here.
	for _, annotations := range m.Annotations {
		// rule annotations are attached to rules, so only check for package scoped ones here
		if annotations.Scope == "package" || annotations.Scope == "subpackages" {
			buf = append(buf, "# METADATA\n# "...)
			buf = append(buf, annotations.String()...)
			buf = append(buf, '\n')
		}
	}

	if buf, err = m.Package.AppendText(buf); err != nil {
		return nil, err
	}
	buf = append(buf, '\n')

	if len(m.Imports) > 0 {
		for _, imp := range m.Imports {
			buf = append(buf, '\n')
			if buf, err = imp.AppendText(buf); err != nil {
				return nil, err
			}
		}
		buf = append(buf, '\n')
	}

	if len(m.Rules) > 0 {
		for _, rule := range m.Rules {
			buf = append(buf, '\n')
			if buf, err = rule.appendWithOpts(toStringOpts{regoVersion: m.regoVersion}, buf); err != nil {
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

	if s, ok := path[0].Value.(String); ok {
		buf = append(buf, s...) // first term should never be quoted
		if len(path) == 1 {
			return buf, nil
		}
		buf = append(buf, '.')
		path = path[1:]
	}

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

func (r *Rule) AppendText(buf []byte) ([]byte, error) {
	regoVersion := DefaultRegoVersion
	if r.Module != nil {
		regoVersion = r.Module.RegoVersion()
	}
	return r.appendWithOpts(toStringOpts{regoVersion: regoVersion}, buf)
}

func (r *Rule) appendWithOpts(opts toStringOpts, buf []byte) ([]byte, error) {
	// See note in [Module.AppendText] regarding annotations.
	for _, annotations := range r.Annotations {
		buf = append(buf, "# METADATA\n# "...)
		buf = append(buf, annotations.String()...)
		buf = append(buf, '\n')
	}

	if r.Default {
		buf = append(buf, "default "...)
	}

	var err error
	if buf, err = r.Head.appendWithOpts(opts, buf); err != nil {
		return nil, err
	}

	if !r.Default {
		switch opts.RegoVersion() {
		case RegoV1, RegoV0CompatV1:
			buf = append(buf, " if { "...)
		default:
			buf = append(buf, " { "...)
		}
		if buf, err = r.Body.AppendText(buf); err != nil {
			return nil, err
		}
		buf = append(buf, " }"...)
	}
	if r.Else != nil {
		if buf, err = r.Else.appendElse(opts, buf); err != nil {
			return nil, err
		}
	}

	return buf, nil
}

func (r *Rule) appendElse(opts toStringOpts, buf []byte) ([]byte, error) {
	buf = append(buf, " else "...)

	var err error
	if r.Head.Value != nil {
		buf = append(buf, "= "...)
		if buf, err = r.Head.Value.AppendText(buf); err != nil {
			return nil, err
		}
	}

	if v := opts.RegoVersion(); v == RegoV1 || v == RegoV0CompatV1 {
		buf = append(buf, " if { "...)
	} else {
		buf = append(buf, " { "...)
	}
	if buf, err = r.Body.AppendText(buf); err != nil {
		return nil, err
	}
	buf = append(buf, " }"...)

	if r.Else != nil {
		if buf, err = r.Else.appendElse(opts, buf); err != nil {
			return nil, err
		}
	}

	return buf, nil
}

func (h *Head) AppendText(buf []byte) ([]byte, error) {
	return h.appendWithOpts(toStringOpts{}, buf)
}

func (h *Head) appendWithOpts(opts toStringOpts, buf []byte) ([]byte, error) {
	var err error
	if h.Reference == nil {
		buf = append(buf, h.Name...)
	} else {
		if buf, err = h.Reference.AppendText(buf); err != nil {
			return nil, err
		}
	}

	containsAdded := false
	switch {
	case len(h.Args) != 0:
		if buf, err = h.Args.AppendText(buf); err != nil {
			return nil, err
		}
	case len(h.Reference) == 1 && h.Key != nil:
		switch opts.RegoVersion() {
		case RegoV0:
			buf = append(buf, '[')
			if buf, err = h.Key.AppendText(buf); err != nil {
				return nil, err
			}
			buf = append(buf, ']')
		default:
			if buf, err = h.Key.AppendText(append(buf, " contains "...)); err != nil {
				return nil, err
			}
			containsAdded = true
		}
	}
	if h.Value != nil {
		if h.Assign {
			buf = append(buf, " := "...)
		} else {
			buf = append(buf, " = "...)
		}
		if buf, err = h.Value.AppendText(buf); err != nil {
			return nil, err
		}
	} else if !containsAdded && h.Name == "" && h.Key != nil {
		if buf, err = h.Key.AppendText(append(buf, " contains "...)); err != nil {
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

func (w *Every) AppendText(buf []byte) ([]byte, error) {
	buf = append(buf, "every "...)
	var err error
	if w.Key != nil {
		if buf, err = w.Key.AppendText(buf); err != nil {
			return nil, err
		}
		buf = append(buf, ", "...)
	}
	if buf, err = w.Value.AppendText(buf); err == nil {
		buf = append(buf, " in "...)
		if buf, err = w.Domain.AppendText(buf); err == nil {
			buf = append(buf, " { "...)
			if buf, err = w.Body.AppendText(buf); err == nil {
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
