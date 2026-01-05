package ast

import (
	"encoding"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/v1/util"
)

// AppendText appends the text representation of term (i.e. as printed in policy) to
// buf and returns the extended buffer.
func (term *Term) AppendText(buf []byte) ([]byte, error) {
	if app, ok := term.Value.(encoding.TextAppender); ok {
		return app.AppendText(buf)
	}

	return append(buf, term.Value.String()...), nil
}

func (v Var) AppendText(buf []byte) ([]byte, error) {
	if v.IsWildcard() {
		return append(buf, WildcardString...), nil
	}
	return append(buf, v...), nil
}

func (b Boolean) AppendText(buf []byte) ([]byte, error) {
	if b {
		return append(buf, "true"...), nil
	}
	return append(buf, "false"...), nil
}

func (Null) AppendText(buf []byte) ([]byte, error) {
	return append(buf, "null"...), nil
}

func (str String) AppendText(buf []byte) ([]byte, error) {
	return strconv.AppendQuote(buf, string(str)), nil
}

func (str String) appendNoQuote(buf []byte) []byte {
	// Append using strconv.AppendQuote for proper escaping, but trim off
	// the leading and trailing quotes afterwards.
	oldLen := len(buf)
	buf = strconv.AppendQuote(buf, string(str))
	newLen := len(buf)
	quoted := buf[oldLen:newLen]

	return append(buf[:oldLen], quoted[1:len(quoted)-1]...)
}

func (num Number) AppendText(buf []byte) ([]byte, error) {
	return append(buf, num...), nil
}

func (arr *Array) AppendText(buf []byte) ([]byte, error) {
	buf, err := AppendDelimeted(append(buf, '['), arr.elems, ", ")
	if err != nil {
		return nil, err
	}
	return append(buf, ']'), nil
}

func (obj *object) AppendText(buf []byte) ([]byte, error) {
	olen := obj.Len()
	if olen == 0 {
		return append(buf, "{}"...), nil
	}

	buf = append(buf, '{')

	var err error

	// first key-value pair
	keys := obj.sortedKeys()
	for i := range keys {
		if buf, err = keys[i].key.AppendText(buf); err != nil {
			return nil, err
		}
		buf = append(buf, ": "...)
		if buf, err = keys[i].value.AppendText(buf); err != nil {
			return nil, err
		}
		if i < olen-1 {
			buf = append(buf, ", "...)
		}
	}

	return append(buf, '}'), nil
}

func (obj *lazyObj) AppendText(buf []byte) ([]byte, error) {
	return append(buf, obj.force().String()...), nil
}

func (s *set) AppendText(buf []byte) ([]byte, error) {
	slen := s.Len()
	if slen == 0 {
		return append(buf, "set()"...), nil
	}

	var err error

	buf = append(buf, '{')
	if buf, err = AppendDelimeted(buf, s.sortedKeys(), ", "); err != nil {
		return nil, err
	}

	return append(buf, '}'), nil
}

func (c Call) AppendText(buf []byte) ([]byte, error) {
	if len(c) == 0 {
		return buf, nil
	}

	var err error

	if buf, err = c[0].AppendText(buf); err != nil {
		return nil, err
	}

	if buf, err = AppendDelimeted(append(buf, '('), c[1:], ", "); err != nil {
		return nil, err
	}
	return append(buf, ')'), nil
}

func (ts *TemplateString) AppendText(buf []byte) ([]byte, error) {
	buf = append(buf, "$\""...)
	for _, p := range ts.Parts {
		switch x := p.(type) {
		case *Expr:
			buf = append(buf, '{')
			var err error
			if buf, err = x.AppendText(buf); err != nil {
				return nil, err
			}
			buf = append(buf, '}')
		case *Term:
			if str, ok := x.Value.(String); ok {
				// TODO(anders): this is a bit of a mess, but as explained by the comment on
				// [EscapeTemplateStringStringPart], required as long as we rely on strconv for escaping, which adds
				// quotes around the string that we don't want here, and trying to "unappend" them is not nice at all..
				s := string(str)
				ulc := countUnescapedLeftCurly(s)
				sl := str.StringLength() + ulc - 2 // no surrounding quotes

				if sl == len(s) { // no escaping needed
					buf = append(buf, s...)
				} else { // some escaping needed
					if sl == len(s)+ulc { // only unescaped {
						buf = AppendEscapedTemplateStringStringPart(buf, string(str))
					} else { // full escaping needed. this is expensive but luckily rare
						tmp := str.appendNoQuote(make([]byte, 0, sl))
						ets := EscapeTemplateStringStringPart(util.ByteSliceToString(tmp))
						buf = append(buf, ets...)
					}
				}
			} else {
				var err error
				if buf, err = x.AppendText(buf); err != nil {
					return nil, err
				}
			}
		default:
			buf = append(buf, "<invalid>"...)
		}
	}
	return append(buf, '"'), nil
}

func (r Ref) AppendText(buf []byte) ([]byte, error) {
	reflen := len(r)
	if reflen == 0 {
		return buf, nil
	}
	if reflen == 1 {
		if s, ok := r[0].Value.(String); ok {
			// While a ref head is typically a Var, a lone String term should not be quoted
			return append(buf, s...), nil
		}
		return r[0].AppendText(buf)
	}
	if name, ok := BuiltinNameFromRef(r); ok {
		return append(buf, name...), nil
	}

	var err error
	if s, ok := r[0].Value.(String); ok {
		buf = append(buf, s...)
	} else if buf, err = r[0].AppendText(buf); err != nil {
		return nil, err
	}

	for _, p := range r[1:] {
		switch v := p.Value.(type) {
		case String:
			str := string(v)
			if IsVarCompatibleString(str) && !IsKeyword(str) {
				buf = append(append(buf, '.'), str...)
			} else {
				buf = append(buf, '[')
				// Determine whether we need the full JSON-escaped form
				if strings.ContainsFunc(str, isControlOrBackslash) {
					if buf, err = v.AppendText(buf); err != nil {
						return nil, err
					}
				} else {
					buf = append(append(append(buf, '"'), str...), '"')
				}
				buf = append(buf, ']')
			}
		default:
			buf = append(buf, '[')
			if buf, err = p.AppendText(buf); err != nil {
				return nil, err
			}
			buf = append(buf, ']')
		}
	}

	return buf, nil
}

func (sc *SetComprehension) AppendText(buf []byte) ([]byte, error) {
	buf = append(buf, '{')
	var err error
	if buf, err = sc.Term.AppendText(buf); err != nil {
		return nil, err
	}
	if buf, err = sc.Body.AppendText(append(buf, " | "...)); err != nil {
		return nil, err
	}
	return append(buf, '}'), nil
}

func (ac *ArrayComprehension) AppendText(buf []byte) ([]byte, error) {
	buf = append(buf, '[')
	var err error
	if buf, err = ac.Term.AppendText(buf); err != nil {
		return nil, err
	}
	if buf, err = ac.Body.AppendText(append(buf, " | "...)); err != nil {
		return nil, err
	}
	return append(buf, ']'), nil
}

func (oc *ObjectComprehension) AppendText(buf []byte) ([]byte, error) {
	buf = append(buf, '{')
	var err error
	if buf, err = oc.Key.AppendText(buf); err != nil {
		return nil, err
	}
	buf = append(buf, ": "...)
	if buf, err = oc.Value.AppendText(buf); err != nil {
		return nil, err
	}
	if buf, err = oc.Body.AppendText(append(buf, " | "...)); err != nil {
		return nil, err
	}
	return append(buf, '}'), nil
}
