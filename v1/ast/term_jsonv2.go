//go:build go1.27

package ast

import (
	"encoding"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/util"
)

var (
	_ json.MarshalerTo = &Term{}
	_ json.Unmarshaler = &LogicalOr{}
	_ json.MarshalerTo = &LogicalOr{}
	_ json.MarshalerTo = &Not{}
	_ json.MarshalerTo = &Array{}
	_ json.MarshalerTo = &set{}
	_ json.MarshalerTo = &object{}
	_ json.MarshalerTo = &TemplateString{}
	_ json.MarshalerTo = &Ref{}
	_ json.MarshalerTo = &lazyObj{}
	_ json.MarshalerTo = Args{}
	_ json.MarshalerTo = Boolean(false)
	_ json.MarshalerTo = Null{}
	_ json.MarshalerTo = Number("")
	_ json.MarshalerTo = String("")
	_ json.MarshalerTo = Var("")
	_ json.Unmarshaler = &Not{}
)

// These are here to ensure that we do not fall down to TextAppender, which
// Go 1.27's encoding/json would otherwise use, encoding these as JSON strings.

func (b Boolean) MarshalJSONTo(e *jsontext.Encoder) error {
	return e.WriteToken(jsontext.Bool(bool(b)))
}

func (Null) MarshalJSONTo(e *jsontext.Encoder) error {
	// Encoded as an empty object rather than null, as that's the representation
	// callers have come to expect. See also [marshalValueTo].
	return e.WriteValue([]byte("{}"))
}

func (v Var) MarshalJSONTo(e *jsontext.Encoder) error {
	// Must produce the var name as a JSON string, wildcard vars included: that's
	// what encoding/json v1 does for a type whose underlying kind is string.
	return e.WriteToken(jsontext.String(string(v)))
}

func (num Number) MarshalJSONTo(e *jsontext.Encoder) error {
	if num == "" {
		// Matches encoding/json v1, which encodes an empty json.Number as 0.
		return e.WriteToken(jsontext.Int(0))
	}
	return e.WriteValue(jsontext.Value(num))
}

func (str String) MarshalJSONTo(e *jsontext.Encoder) error {
	return e.WriteToken(jsontext.String(string(str)))
}

func (t *Term) MarshalJSONTo(e *jsontext.Encoder) (err error) {
	e.WriteToken(jsontext.BeginObject)

	includeLocation := astJSON.GetOptions().MarshalOptions.IncludeLocation
	if t.Location != nil && includeLocation.Term {
		e.WriteToken(jsontext.String("location"))
		t.Location.MarshalJSONTo(e)
	}

	e.WriteToken(jsontext.String("type"))
	e.WriteToken(jsontext.String(ValueName(t.Value)))

	e.WriteToken(jsontext.String("value"))
	if err = marshalValueTo(e, t.Value); err != nil {
		return fmt.Errorf("failed to marshal term of %s: %w", ValueName(t.Value), err)
	}

	return e.WriteToken(jsontext.EndObject)
}

// MarshalJSON returns the JSON encoding of the term.
func (term *Term) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(term)
}

func (r Ref) MarshalJSONTo(e *jsontext.Encoder) (err error) {
	return util.WriteMarshalerToArray(e, r)
}

func (t *TemplateString) MarshalJSONTo(e *jsontext.Encoder) (err error) {
	e.WriteToken(jsontext.BeginObject)
	e.WriteToken(jsontext.String("parts"))
	if t.Parts == nil {
		// Parts has no omitempty tag, so it's always written. Matches
		// encoding/json v1, which encodes a nil slice as null rather than as an
		// empty array.
		e.WriteToken(jsontext.Null)
	} else {
		e.WriteToken(jsontext.BeginArray)
		for _, p := range t.Parts {
			switch v := p.(type) {
			case *Expr:
				v.MarshalJSONTo(e)
			case *Term:
				v.MarshalJSONTo(e)
			}
		}
		e.WriteToken(jsontext.EndArray)
	}

	e.WriteToken(jsontext.String("multi_line"))
	e.WriteToken(jsontext.Bool(t.MultiLine))

	return e.WriteToken(jsontext.EndObject)
}

func (n *Not) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)
	e.WriteToken(jsontext.String("type"))
	e.WriteToken(jsontext.String("not"))

	e.WriteToken(jsontext.String("body"))
	n.Body.MarshalJSONTo(e)

	e.WriteToken(jsontext.String("explicit_body"))
	e.WriteToken(jsontext.Bool(n.ExplicitBody))

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Not && n.Location != nil {
		e.WriteToken(jsontext.String("location"))
		n.Location.MarshalJSONTo(e)
	}

	return e.WriteToken(jsontext.EndObject)
}

func (n *Not) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(n)
}

func (n *Not) UnmarshalJSON(bs []byte) error {
	v := map[string]any{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}

	return unmarshalNot(n, v)
}

func (obj *object) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginArray)

	for _, node := range obj.sortedKeys() {
		e.WriteToken(jsontext.BeginArray)
		node.key.MarshalJSONTo(e)
		node.value.MarshalJSONTo(e)
		e.WriteToken(jsontext.EndArray)
	}
	return e.WriteToken(jsontext.EndArray)
}

func (l *lazyObj) MarshalJSONTo(e *jsontext.Encoder) error {
	return l.force().(*object).MarshalJSONTo(e)
}

func (l *lazyObj) MarshalJSON() ([]byte, error) {
	return l.force().(*object).MarshalJSON()
}

// MarshalJSON returns JSON encoded bytes representing obj.
func (obj *object) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(obj)
}

func (a *Array) MarshalJSONTo(e *jsontext.Encoder) error {
	return util.WriteMarshalerToArray(e, a.elems)
}

// MarshalJSON returns JSON encoded bytes representing arr.
func (arr *Array) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(arr)
}

func (s *set) MarshalJSONTo(e *jsontext.Encoder) error {
	return util.WriteMarshalerToArray(e, s.sortedKeys())
}

// MarshalJSON returns JSON encoded bytes representing s.
func (s *set) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(s)
}

func (o *LogicalOr) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(o)
}

func (o *LogicalOr) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("type"))
	e.WriteToken(jsontext.String("or"))

	e.WriteToken(jsontext.String("lhs"))
	o.Lhs.MarshalJSONTo(e)

	e.WriteToken(jsontext.String("rhs"))
	o.Rhs.MarshalJSONTo(e)

	if o.ExplicitLhs {
		e.WriteToken(jsontext.String("explicit_lhs"))
		e.WriteToken(jsontext.True)
	}
	if o.ExplicitRhs {
		e.WriteToken(jsontext.String("explicit_rhs"))
		e.WriteToken(jsontext.True)
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Or && o.Location != nil {
		e.WriteToken(jsontext.String("location"))
		o.Location.MarshalJSONTo(e)
	}

	return e.WriteToken(jsontext.EndObject)
}

func (o *LogicalOr) UnmarshalJSON(bs []byte) error {
	v := map[string]any{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}
	return unmarshalLogical("or", &o.Lhs, &o.Rhs, &o.ExplicitLhs, &o.ExplicitRhs, v)
}

func marshalValueTo(e *jsontext.Encoder, val Value) (err error) {
	switch v := val.(type) {
	case json.MarshalerTo:
		err = v.MarshalJSONTo(e)
	case encoding.TextAppender:
		buf, _ := v.AppendText(e.AvailableBuffer())
		err = e.WriteValue(buf)
	default:
		err = json.MarshalEncode(e, v)
	}
	return err
}
