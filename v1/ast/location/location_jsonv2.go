// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.27

package location

import (
	"encoding/base64"
	"encoding/json/jsontext"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
)

func (loc *Location) MarshalJSONTo(e *jsontext.Encoder) (err error) {
	e.WriteToken(jsontext.BeginObject)

	jsonOptions := astJSON.GetOptions().MarshalOptions
	if !jsonOptions.ExcludeLocationFile {
		e.WriteToken(jsontext.String("file"))
		e.WriteToken(jsontext.String(loc.File))
	}

	e.WriteToken(jsontext.String("row"))
	e.WriteToken(jsontext.Int(int64(loc.Row)))
	e.WriteToken(jsontext.String("col"))
	e.WriteToken(jsontext.Int(int64(loc.Col)))

	// NOTE: len check to match the `json:"text,omitempty"` behaviour of the
	// pre-go1.27 marshaller.
	if jsonOptions.IncludeLocationText && len(loc.Text) > 0 {
		e.WriteToken(jsontext.String("text"))
		e.WriteToken(jsontext.String(base64.StdEncoding.EncodeToString(loc.Text)))
	}

	return e.WriteToken(jsontext.EndObject)
}
