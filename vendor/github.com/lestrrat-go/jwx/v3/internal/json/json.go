package json

import (
	"bytes"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/lestrrat-go/jwx/v3/internal/base64"
)

var useNumber uint32 // TODO: at some point, change to atomic.Bool

func UseNumber() bool {
	return atomic.LoadUint32(&useNumber) == 1
}

// Sets the global configuration for json decoding
func DecoderSettings(inUseNumber bool) {
	var val uint32
	if inUseNumber {
		val = 1
	}
	atomic.StoreUint32(&useNumber, val)
}

// Unmarshal respects the values specified in DecoderSettings,
// and uses a Decoder that has certain features turned on/off
func Unmarshal(b []byte, v any) error {
	dec := NewDecoder(bytes.NewReader(b))
	return dec.Decode(v)
}

func AssignNextBytesToken(dst *[]byte, dec *Decoder) error {
	var val string
	if err := dec.Decode(&val); err != nil {
		return fmt.Errorf(`error reading next value: %w`, err)
	}

	buf, err := base64.DecodeString(val)
	if err != nil {
		return fmt.Errorf(`expected base64 encoded []byte (%T)`, val)
	}
	*dst = buf
	return nil
}

func ReadNextStringToken(dec *Decoder) (string, error) {
	var val string
	if err := dec.Decode(&val); err != nil {
		return "", fmt.Errorf(`error reading next value: %w`, err)
	}
	return val, nil
}

func AssignNextStringToken(dst **string, dec *Decoder) error {
	val, err := ReadNextStringToken(dec)
	if err != nil {
		return err
	}
	*dst = &val
	return nil
}

// FlattenAudience is a flag to specify if we should flatten the "aud"
// entry to a string when there's only one entry.
// In jwx < 1.1.8 we just dumped everything as an array of strings,
// but apparently AWS Cognito doesn't handle this well.
//
// So now we have the ability to dump "aud" as a string if there's
// only one entry, but we need to retain the old behavior so that
// we don't accidentally break somebody else's code. (e.g. messing
// up how signatures are calculated)
var FlattenAudience uint32

func MarshalAudience(aud []string, flatten bool) ([]byte, error) {
	var val any
	if len(aud) == 1 && flatten {
		val = aud[0]
	} else {
		val = aud
	}
	return Marshal(val)
}

func EncodeAudience(enc *Encoder, aud []string, flatten bool) error {
	var val any
	if len(aud) == 1 && flatten {
		val = aud[0]
	} else {
		val = aud
	}
	return enc.Encode(val)
}

// DecodeCtx is an interface for objects that needs that extra something
// when decoding JSON into an object.
type DecodeCtx interface {
	Registry() *Registry
}

// DecodeCtxContainer is used to differentiate objects that can carry extra
// decoding hints and those who can't.
type DecodeCtxContainer interface {
	DecodeCtx() DecodeCtx
	SetDecodeCtx(DecodeCtx)
}

// stock decodeCtx. should cover 80% of the cases
type decodeCtx struct {
	registry *Registry
}

func NewDecodeCtx(r *Registry) DecodeCtx {
	return &decodeCtx{registry: r}
}

func (dc *decodeCtx) Registry() *Registry {
	return dc.registry
}

func Dump(v any) {
	enc := NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	//nolint:errchkjson
	_ = enc.Encode(v)
}
