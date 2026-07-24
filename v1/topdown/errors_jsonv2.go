//go:build go1.27

package topdown

import (
	"encoding/json/jsontext"
	"errors"
)

func (e *Error) MarshalJSONTo(enc *jsontext.Encoder) (err error) {
	enc.WriteToken(jsontext.BeginObject)
	enc.WriteToken(jsontext.String("code"))
	enc.WriteToken(jsontext.String(e.Code))
	enc.WriteToken(jsontext.String("message"))
	enc.WriteToken(jsontext.String(e.Message))

	if e.Location != nil {
		enc.WriteToken(jsontext.String("location"))
		err = e.Location.MarshalJSONTo(enc)
	}

	return errors.Join(err, enc.WriteToken(jsontext.EndObject))
}
