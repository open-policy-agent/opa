//go:build !go1.27

package ast

import (
	"encoding/json"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
)

func (a *Annotations) MarshalJSON() ([]byte, error) {
	if a == nil {
		return []byte(`{"scope":""}`), nil
	}

	data := map[string]any{
		"scope": a.Scope,
	}

	if a.Title != "" {
		data["title"] = a.Title
	}

	if a.Description != "" {
		data["description"] = a.Description
	}

	if a.Entrypoint {
		data["entrypoint"] = a.Entrypoint
	}

	if len(a.Organizations) > 0 {
		data["organizations"] = a.Organizations
	}

	if len(a.RelatedResources) > 0 {
		data["related_resources"] = a.RelatedResources
	}

	if len(a.Authors) > 0 {
		data["authors"] = a.Authors
	}

	if len(a.Schemas) > 0 {
		data["schemas"] = a.Schemas
	}

	if a.Compile != nil {
		data["compile"] = a.Compile
	}

	if len(a.Custom) > 0 {
		data["custom"] = a.Custom
	}

	if len(a.Labels) > 0 {
		data["labels"] = a.Labels
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.Annotations {
		if a.Location != nil {
			data["location"] = a.Location
		}
	}

	return json.Marshal(data)
}

func (rr *RelatedResourceAnnotation) MarshalJSON() ([]byte, error) {
	d := map[string]any{
		"ref": rr.Ref.String(),
	}

	if len(rr.Description) > 0 {
		d["description"] = rr.Description
	}

	return json.Marshal(d)
}

func (ar *AnnotationsRef) MarshalJSON() ([]byte, error) {
	data := map[string]any{
		"path": ar.Path,
	}

	if ar.Annotations != nil {
		data["annotations"] = ar.Annotations
	}

	if astJSON.GetOptions().MarshalOptions.IncludeLocation.AnnotationsRef {
		if ar.Location != nil {
			data["location"] = ar.Location
		}
	}

	return json.Marshal(data)
}

// schemaAnnotationJSON mirrors SchemaAnnotation's JSON tags, with location-free
// path terms.
type schemaAnnotationJSON struct {
	Path       []termJSON `json:"path"`
	Schema     Ref        `json:"schema,omitempty"`
	Definition *any       `json:"definition,omitempty"`
}

func (s *SchemaAnnotation) MarshalJSON() ([]byte, error) {
	d := schemaAnnotationJSON{
		Schema:     s.Schema,
		Definition: s.Definition,
	}

	if s.Path != nil {
		d.Path = make([]termJSON, len(s.Path))
		for i, t := range s.Path {
			// The location is omitted: path terms are parsed on their own from
			// the annotation's YAML key, so their locations are offsets into that
			// key (always row 1) rather than positions in the module.
			d.Path[i] = termJSON{Type: ValueName(t.Value), Value: t.Value}
		}
	}

	return json.Marshal(d)
}
