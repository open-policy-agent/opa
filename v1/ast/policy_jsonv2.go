//go:build go1.27

package ast

import (
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/open-policy-agent/opa/internal/jsonv2"
	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/util"
)

var (
	_ json.Unmarshaler = &Module{}

	// These are exported types, so losing MarshalJSON here would be a breaking
	// API change even though callers should go through json.Marshal, not this
	// method directly.
	_ json.Marshaler = Body{}
	_ json.Marshaler = &Expr{}
	_ json.Marshaler = &Package{}
	_ json.Marshaler = &Import{}
	_ json.Marshaler = &Rule{}
	_ json.Marshaler = &Head{}
	_ json.Marshaler = &With{}
	_ json.Marshaler = &SomeDecl{}
	_ json.Marshaler = &Every{}
	_ json.Marshaler = &LogicalAnd{}
	_ json.Marshaler = &LogicalOr{}
)

// UnmarshalJSON parses bs and stores the result in mod. The rules in the module
// will have their module pointer set to mod.
func (mod *Module) UnmarshalJSON(bs []byte) error {

	// Declare a new type and use a type conversion to avoid recursively calling
	// Module#UnmarshalJSON.
	type module Module

	if err := util.UnmarshalJSON(bs, (*module)(mod)); err != nil {
		return err
	}

	// The decoded rules have no module pointer, as it isn't part of the JSON
	// representation; without this, an unmarshalled module can't be compiled.
	WalkRules(mod, func(rule *Rule) bool {
		rule.Module = mod
		return false
	})

	return nil
}

// MarshalJSONTo is here to ensure that we do not fall down to TextAppender,
// which Go 1.27's encoding/json would otherwise use, encoding args as the Rego
// representation of the argument list rather than as a JSON array.
func (a Args) MarshalJSONTo(e *jsontext.Encoder) error {
	return jsonv2.WriteMarshalerToArrayOrNull(e, a)
}

// MarshalJSONTo is here to ensure that we do not fall down to TextAppender,
// which Go 1.27's encoding/json would otherwise use, encoding the module as
// Rego source rather than as JSON. Module's own fields are fully described by
// their struct tags, so the encoding is left to them, as it is pre-1.27. The
// field types provide their own MarshalJSONTo where one is needed.
func (mod *Module) MarshalJSONTo(e *jsontext.Encoder) error {
	// Declare a new type and use a type conversion to avoid recursively calling
	// Module#MarshalJSONTo. It's the highest precedence marshaller, so there is
	// nothing below it to fall to, and the new type has no methods of its own.
	type module Module

	return json.MarshalEncode(e, (*module)(mod))
}

func (pkg *Package) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Package && pkg.Location != nil {
		if err := jsonv2.WriteField(e, "location", pkg.Location); err != nil {
			return err
		}
	}

	if err := jsonv2.WriteField(e, "path", pkg.Path); err != nil {
		return err
	}

	return e.WriteToken(jsontext.EndObject)
}

func (imp *Import) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if err := jsonv2.WriteField(e, "path", imp.Path); err != nil {
		return err
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Import && imp.Location != nil {
		if err := jsonv2.WriteField(e, "location", imp.Location); err != nil {
			return err
		}
	}

	if len(imp.Alias) > 0 {
		e.WriteToken(jsontext.String("alias"))
		e.WriteToken(jsontext.String(string(imp.Alias)))
	}

	return e.WriteToken(jsontext.EndObject)
}

