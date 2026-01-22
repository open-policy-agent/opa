package ast

import (
	"fmt"
	"unicode/utf8"

	"github.com/open-policy-agent/opa/v1/util"
)

// StringLengther is an interface for types that can report their string length without
// actually constructing the string. This is useful for pre-allocating buffers, like those
// used in AppendText, strings.Builder, bytes.Buffer, etc.
type StringLengther interface {
	StringLength() int
}

// TermSliceStringLength returns the total string length of the given terms, as reported
// by the [StringLengther.StringLength] method implementation of each term's [Value]. The
// delimLen value will be added between each term's length to account for a delimiter, or
// no delimiter if delimLen is 0.
// Implementation note: this function is optimized for inlining, and just meets the threshold
// for that. Don't change without making sure that's still the case.
func TermSliceStringLength(terms []*Term, delimLen int) (n int) {
	for i := range terms {
		n += terms[i].StringLength() + delimLen
	}
	return max(n-delimLen, 0)
}

func (t *Term) StringLength() int {
	if sl, ok := t.Value.(StringLengther); ok {
		return sl.StringLength()
	}

	panic("expected all ast.Value types to implement StringLenghter interface, got: " + ValueName(t.Value))
}

func (s String) StringLength() int {
	n := 2 // surrounding quotes
	bs := util.StringToByteSlice(s)
	for i := 0; i < len(bs); {
		r, size := utf8.DecodeRune(bs[i:])
		switch r {
		case '\\', '"':
			n += 2 // escaped backslash or quote
		case '\b', '\f', '\n', '\r', '\t':
			n += 2 // escaped control characters
		default:
			if r < 0x20 {
				n += 6 // unicode escape for other control characters
			} else {
				n += size // normal rune
			}
		}
		i += size
	}
	return n
}

func (n Number) StringLength() int {
	return len(n)
}

func (b Boolean) StringLength() int {
	if b {
		return 4
	}
	return 5
}

func (Null) StringLength() int {
	return 4
}

func (s *set) StringLength() int {
	if s.Len() == 0 {
		return 5 // set()
	}
	// surrounding {} + ", " for every element - 1
	return TermSliceStringLength(s.Slice(), 2) + 2
}

func (a *Array) StringLength() int {
	if a.Len() == 0 {
		return 2 // []
	}
	// surrounding brackets + ", " for every element - 1
	return TermSliceStringLength(a.elems, 2) + 2
}

func (o *object) StringLength() (n int) {
	if o.Len() == 0 {
		return 2 // {}
	}
	// ": " for every item + ", " for every item - 1
	o.Foreach(func(key, value *Term) {
		n += key.StringLength() + 4 + value.StringLength() // ": " and ", "
	})
	return n // surrounding {} but also minus last ", "
}

func (ts *TemplateString) StringLength() (n int) {
	for _, p := range ts.Parts {
		switch x := p.(type) {
		case *Expr:
			n += 2 + x.StringLength() // for {}
		case *Term:
			if s, ok := x.Value.(String); ok {
				n += len(s) + countUnescapedLeftCurly(string(s))
			} else {
				n += x.StringLength()
			}
		default:
			n += 9 // <invalid>
		}
	}
	return n + 3 // $"" or $``
}

func (c Call) StringLength() int {
	return c[0].StringLength() + 2 + TermSliceStringLength(c[1:], 2)
}

func (r Ref) StringLength() (n int) {
	rlen := len(r)
	if rlen == 0 {
		return 0
	}

	if s, ok := r[0].Value.(String); ok {
		n = len(s) // first term should never be quoted
	} else {
		n = r[0].StringLength()
	}

	if rlen == 1 {
		return n
	}

	for _, p := range r[1:] {
		switch v := p.Value.(type) {
		case String:
			str := string(v)
			if IsVarCompatibleString(str) && !IsKeyword(str) {
				n += 1 + len(str) // dot + name
			} else {
				n += 2 + p.StringLength() // brackets
			}
		default:
			n += 2 + p.StringLength() // brackets
		}
	}
	return n
}

func (v Var) StringLength() int {
	if v.IsWildcard() {
		return 1
	}
	return len(v)
}

func (s *SetComprehension) StringLength() int {
	return s.Term.StringLength() + s.Body.StringLength() + 5 // {} and " | "
}

func (a *ArrayComprehension) StringLength() int {
	return a.Term.StringLength() + a.Body.StringLength() + 5 // [] and " | "
}

func (o *ObjectComprehension) StringLength() (n int) {
	n += o.Key.StringLength()
	n += o.Value.StringLength()
	n += o.Body.StringLength()
	return n + 7 // "{}"", " | ", and ": "
}

