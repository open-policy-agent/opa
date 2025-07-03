//go:build jwx_asmbase64

package base64

import (
	"fmt"
	"slices"

	asmbase64 "github.com/segmentio/asm/base64"
)

func init() {
	SetEncoder(asmEncoder{asmbase64.RawURLEncoding})
	SetDecoder(asmDecoder{})
}

type asmEncoder struct {
	*asmbase64.Encoding
}

func (e asmEncoder) AppendEncode(dst, src []byte) []byte {
	n := e.Encoding.EncodedLen(len(src))
	dst = slices.Grow(dst, n)
	e.Encoding.Encode(dst[len(dst):][:n], src)
	return dst[:len(dst)+n]
}

type asmDecoder struct{}

func (d asmDecoder) Decode(src []byte) ([]byte, error) {
	var enc *asmbase64.Encoding
	switch Guess(src) {
	case Std:
		enc = asmbase64.StdEncoding
	case RawStd:
		enc = asmbase64.RawStdEncoding
	case URL:
		enc = asmbase64.URLEncoding
	case RawURL:
		enc = asmbase64.RawURLEncoding
	default:
		return nil, fmt.Errorf(`invalid encoding`)
	}

	dst := make([]byte, enc.DecodedLen(len(src)))
	n, err := enc.Decode(dst, src)
	if err != nil {
		return nil, fmt.Errorf(`failed to decode source: %w`, err)
	}
	return dst[:n], nil
}
