//go:build !go1.27

package location

import (
	"encoding/json"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
)

func (loc *Location) MarshalJSON() ([]byte, error) {
	// structs are used here to preserve the field ordering of the original Location struct
	jsonOptions := astJSON.GetOptions().MarshalOptions
	if jsonOptions.ExcludeLocationFile {
		data := struct {
			Row  int    `json:"row"`
			Col  int    `json:"col"`
			Text []byte `json:"text,omitempty"`
		}{
			Row: loc.Row,
			Col: loc.Col,
		}

		if jsonOptions.IncludeLocationText {
			data.Text = loc.Text
		}

		return json.Marshal(data)
	}

	data := struct {
		File string `json:"file"`
		Row  int    `json:"row"`
		Col  int    `json:"col"`
		Text []byte `json:"text,omitempty"`
	}{
		Row:  loc.Row,
		Col:  loc.Col,
		File: loc.File,
	}

	if jsonOptions.IncludeLocationText {
		data.Text = loc.Text
	}

	return json.Marshal(data)
}
