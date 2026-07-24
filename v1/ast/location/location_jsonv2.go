// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build go1.27

package location

import (
	"encoding/base64"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"

	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
)

var (
	UnmarshalFromFunc = json.UnmarshalFromFunc(UnmarshalFromFn)
	UnmarshalOpts     = json.WithUnmarshalers(json.JoinUnmarshalers(UnmarshalFromFunc))
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

	if jsonOptions.IncludeLocationText {
		e.WriteToken(jsontext.String("text"))
		e.WriteToken(jsontext.String(base64.StdEncoding.EncodeToString(loc.Text)))
	}

	return e.WriteToken(jsontext.EndObject)
}

func UnmarshalFromFn(d *jsontext.Decoder, loc *Location) error {
	if d.PeekKind() != jsontext.KindBeginObject {
		return nil
	}
	tok, err := d.ReadToken()

	for {
		if err != nil || d.PeekKind() == jsontext.KindEndObject {
			break
		}

		if tok, err = d.ReadToken(); err == nil {
			key := tok.String()
			if tok, err = d.ReadToken(); err != nil {
				break
			}
			switch key {
			case "file":
				loc.File = tok.String()
			case "row":
				loc.Row, err = readInt(tok, "row")
			case "col":
				loc.Col, err = readInt(tok, "col")
			case "text", "tabs": // marked as ignored
				d.SkipValue()
			default:
				return fmt.Errorf("unexpected key '%s' in location object", key)
			}
		}
	}

	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	_, err = d.ReadToken()

	return err
}

func readInt(tok jsontext.Token, key string) (int, error) {
	val, err := tok.Int()
	if err != nil {
		return 0, fmt.Errorf("expected integer value for '%s', got %v: %w", key, tok, err)
	}
	return int(val), nil
}
