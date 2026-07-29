//go:build go1.27

package ast

import (
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/util"
)

var _ json.Unmarshaler = &Module{}

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
	if a == nil {
		// Matches encoding/json v1, which encodes a nil slice as null rather
		// than as an empty array.
		return e.WriteToken(jsontext.Null)
	}
	return util.WriteMarshalerToArray(e, a)
}

func (m *Module) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("package"))
	m.Package.MarshalJSONTo(e)

	if len(m.Imports) > 0 {
		e.WriteToken(jsontext.String("imports"))
		util.WriteMarshalerToArray(e, m.Imports)
	}

	if len(m.Rules) > 0 {
		e.WriteToken(jsontext.String("rules"))
		util.WriteMarshalerToArray(e, m.Rules)
	}

	if len(m.Annotations) > 0 {
		e.WriteToken(jsontext.String("annotations"))
		util.WriteMarshalerToArray(e, m.Annotations)
	}

	if len(m.Comments) > 0 {
		e.WriteToken(jsontext.String("comments"))
		util.WriteMarshalerToArray(e, m.Comments)
	}

	return e.WriteToken(jsontext.EndObject)
}

func (pkg *Package) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Package && pkg.Location != nil {
		e.WriteToken(jsontext.String("location"))
		pkg.Location.MarshalJSONTo(e)
	}

	e.WriteToken(jsontext.String("path"))
	pkg.Path.MarshalJSONTo(e)

	return e.WriteToken(jsontext.EndObject)
}

func (i *Import) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("path"))
	i.Path.MarshalJSONTo(e)

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Import && i.Location != nil {
		e.WriteToken(jsontext.String("location"))
		i.Location.MarshalJSONTo(e)
	}

	if len(i.Alias) > 0 {
		e.WriteToken(jsontext.String("alias"))
		e.WriteToken(jsontext.String(string(i.Alias)))
	}

	return e.WriteToken(jsontext.EndObject)
}

func (r *Rule) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if r.Default {
		e.WriteToken(jsontext.String("default"))
		e.WriteToken(jsontext.True)
	}

	if r.Else != nil {
		e.WriteToken(jsontext.String("else"))
		r.Else.MarshalJSONTo(e)
	}

	e.WriteToken(jsontext.String("head"))
	r.Head.MarshalJSONTo(e)

	e.WriteToken(jsontext.String("body"))
	r.Body.MarshalJSONTo(e)

	if len(r.Annotations) > 0 {
		e.WriteToken(jsontext.String("annotations"))
		util.WriteMarshalerToArray(e, r.Annotations)
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Rule && r.Location != nil {
		e.WriteToken(jsontext.String("location"))
		r.Location.MarshalJSONTo(e)
	}

	return e.WriteToken(jsontext.EndObject)
}

func (h *Head) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if h.Name != "" {
		e.WriteToken(jsontext.String("name"))
		e.WriteToken(jsontext.String(string(h.Name)))
	}

	e.WriteToken(jsontext.String("ref"))
	h.Ref().MarshalJSONTo(e)

	if len(h.Args) > 0 {
		e.WriteToken(jsontext.String("args"))
		util.WriteMarshalerToArray(e, h.Args)
	}

	if h.Key != nil {
		e.WriteToken(jsontext.String("key"))
		h.Key.MarshalJSONTo(e)
	}

	if h.Value != nil {
		e.WriteToken(jsontext.String("value"))
		h.Value.MarshalJSONTo(e)
	}

	if h.Assign {
		e.WriteToken(jsontext.String("assign"))
		e.WriteToken(jsontext.True)
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Head && h.Location != nil {
		e.WriteToken(jsontext.String("location"))
		h.Location.MarshalJSONTo(e)
	}

	return e.WriteToken(jsontext.EndObject)
}

func (c Call) MarshalJSONTo(e *jsontext.Encoder) (err error) {
	return util.WriteMarshalerToArray(e, c)
}

func (c *Comment) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	// Comment has no JSON tags, hence the capitalised keys, the base64 encoded
	// text, and the location being written even when it's nil.
	e.WriteToken(jsontext.String("Text"))

	buf := make([]byte, base64.StdEncoding.EncodedLen(len(c.Text)))
	base64.StdEncoding.Encode(buf, c.Text)

	e.WriteValue(append(append(append(e.AvailableBuffer(), '"'), buf...), '"'))

	e.WriteToken(jsontext.String("Location"))
	if c.Location != nil {
		c.Location.MarshalJSONTo(e)
	} else {
		e.WriteToken(jsontext.Null)
	}

	return e.WriteToken(jsontext.EndObject)
}

