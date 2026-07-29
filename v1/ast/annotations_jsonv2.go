//go:build go1.27

package ast

import (
	"encoding/json/jsontext"
	"fmt"

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
		if err := util.WriteFieldValue(e, "organizations", a.Organizations); err != nil {
			return err
		}
	}

	if len(a.RelatedResources) > 0 {
		if err := util.WriteFieldArray(e, "related_resources", a.RelatedResources); err != nil {
			return err
		}
	}

	if len(a.Authors) > 0 {
		if err := util.WriteFieldArray(e, "authors", a.Authors); err != nil {
			return err
		}
	}

	if len(a.Schemas) > 0 {
		if err := util.WriteFieldArray(e, "schemas", a.Schemas); err != nil {
			return err
		}
	}

	if a.Compile != nil {
		if err := util.WriteFieldValue(e, "compile", a.Compile); err != nil {
			return err
		}
	}

	if len(a.Custom) > 0 {
		if err := util.WriteFieldValue(e, "custom", a.Custom); err != nil {
			return err
		}
	}

	if len(a.Labels) > 0 {
		if err := util.WriteFieldValue(e, "labels", a.Labels); err != nil {
			return err
		}
	}

	e.WriteToken(jsontext.String("scope"))
	e.WriteToken(jsontext.String(a.Scope))

	if a.Title != "" {
		e.WriteToken(jsontext.String("title"))
		e.WriteToken(jsontext.String(a.Title))
	}

	if a.Location != nil && astJSON.GetOptions().MarshalOptions.IncludeLocation.Annotations {
		if err := util.WriteField(e, "location", a.Location); err != nil {
			return err
		}
	}

	return e.WriteToken(jsontext.EndObject)
}

func (a *Annotations) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(a)
}

func (ar *AnnotationsRef) MarshalJSONTo(e *jsontext.Encoder) error {
	e.WriteToken(jsontext.BeginObject)

	if ar.Annotations != nil {
		if err := util.WriteField(e, "annotations", ar.Annotations); err != nil {
			return err
		}
	}

	if ar.Location != nil && astJSON.GetOptions().MarshalOptions.IncludeLocation.AnnotationsRef {
		if err := util.WriteField(e, "location", ar.Location); err != nil {
			return err
		}
	}

	if err := util.WriteField(e, "path", ar.Path); err != nil {
		return err
	}

	return e.WriteToken(jsontext.EndObject)
}

func (ar *AnnotationsRef) MarshalJSON() ([]byte, error) {
	return util.MarshalMarshalerTo(ar)
}

func (s *SchemaAnnotation) MarshalJSONTo(e *jsontext.Encoder) error {
	// Token write errors are unchecked: an unbalanced value fails at the closing
	// token. A marshaller can fail having written a balanced value, so is checked.
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
			if err := marshalValueTo(e, t.Value); err != nil {
				return fmt.Errorf("failed to marshal schema path term of %s: %w", ValueName(t.Value), err)
			}
			e.WriteToken(jsontext.EndObject)
		}
		e.WriteToken(jsontext.EndArray)
	}

	if len(s.Schema) > 0 {
		if err := util.WriteField(e, "schema", s.Schema); err != nil {
			return err
		}
	}

	if s.Definition != nil {
		if err := util.WriteFieldValue(e, "definition", s.Definition); err != nil {
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
