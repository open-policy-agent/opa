//go:build go1.27

package ast

import (
	"encoding"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/open-policy-agent/opa/internal/jsonv2"
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

	// These are exported types, so losing MarshalJSON here would be a breaking
	// API change even though callers should go through json.Marshal, not this
	// method directly.
	_ json.Marshaler = Number("")
	_ json.Marshaler = &Term{}
	_ json.Marshaler = &Not{}
	_ json.Marshaler = &lazyObj{}
	_ json.Marshaler = &object{}
	_ json.Marshaler = &Array{}
	_ json.Marshaler = &set{}
)

// These are here to ensure that we do not fall down to TextAppender, which
// Go 1.27's encoding/json would otherwise use, encoding these as JSON strings.

func (bol Boolean) MarshalJSONTo(e *jsontext.Encoder) error {
	return e.WriteToken(jsontext.Bool(bool(bol)))
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

// MarshalJSON returns JSON encoded bytes representing num.
func (num Number) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(num)
}

func (str String) MarshalJSONTo(e *jsontext.Encoder) error {
	return e.WriteToken(jsontext.String(string(str)))
}

func (term *Term) MarshalJSONTo(e *jsontext.Encoder) (err error) {
	// Token write errors are unchecked: an unbalanced value fails at the closing
	// token. A marshaller can fail having written a balanced value, so is checked.
	e.WriteToken(jsontext.BeginObject)

	includeLocation := astJSON.GetOptions().MarshalOptions.IncludeLocation
	if term.Location != nil && includeLocation.Term {
		if err := jsonv2.WriteField(e, "location", term.Location); err != nil {
			return err
		}
	}

	e.WriteToken(jsontext.String("type"))
	e.WriteToken(jsontext.String(ValueName(term.Value)))

	e.WriteToken(jsontext.String("value"))
	if err = marshalValueTo(e, term.Value); err != nil {
		return fmt.Errorf("failed to marshal term of %s: %w", ValueName(term.Value), err)
	}

	return e.WriteToken(jsontext.EndObject)
}

// MarshalJSON returns the JSON encoding of the term.
func (term *Term) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(term)
}

func (ref Ref) MarshalJSONTo(e *jsontext.Encoder) (err error) {
	return jsonv2.WriteMarshalerToArrayOrNull(e, ref)
}

func (ts *TemplateString) MarshalJSONTo(e *jsontext.Encoder) (err error) {
	// Token write errors are unchecked: an unbalanced value fails at the closing
	// token. A marshaller can fail having written a balanced value, so is checked.
	e.WriteToken(jsontext.BeginObject)
	e.WriteToken(jsontext.String("parts"))
	if ts.Parts == nil {
		// Parts has no omitempty tag, so it's always written. Matches
		// encoding/json v1, which encodes a nil slice as null rather than as an
		// empty array.
		e.WriteToken(jsontext.Null)
	} else {
		e.WriteToken(jsontext.BeginArray)
		for _, p := range ts.Parts {
			switch v := p.(type) {
			case *Expr:
				if err := v.MarshalJSONTo(e); err != nil {
					return err
				}
			case *Term:
				if err := v.MarshalJSONTo(e); err != nil {
					return err
				}
			}
		}
		e.WriteToken(jsontext.EndArray)
	}

	e.WriteToken(jsontext.String("multi_line"))
	e.WriteToken(jsontext.Bool(ts.MultiLine))

	return e.WriteToken(jsontext.EndObject)
}

func (n *Not) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)
	e.WriteToken(jsontext.String("type"))
	e.WriteToken(jsontext.String("not"))

	if err := jsonv2.WriteField(e, "body", n.Body); err != nil {
		return err
	}

	e.WriteToken(jsontext.String("explicit_body"))
	e.WriteToken(jsontext.Bool(n.ExplicitBody))

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Not && n.Location != nil {
		if err := jsonv2.WriteField(e, "location", n.Location); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (n *Not) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(n)
}

func (n *Not) UnmarshalJSON(bs []byte) error {
	v := map[string]any{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}

	return unmarshalNot(n, v)
}

func (obj *object) MarshalJSONTo(e *jsontext.Encoder) error {
	// Token write errors are unchecked: an unbalanced value fails at the closing
	// token. A marshaller can fail having written a balanced value, so is checked.
	e.WriteToken(jsontext.BeginArray)

	for _, node := range obj.sortedKeys() {
		e.WriteToken(jsontext.BeginArray)
		if err := node.key.MarshalJSONTo(e); err != nil {
			return err
		}
		if err := node.value.MarshalJSONTo(e); err != nil {
			return err
		}
		e.WriteToken(jsontext.EndArray)
	}
	return e.WriteToken(jsontext.EndArray)
}

func (lob *lazyObj) MarshalJSONTo(e *jsontext.Encoder) error {
	return lob.force().(*object).MarshalJSONTo(e)
}

func (lob *lazyObj) MarshalJSON() ([]byte, error) {
	return lob.force().(*object).MarshalJSON()
}

// MarshalJSON returns JSON encoded bytes representing obj.
func (obj *object) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(obj)
}

func (arr *Array) MarshalJSONTo(e *jsontext.Encoder) error {
	return jsonv2.WriteMarshalerToArray(e, arr.elems)
}

// MarshalJSON returns JSON encoded bytes representing arr.
func (arr *Array) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(arr)
}

func (s *set) MarshalJSONTo(e *jsontext.Encoder) error {
	return jsonv2.WriteMarshalerToArray(e, s.sortedKeys())
}

// MarshalJSON returns JSON encoded bytes representing s.
func (s *set) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(s)
}

func marshalValueTo(e *jsontext.Encoder, val Value) (err error) {
	switch v := val.(type) {
	case json.MarshalerTo:
		err = v.MarshalJSONTo(e)
	case encoding.TextAppender:
		var text []byte
		if text, err = v.AppendText(nil); err != nil {
			return err
		}

		if text, err = jsontext.AppendQuote(e.AvailableBuffer(), text); err != nil {
			return err
		}

		err = e.WriteValue(text)
	default:
		err = json.MarshalEncode(e, v)
	}

	return err
}
