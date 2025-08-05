package jwx

import "github.com/lestrrat-go/option/v2"

type identUseNumber struct{}

type Option = option.Interface

type JSONOption interface {
	Option
	isJSONOption()
}

type jsonOption struct {
	Option
}

func (o *jsonOption) isJSONOption() {}

func newJSONOption(n any, v any) JSONOption {
	return &jsonOption{option.New(n, v)}
}

// WithUseNumber controls whether the jwx package should unmarshal
// JSON objects with the "encoding/json".Decoder.UseNumber feature on.
//
// Default is false.
func WithUseNumber(b bool) JSONOption {
	return newJSONOption(identUseNumber{}, b)
}