func (q *Every) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("key"))
	if q.Key == nil {
		e.WriteToken(jsontext.Null)
	} else {
		if err := q.Key.MarshalJSONTo(e); err != nil {
			return err
		}
	}

	e.WriteToken(jsontext.String("value"))
	if err := q.Value.MarshalJSONTo(e); err != nil {
		return err
	}

	e.WriteToken(jsontext.String("domain"))
	if err := q.Domain.MarshalJSONTo(e); err != nil {
		return err
	}

	e.WriteToken(jsontext.String("body"))
	if err := q.Body.MarshalJSONTo(e); err != nil {
		return err
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Every && q.Location != nil {
		e.WriteToken(jsontext.String("location"))
		if err := q.Location.MarshalJSONTo(e); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (b Body) MarshalJSONTo(e *jsontext.Encoder) error {
	return util.WriteMarshalerToArray(e, b)
}

// MarshalJSON returns JSON encoded bytes representing body.
func (body Body) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(body)
}

func (expr *Expr) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(expr)
}

// UnmarshalJSON parses the byte array and stores the result in expr.
func (expr *Expr) UnmarshalJSON(bs []byte) error {
	v := map[string]any{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}
	return unmarshalExpr(expr, v)
}

func (e *Expr) MarshalJSONTo(enc *jsontext.Encoder) error {
	enc.WriteToken(jsontext.BeginObject)

	enc.WriteToken(jsontext.String("index"))
	enc.WriteToken(jsontext.Int(int64(e.Index)))

	includeLocation := astJSON.GetOptions().MarshalOptions.IncludeLocation
	if e.Location != nil && includeLocation.Expr {
		enc.WriteToken(jsontext.String("location"))
		e.Location.MarshalJSONTo(enc)
	}

	if e.Negated {
		enc.WriteToken(jsontext.String("negated"))
		enc.WriteToken(jsontext.True)
	}

	if e.Generated {
		enc.WriteToken(jsontext.String("generated"))
		enc.WriteToken(jsontext.True)
	}

	enc.WriteToken(jsontext.String("terms"))
	var err error
	switch t := e.Terms.(type) {
	case []*Term:
		err = util.WriteMarshalerToArray(enc, t)
	case json.MarshalerTo:
		err = t.MarshalJSONTo(enc)
	default:
		return fmt.Errorf("unsupported expr terms type: %T", e.Terms)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal expr terms: %w", err)
	}

	if len(e.With) > 0 {
		enc.WriteToken(jsontext.String("with"))
		util.WriteMarshalerToArray(enc, e.With)
	}

	return enc.WriteToken(jsontext.EndObject)
}

func (a *LogicalAnd) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)
	e.WriteToken(jsontext.String("type"))
	e.WriteToken(jsontext.String("and"))
	e.WriteToken(jsontext.String("lhs"))
	a.Lhs.MarshalJSONTo(e)
	e.WriteToken(jsontext.String("rhs"))
	a.Rhs.MarshalJSONTo(e)

	if a.ExplicitLhs {
		e.WriteToken(jsontext.String("explicit_lhs"))
		e.WriteToken(jsontext.True)
	}
	if a.ExplicitRhs {
		e.WriteToken(jsontext.String("explicit_rhs"))
		e.WriteToken(jsontext.True)
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.And && a.Location != nil {
		e.WriteToken(jsontext.String("location"))
		if err := a.Location.MarshalJSONTo(e); err != nil {
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

func (w *With) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("target"))
	w.Target.MarshalJSONTo(e)

	e.WriteToken(jsontext.String("value"))
	w.Value.MarshalJSONTo(e)

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.With && w.Location != nil {
		e.WriteToken(jsontext.String("location"))
		if err := w.Location.MarshalJSONTo(e); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (d *SomeDecl) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("symbols"))
	util.WriteMarshalerToArray(e, d.Symbols)

	if d.Location != nil && astJSON.GetOptions().MarshalOptions.IncludeLocation.SomeDecl {
		e.WriteToken(jsontext.String("location"))
		if err := d.Location.MarshalJSONTo(e); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (ac *ArrayComprehension) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("term"))
	ac.Term.MarshalJSONTo(e)

	e.WriteToken(jsontext.String("body"))
	ac.Body.MarshalJSONTo(e)

	return e.WriteToken(jsontext.EndObject)
}

func (sc *SetComprehension) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("term"))
	sc.Term.MarshalJSONTo(e)

	e.WriteToken(jsontext.String("body"))
	sc.Body.MarshalJSONTo(e)

	return e.WriteToken(jsontext.EndObject)
}

func (oc *ObjectComprehension) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("key"))
	oc.Key.MarshalJSONTo(e)

	e.WriteToken(jsontext.String("value"))
	oc.Value.MarshalJSONTo(e)

	e.WriteToken(jsontext.String("body"))
	oc.Body.MarshalJSONTo(e)

	return e.WriteToken(jsontext.EndObject)
}

func (pkg *Package) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(pkg)
}

func (imp *Import) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(imp)
}

func (rule *Rule) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(rule)
}

func (head *Head) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(head)
}

func (w *With) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(w)
}

func (d *SomeDecl) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(d)
}

func (q *Every) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(q)
}

func (a *LogicalAnd) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(a)
}

func (o *LogicalOr) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(o)
}
