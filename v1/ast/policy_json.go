//go:build !go1.27

package ast

import (
	"encoding/json"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/util"
)

// ruleJSON is used for JSON serialization of Rule to avoid map allocation overhead.
// Field order is alphabetical to match previous map-based output.
type ruleJSON struct {
	Annotations []*Annotations `json:"annotations,omitempty"`
	Body        Body           `json:"body"`
	Default     bool           `json:"default,omitempty"`
	Else        *Rule          `json:"else,omitempty"`
	Head        *Head          `json:"head"`
	Location    *Location      `json:"location,omitempty"`
}

// exprJSON is used for JSON serialization of Expr to avoid map allocation overhead.
// Field order is alphabetical to match previous map-based output.
type exprJSON struct {
	Generated bool      `json:"generated,omitempty"`
	Index     int       `json:"index"`
	Location  *Location `json:"location,omitempty"`
	Negated   bool      `json:"negated,omitempty"`
	Terms     any       `json:"terms"`
	With      []*With   `json:"with,omitempty"`
}

// withJSON is used for JSON serialization of With to avoid map allocation overhead.
// Field order is alphabetical to match previous map-based output.
type withJSON struct {
	Location *Location `json:"location,omitempty"`
	Target   *Term     `json:"target"`
	Value    *Term     `json:"value"`
}

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

func (d *SomeDecl) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"symbols": d.Symbols,
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.SomeDecl {
		if d.Location != nil {
			data["location"] = d.Location
		}
	}

	return json.Marshal(data)
}

func (q *Every) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"key":    q.Key,
		"value":  q.Value,
		"domain": q.Domain,
		"body":   q.Body,
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Every {
		if q.Location != nil {
			data["location"] = q.Location
		}
	}

	return json.Marshal(data)
}

func (a *LogicalAnd) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"type": "and",
		"lhs":  a.Lhs,
		"rhs":  a.Rhs,
	}
	if a.ExplicitLhs {
		data["explicit_lhs"] = true
	}
	if a.ExplicitRhs {
		data["explicit_rhs"] = true
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.And {
		if a.Location != nil {
			data["location"] = a.Location
		}
	}

	return json.Marshal(data)
}

func (a *LogicalAnd) UnmarshalJSON(bs []byte) error {
	v := map[string]any{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}
	return unmarshalLogical("and", &a.Lhs, &a.Rhs, &a.ExplicitLhs, &a.ExplicitRhs, v)
}

func (o *LogicalOr) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"type": "or",
		"lhs":  o.Lhs,
		"rhs":  o.Rhs,
	}
	if o.ExplicitLhs {
		data["explicit_lhs"] = true
	}
	if o.ExplicitRhs {
		data["explicit_rhs"] = true
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Or {
		if o.Location != nil {
			data["location"] = o.Location
		}
	}

	return json.Marshal(data)
}

func (o *LogicalOr) UnmarshalJSON(bs []byte) error {
	v := map[string]any{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}
	return unmarshalLogical("or", &o.Lhs, &o.Rhs, &o.ExplicitLhs, &o.ExplicitRhs, v)
}

// UnmarshalJSON parses the byte array and stores the result in expr.
func (expr *Expr) UnmarshalJSON(bs []byte) error {
	v := map[string]any{}
	if err := util.UnmarshalJSON(bs, &v); err != nil {
		return err
	}
	return unmarshalExpr(expr, v)
}

func (expr *Expr) MarshalJSON() ([]byte, error) {
	data := exprJSON{
		Index: expr.Index,
		Terms: expr.Terms,
	}

	if len(expr.With) > 0 {
		data.With = expr.With
	}

	if expr.Generated {
		data.Generated = true
	}

	if expr.Negated {
		data.Negated = true
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Expr {
		data.Location = expr.Location
	}

	return json.Marshal(data)
}

func (w *With) MarshalJSON() ([]byte, error) {
	data := withJSON{
		Target: w.Target,
		Value:  w.Value,
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.With {
		data.Location = w.Location
	}

	return json.Marshal(data)
}

func (pkg *Package) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"path": pkg.Path,
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Package {
		if pkg.Location != nil {
			data["location"] = pkg.Location
		}
	}

	return json.Marshal(data)
}

func (imp *Import) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"path": imp.Path,
	}

	if len(imp.Alias) != 0 {
		data["alias"] = imp.Alias
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Import {
		if imp.Location != nil {
			data["location"] = imp.Location
		}
	}

	return json.Marshal(data)
}

func (rule *Rule) MarshalJSON() ([]byte, error) {
	data := ruleJSON{
		Head: rule.Head,
		Body: rule.Body,
	}

	if rule.Default {
		data.Default = true
	}

	if rule.Else != nil {
		data.Else = rule.Else
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Rule {
		data.Location = rule.Location
	}

	if len(rule.Annotations) != 0 {
		data.Annotations = rule.Annotations
	}

	return json.Marshal(data)
}

func (head *Head) MarshalJSON() ([]byte, error) {
	var loc *Location
	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Head && head.Location != nil {
		loc = head.Location
	}

	// NOTE(sr): we do this to override the rendering of `head.Reference`.
	// It's still what'll be used via the default means of encoding/json
	// for unmarshaling a json object into a Head struct!
	type h Head
	return json.Marshal(struct {
		h
		Ref      Ref       `json:"ref"`
		Location *Location `json:"location,omitempty"`
	}{
		h:        h(*head),
		Ref:      head.Ref(),
		Location: loc,
	})
}

// MarshalJSON returns JSON encoded bytes representing body.
func (body Body) MarshalJSON() ([]byte, error) {
	// Serialize empty Body to empty array. This handles both the empty case and the
	// nil case (whereas by default the result would be null if body was nil.)
	if len(body) == 0 {
		return []byte(`[]`), nil
	}
	ret, err := json.Marshal([]*Expr(body))
	return ret, err
}