func (rule *Rule) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if rule.Default {
		e.WriteToken(jsontext.String("default"))
		e.WriteToken(jsontext.True)
	}

	if rule.Else != nil {
		if err := jsonv2.WriteField(e, "else", rule.Else); err != nil {
			return err
		}
	}

	if err := jsonv2.WriteField(e, "head", rule.Head); err != nil {
		return err
	}

	if err := jsonv2.WriteField(e, "body", rule.Body); err != nil {
		return err
	}

	if len(rule.Annotations) > 0 {
		if err := jsonv2.WriteFieldArray(e, "annotations", rule.Annotations); err != nil {
			return err
		}
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Rule && rule.Location != nil {
		if err := jsonv2.WriteField(e, "location", rule.Location); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (head *Head) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if head.Name != "" {
		e.WriteToken(jsontext.String("name"))
		e.WriteToken(jsontext.String(string(head.Name)))
	}

	if err := jsonv2.WriteField(e, "ref", head.Ref()); err != nil {
		return err
	}

	if len(head.Args) > 0 {
		if err := jsonv2.WriteFieldArray(e, "args", head.Args); err != nil {
			return err
		}
	}

	if head.Key != nil {
		if err := jsonv2.WriteField(e, "key", head.Key); err != nil {
			return err
		}
	}

	if head.Value != nil {
		if err := jsonv2.WriteField(e, "value", head.Value); err != nil {
			return err
		}
	}

	if head.Assign {
		e.WriteToken(jsontext.String("assign"))
		e.WriteToken(jsontext.True)
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Head && head.Location != nil {
		if err := jsonv2.WriteField(e, "location", head.Location); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (c Call) MarshalJSONTo(e *jsontext.Encoder) (err error) {
	return jsonv2.WriteMarshalerToArrayOrNull(e, c)
}

func (c *Comment) MarshalJSONTo(e *jsontext.Encoder) error {
	// Token write errors are unchecked: an unbalanced value fails at the closing
	// token. A marshaller can fail having written a balanced value, so is checked.
	e.WriteToken(jsontext.BeginObject)

	// Comment has no JSON tags, hence the capitalised keys, the base64 encoded
	// text, and the location being written even when it's nil.
	e.WriteToken(jsontext.String("Text"))

	buf := make([]byte, base64.StdEncoding.EncodedLen(len(c.Text)))
	base64.StdEncoding.Encode(buf, c.Text)

	e.WriteValue(append(append(append(e.AvailableBuffer(), '"'), buf...), '"'))

	e.WriteToken(jsontext.String("Location"))
	if c.Location != nil {
		if err := c.Location.MarshalJSONTo(e); err != nil {
			return err
		}
	} else {
		e.WriteToken(jsontext.Null)
	}

	return e.WriteToken(jsontext.EndObject)
}

func (q *Every) MarshalJSONTo(e *jsontext.Encoder) error {
	// Token write errors are unchecked: an unbalanced value fails at the closing
	// token. A marshaller can fail having written a balanced value, so is checked.
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("key"))
	if q.Key == nil {
		e.WriteToken(jsontext.Null)
	} else {
		if err := q.Key.MarshalJSONTo(e); err != nil {
			return err
		}
	}

	if err := jsonv2.WriteField(e, "value", q.Value); err != nil {
		return err
	}

	if err := jsonv2.WriteField(e, "domain", q.Domain); err != nil {
		return err
	}

	if err := jsonv2.WriteField(e, "body", q.Body); err != nil {
		return err
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Every && q.Location != nil {
		if err := jsonv2.WriteField(e, "location", q.Location); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (body Body) MarshalJSONTo(e *jsontext.Encoder) error {
	return jsonv2.WriteMarshalerToArray(e, body)
}

// MarshalJSON returns JSON encoded bytes representing body.
func (body Body) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(body)
}

func (expr *Expr) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(expr)
}

// UnmarshalJSON parses the byte array and stores the result in expr.
func (expr *Expr) UnmarshalJSON(bs []byte) error {
	v := map[string]any{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}
	return unmarshalExpr(expr, v)
}

func (expr *Expr) MarshalJSONTo(enc *jsontext.Encoder) error {
	enc.WriteToken(jsontext.BeginObject)

	enc.WriteToken(jsontext.String("index"))
	enc.WriteToken(jsontext.Int(int64(expr.Index)))

	includeLocation := astJSON.GetOptions().MarshalOptions.IncludeLocation
	if expr.Location != nil && includeLocation.Expr {
		if err := jsonv2.WriteField(enc, "location", expr.Location); err != nil {
			return err
		}
	}

	if expr.Negated {
		enc.WriteToken(jsontext.String("negated"))
		enc.WriteToken(jsontext.True)
	}

	if expr.Generated {
		enc.WriteToken(jsontext.String("generated"))
		enc.WriteToken(jsontext.True)
	}

	enc.WriteToken(jsontext.String("terms"))
	var err error
	switch t := expr.Terms.(type) {
	case []*Term:
		err = jsonv2.WriteMarshalerToArrayOrNull(enc, t)
	case json.MarshalerTo:
		err = t.MarshalJSONTo(enc)
	default:
		return fmt.Errorf("unsupported expr terms type: %T", expr.Terms)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal expr terms: %w", err)
	}

	if len(expr.With) > 0 {
		if err := jsonv2.WriteFieldArray(enc, "with", expr.With); err != nil {
			return err
		}
	}

	return enc.WriteToken(jsontext.EndObject)
}

func (a *LogicalAnd) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)
	e.WriteToken(jsontext.String("type"))
	e.WriteToken(jsontext.String("and"))
	if err := jsonv2.WriteField(e, "lhs", a.Lhs); err != nil {
		return err
	}
	if err := jsonv2.WriteField(e, "rhs", a.Rhs); err != nil {
		return err
	}

	if a.ExplicitLhs {
		e.WriteToken(jsontext.String("explicit_lhs"))
		e.WriteToken(jsontext.True)
	}
	if a.ExplicitRhs {
		e.WriteToken(jsontext.String("explicit_rhs"))
		e.WriteToken(jsontext.True)
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.And && a.Location != nil {
		if err := jsonv2.WriteField(e, "location", a.Location); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (a *LogicalAnd) UnmarshalJSON(bs []byte) error {
	v := map[string]any{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}
	return unmarshalLogical("and", &a.Lhs, &a.Rhs, &a.ExplicitLhs, &a.ExplicitRhs, v)
}

func (o *LogicalOr) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("type"))
	e.WriteToken(jsontext.String("or"))

	if err := jsonv2.WriteField(e, "lhs", o.Lhs); err != nil {
		return err
	}

	if err := jsonv2.WriteField(e, "rhs", o.Rhs); err != nil {
		return err
	}

	if o.ExplicitLhs {
		e.WriteToken(jsontext.String("explicit_lhs"))
		e.WriteToken(jsontext.True)
	}
	if o.ExplicitRhs {
		e.WriteToken(jsontext.String("explicit_rhs"))
		e.WriteToken(jsontext.True)
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Or && o.Location != nil {
		if err := jsonv2.WriteField(e, "location", o.Location); err != nil {
			return err
		}
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

func (w *With) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if err := jsonv2.WriteField(e, "target", w.Target); err != nil {
		return err
	}

	if err := jsonv2.WriteField(e, "value", w.Value); err != nil {
		return err
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.With && w.Location != nil {
		if err := jsonv2.WriteField(e, "location", w.Location); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (d *SomeDecl) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("symbols"))
	if err := jsonv2.WriteMarshalerToArrayOrNull(e, d.Symbols); err != nil {
		return err
	}

	if d.Location != nil && astJSON.GetOptions().MarshalOptions.IncludeLocation.SomeDecl {
		if err := jsonv2.WriteField(e, "location", d.Location); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (ac *ArrayComprehension) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if err := jsonv2.WriteField(e, "term", ac.Term); err != nil {
		return err
	}

	if err := jsonv2.WriteField(e, "body", ac.Body); err != nil {
		return err
	}

	return e.WriteToken(jsontext.EndObject)
}

func (sc *SetComprehension) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if err := jsonv2.WriteField(e, "term", sc.Term); err != nil {
		return err
	}

	if err := jsonv2.WriteField(e, "body", sc.Body); err != nil {
		return err
	}

	return e.WriteToken(jsontext.EndObject)
}

func (oc *ObjectComprehension) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if err := jsonv2.WriteField(e, "key", oc.Key); err != nil {
		return err
	}

	if err := jsonv2.WriteField(e, "value", oc.Value); err != nil {
		return err
	}

	if err := jsonv2.WriteField(e, "body", oc.Body); err != nil {
		return err
	}

	return e.WriteToken(jsontext.EndObject)
}

func (pkg *Package) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(pkg)
}

func (imp *Import) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(imp)
}

func (rule *Rule) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(rule)
}

func (head *Head) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(head)
}

func (w *With) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(w)
}

func (d *SomeDecl) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(d)
}

func (q *Every) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(q)
}

func (a *LogicalAnd) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(a)
}

func (o *LogicalOr) MarshalJSON() ([]byte, error) {
	return jsonv2.MarshalMarshalerTo(o)
}
