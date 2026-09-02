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

func (term *Term) StringLength() int {
	if sl, ok := term.Value.(StringLengther); ok {
		return sl.StringLength()
	}
	panic("expected all ast.Value types to implement StringLenghter interface, got: " + ValueName(term.Value))
}

func (str String) StringLength() int {
	n := 2 // surrounding quotes
	bs := util.StringToByteSlice(str)
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

func (num Number) StringLength() int {
	return len(num)
}

func (bol Boolean) StringLength() int {
	if bol {
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

func (arr *Array) StringLength() int {
	if arr.Len() == 0 {
		return 2 // []
	}
	// surrounding brackets + ", " for every element - 1
	return TermSliceStringLength(arr.elems, 2) + 2
}

func (obj *object) StringLength() (n int) {
	if obj.Len() == 0 {
		return 2 // {}
	}
	// ": " for every item + ", " for every item - 1
	obj.Foreach(func(key, value *Term) {
		n += key.StringLength() + 4 + value.StringLength() // ": " and ", "
	})
	return n // surrounding {} but also minus last ", "
}

func (lob *lazyObj) StringLength() int {
	return lob.force().(*object).StringLength()
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

// comprehensionTermStringLength returns the string length of a term when
// rendered inside a comprehension head, where infix operators are rendered
// in infix notation wrapped in parens.
func comprehensionTermStringLength(t *Term) int {
	if call, ok := t.Value.(Call); ok && len(call) == 3 {
		if bi, found := BuiltinMap[call[0].String()]; found && bi.Infix != "" && bi.Infix != "in" {
			// "(left op right)" = left + " " + op + " " + right + "()"
			return comprehensionTermStringLength(call[1]) + len(bi.Infix) + comprehensionTermStringLength(call[2]) + 4
		}
	}
	return t.StringLength()
}

func (ref Ref) StringLength() (n int) {
	rlen := len(ref)
	if rlen == 0 {
		return 0
	}

	if s, ok := ref[0].Value.(String); ok {
		n = len(s) // first term should never be quoted
	} else {
		n = ref[0].StringLength()
	}

	if rlen == 1 {
		return n
	}

	for _, p := range ref[1:] {
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

func (sc *SetComprehension) StringLength() int {
	return comprehensionTermStringLength(sc.Term) + sc.Body.StringLength() + 5 // {} and " | "
}

func (ac *ArrayComprehension) StringLength() int {
	return comprehensionTermStringLength(ac.Term) + ac.Body.StringLength() + 5 // [] and " | "
}

func (oc *ObjectComprehension) StringLength() (n int) {
	n += comprehensionTermStringLength(oc.Key)
	n += comprehensionTermStringLength(oc.Value)
	n += oc.Body.StringLength()
	return n + 7 // "{}"", " | ", and ": "
}

func (mod *Module) StringLength() (n int) {
	if mod.Package != nil {
		n += mod.Package.StringLength() + 2 // newlines
	}

	if len(mod.Imports) > 0 {
		for _, imp := range mod.Imports {
			n += imp.StringLength() + 1 // newline
		}
	}

	if len(mod.Rules) > 0 {
		for _, rule := range mod.Rules {
			n += rule.stringLengthWithOpts(toStringOpts{regoVersion: mod.regoVersion}) + 1 // newline
		}
	}

	return n
}

func (pkg *Package) StringLength() int {
	if pkg == nil {
		return 21 // <illegal nil package>
	}
	if len(pkg.Path) <= 1 {
		return 25 + pkg.Path.StringLength() // // package <illegal path " ... ">
	}

	return 8 + pkg.Path[1:].StringLength() // "package ..."
}

func (imp *Import) StringLength() (n int) {
	n = 7 + imp.Path.StringLength() // "import " and path
	if imp.Alias != "" {
		n += 4 + imp.Alias.StringLength() // " as " and alias
	}
	return n
}

func (rule *Rule) StringLength() int {
	return rule.stringLengthWithOpts(toStringOpts{})
}

func (rule *Rule) stringLengthWithOpts(opts toStringOpts) int {
	n := 0
	if rule.Default {
		n += 8 // "default "
	}
	n += rule.Head.stringLengthWithOpts(opts)
	if !rule.Default {
		switch opts.RegoVersion() {
		case RegoV1, RegoV0CompatV1:
			n += 6 // " if { "
		default:
			n += 3 // " { "
		}
		n += rule.Body.StringLength() + 2 // body and closing " }"
	}
	if rule.Else != nil {
		n += rule.Else.stringLengthWithOpts(opts)
	}
	return n
}

func (head *Head) StringLength() int {
	return head.stringLengthWithOpts(toStringOpts{})
}

func (head *Head) stringLengthWithOpts(opts toStringOpts) int {
	n := head.Reference.StringLength()
	containsAdded := false
	switch {
	case len(head.Args) != 0:
		n += head.Args.StringLength()
	case len(head.Reference) == 1 && head.Key != nil:
		switch opts.RegoVersion() {
		case RegoV0:
			n += 2 + head.Key.StringLength() // for []
		default:
			n += 10 + head.Key.StringLength() // " contains "
			containsAdded = true
		}
	}
	if head.Value != nil {
		if head.Assign {
			n += 4 // " := "
		} else {
			n += 3 // " = "
		}
		n += head.Value.StringLength()
	} else if !containsAdded && head.Name == "" && head.Key != nil {
		n += 10 + head.Key.StringLength() // " contains "
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

func (body Body) StringLength() (n int) {
	for _, expr := range body {
		n += expr.StringLength() + 2 // "; "
	}
	return max(n-2, 0) // minus last "; " (if `n` isn't 0)
}

func (expr *Expr) StringLength() (n int) {
	if expr.Negated {
		n += 4 // "not "
	}
	switch terms := expr.Terms.(type) {
	case []*Term:
		if expr.IsEquality() && validEqAssignArgCount(expr) {
			n += terms[1].StringLength() + len(Equality.Infix) + terms[2].StringLength() + 2 // spaces around =
		} else {
			n += Call(terms).StringLength()
		}
	case StringLengther:
		n += terms.StringLength()
	default:
		panic(fmt.Sprintf("string length estimation not implemented for type: %T", expr.Terms))
	}

	for _, w := range expr.With {
		n += w.StringLength() + 1 // space before with
	}

	return n
}

func (w *With) StringLength() int {
	return w.Target.StringLength() + w.Value.StringLength() + 9 // "with " and " as "
}

func (q *Every) StringLength() int {
	n := 6 // "every "
	if q.Key != nil {
		n += q.Key.StringLength() + 2 // ", "
	}
	n += q.Value.StringLength() + 4  // " in "
	n += q.Domain.StringLength() + 3 // " { "
	n += q.Body.StringLength() + 2   // " }"
	return n
}

func (d *SomeDecl) StringLength() int {
	n := 5 // "some "
	if call, ok := d.Symbols[0].Value.(Call); ok {
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
	return n + TermSliceStringLength(d.Symbols, 2)
}

func (c *Comment) StringLength() int {
	return 1 + len(c.Text) // '#' + text
}

func (n *Not) StringLength() int {
	if !n.ExplicitBody && len(n.Body) == 1 {
		if notBodyNeedsParens(n.Body) {
			// "not (...)"
			return 6 + n.Body.StringLength()
		}
		// "not ..."
		return 4 + n.Body.StringLength()
	}
	// "not {...}"
	return 6 + n.Body.StringLength()
}

func (a *LogicalAnd) StringLength() int {
	return logicalOperandStringLength(a.Lhs, a.ExplicitLhs, "and", false) +
		5 + // " and "
		logicalOperandStringLength(a.Rhs, a.ExplicitRhs, "and", true)
}

func (o *LogicalOr) StringLength() int {
	return logicalOperandStringLength(o.Lhs, o.ExplicitLhs, "or", false) +
		4 + // " or "
		logicalOperandStringLength(o.Rhs, o.ExplicitRhs, "or", true)
}

func logicalOperandStringLength(b Body, explicit bool, parentOp string, rhs bool) int {
	if !explicit && len(b) == 1 {
		if logicalOperandNeedsParens(b, parentOp, rhs) {
			return b.StringLength() + 2 // "(" + body + ")"
		}
		return b.StringLength()
	}
	return b.StringLength() + 4 // "{ " + body + " }"
}
