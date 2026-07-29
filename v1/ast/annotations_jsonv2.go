//go:build go1.27

package ast

import (
	"encoding/json/jsontext"
	"encoding/json/v2"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/util"
)

func (a *Annotations) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if a == nil {
		e.WriteToken(jsontext.String("scope"))
		e.WriteToken(jsontext.String(""))
		return e.WriteToken(jsontext.EndObject)
	}

	if a.Description != "" {
		e.WriteToken(jsontext.String("description"))
		e.WriteToken(jsontext.String(a.Description))
	}

	if a.Entrypoint {
		e.WriteToken(jsontext.String("entrypoint"))
		e.WriteToken(jsontext.True)
	}

	if len(a.Organizations) > 0 {
		e.WriteToken(jsontext.String("organizations"))
		json.MarshalEncode(e, a.Organizations)
	}

	if len(a.RelatedResources) > 0 {
		e.WriteToken(jsontext.String("related_resources"))
		util.WriteMarshalerToArray(e, a.RelatedResources)
	}

	if len(a.Authors) > 0 {
		e.WriteToken(jsontext.String("authors"))
		util.WriteMarshalerToArray(e, a.Authors)
	}

	if len(a.Schemas) > 0 {
		e.WriteToken(jsontext.String("schemas"))
		util.WriteMarshalerToArray(e, a.Schemas)
	}

	if a.Compile != nil {
		e.WriteToken(jsontext.String("compile"))
		json.MarshalEncode(e, a.Compile)
	}

	if len(a.Custom) > 0 {
		e.WriteToken(jsontext.String("custom"))
		json.MarshalEncode(e, a.Custom)
	}

	if len(a.Labels) > 0 {
		e.WriteToken(jsontext.String("labels"))
		json.MarshalEncode(e, a.Labels)
	}

	e.WriteToken(jsontext.String("scope"))
	e.WriteToken(jsontext.String(a.Scope))

	if a.Title != "" {
		e.WriteToken(jsontext.String("title"))
		e.WriteToken(jsontext.String(a.Title))
	}

	if a.Location != nil && astJSON.GetOptions().MarshalOptions.IncludeLocation.Annotations {
		e.WriteToken(jsontext.String("location"))
		a.Location.MarshalJSONTo(e)
	}

	return e.WriteToken(jsontext.EndObject)
}

func (a *Annotations) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(a)
}

func (ar *AnnotationsRef) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if ar.Annotations != nil {
		e.WriteToken(jsontext.String("annotations"))
		ar.Annotations.MarshalJSONTo(e)
	}

	if ar.Location != nil && astJSON.GetOptions().MarshalOptions.IncludeLocation.AnnotationsRef {
		e.WriteToken(jsontext.String("location"))
		ar.Location.MarshalJSONTo(e)
	}

	e.WriteToken(jsontext.String("path"))
	if err := ar.Path.MarshalJSONTo(e); err != nil {
		return err
	}

	return e.WriteToken(jsontext.EndObject)
}

func (ar *AnnotationsRef) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(ar)
}

func (s *SchemaAnnotation) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	// Path has no omitempty tag, so it's always written. A nil ref is written
	// as null, matching encoding/json v1's treatment of a nil slice.
	e.WriteToken(jsontext.String("path"))
	if s.Path == nil {
		e.WriteToken(jsontext.Null)
	} else {
		e.WriteToken(jsontext.BeginArray)
		for _, t := range s.Path {
			// The location is omitted: path terms are parsed on their own from
			// the annotation's YAML key, so their locations are offsets into that
			// key (always row 1) rather than positions in the module.
			e.WriteToken(jsontext.BeginObject)
			e.WriteToken(jsontext.String("type"))
			e.WriteToken(jsontext.String(ValueName(t.Value)))
			e.WriteToken(jsontext.String("value"))
			marshalValueTo(e, t.Value)
			e.WriteToken(jsontext.EndObject)
		}
		e.WriteToken(jsontext.EndArray)
	}

	if len(s.Schema) > 0 {
		e.WriteToken(jsontext.String("schema"))
		s.Schema.MarshalJSONTo(e)
	}

	if s.Definition != nil {
		e.WriteToken(jsontext.String("definition"))
		if err := json.MarshalEncode(e, s.Definition); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (rr *RelatedResourceAnnotation) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(rr)
}

func (rr *RelatedResourceAnnotation) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	e.WriteToken(jsontext.String("ref"))
	e.WriteToken(jsontext.String(rr.Ref.String()))

	if len(rr.Description) > 0 {
		e.WriteToken(jsontext.String("description"))
		e.WriteToken(jsontext.String(rr.Description))
	}

	return e.WriteToken(jsontext.EndObject)
}

func (a *AuthorAnnotation) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	// Name has no omitempty tag, so it's always written.
	e.WriteToken(jsontext.String("name"))
	e.WriteToken(jsontext.String(a.Name))

	if len(a.Email) > 0 {
		e.WriteToken(jsontext.String("email"))
		e.WriteToken(jsontext.String(a.Email))
	}

	return e.WriteToken(jsontext.EndObject)
}