func (m *Module) StringLength() (n int) {
	if m.Package != nil {
		n += m.Package.StringLength() + 2 // newlines
	}

	if len(m.Imports) > 0 {
		for _, imp := range m.Imports {
			n += imp.StringLength() + 1 // newline
		}
	}

	if len(m.Rules) > 0 {
		for _, rule := range m.Rules {
			n += rule.stringLengthWithOpts(toStringOpts{regoVersion: m.regoVersion}) + 1 // newline
		}
	}

	return n
}

func (p *Package) StringLength() int {
	if p == nil {
		return 21 // <illegal nil package>
	}
	if len(p.Path) <= 1 {
		return 25 + p.Path.StringLength() // // package <illegal path " ... ">
	}

	return 8 + p.Path[1:].StringLength() // "package ..."
}

func (i *Import) StringLength() (n int) {
	n = 7 + i.Path.StringLength() // "import " and path
	if i.Alias != "" {
		n += 4 + i.Alias.StringLength() // " as " and alias
	}
	return n
}

func (r *Rule) StringLength() int {
	return r.stringLengthWithOpts(toStringOpts{})
}

func (r *Rule) stringLengthWithOpts(opts toStringOpts) int {
	n := 0
	if r.Default {
		n += 8 // "default "
	}
	n += r.Head.stringLengthWithOpts(opts)
	if !r.Default {
		switch opts.RegoVersion() {
		case RegoV1, RegoV0CompatV1:
			n += 6 // " if { "
		default:
			n += 3 // " { "
		}
		n += r.Body.StringLength() + 2 // body and closing " }"
	}
	if r.Else != nil {
		n += r.Else.stringLengthWithOpts(opts)
	}
	return n
}

func (h *Head) StringLength() int {
	return h.stringLengthWithOpts(toStringOpts{})
}

func (h *Head) stringLengthWithOpts(opts toStringOpts) int {
	n := h.Reference.StringLength()
	containsAdded := false
	switch {
	case len(h.Args) != 0:
		n += h.Args.StringLength()
	case len(h.Reference) == 1 && h.Key != nil:
		switch opts.RegoVersion() {
		case RegoV0:
			n += 2 + h.Key.StringLength() // for []
		default:
			n += 10 + h.Key.StringLength() // " contains "
			containsAdded = true
		}
	}
	if h.Value != nil {
		if h.Assign {
			n += 4 // " := "
		} else {
			n += 3 // " = "
		}
		n += h.Value.StringLength()
	} else if !containsAdded && h.Name == "" && h.Key != nil {
		n += 10 + h.Key.StringLength() // " contains "
	}
	return n
}

func (a Args) StringLength() (n int) {
	n = 2 // ()
	for _, t := range a {
		n += t.StringLength() + 2 // ", "
	}
	return n - 2 // minus last ", "
}

func (b Body) StringLength() (n int) {
	for _, expr := range b {
		n += expr.StringLength() + 2 // "; "
	}
	return max(n-2, 0) // minus last "; " (if `n` isn't 0)
}

func (e *Expr) StringLength() (n int) {
	if e.Negated {
		n += 4 // "not "
	}
	switch terms := e.Terms.(type) {
	case []*Term:
		if e.IsEquality() && validEqAssignArgCount(e) {
			n += terms[1].StringLength() + len(Equality.Infix) + terms[2].StringLength() + 2 // spaces around =
		} else {
			n += Call(terms).StringLength()
		}
	case StringLengther:
		n += terms.StringLength()
	default:
		panic(fmt.Sprintf("string length estimation not implemented for type: %T", e.Terms))
	}

	for _, w := range e.With {
		n += w.StringLength() + 1 // space before with
	}

	return n
}

func (w *With) StringLength() int {
	return w.Target.StringLength() + w.Value.StringLength() + 9 // "with " and " as "
}

func (e *Every) StringLength() int {
	n := 6 // "every "
	if e.Key != nil {
		n += e.Key.StringLength() + 2 // ", "
	}
	n += e.Value.StringLength() + 4  // " in "
	n += e.Domain.StringLength() + 3 // " { "
	n += e.Body.StringLength() + 2   // " }"
	return n
}

func (s *SomeDecl) StringLength() int {
	n := 5 // "some "
	if call, ok := s.Symbols[0].Value.(Call); ok {
		n += 4 // " in "
		n += call[1].StringLength()
		if len(call) == 4 {
			n += 2 // ", "
		}
		n += call[2].StringLength()
		if len(call) == 4 {
			n += call[3].StringLength()
		}
		return n
	}
	return n + TermSliceStringLength(s.Symbols, 2)
}

func (c *Comment) StringLength() int {
	return 1 + len(c.Text) // '#' + text
}
