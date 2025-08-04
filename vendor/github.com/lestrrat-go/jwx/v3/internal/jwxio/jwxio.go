package jwxio

import (
	"bytes"
	"errors"
	"io"
	"strings"
)

var errNonFiniteSource = errors.New(`cannot read from non-finite source`)

func NonFiniteSourceError() error {
	return errNonFiniteSource
}

// ReadAllFromFiniteSource reads all data from a io.Reader _if_ it comes from a
// finite source.
func ReadAllFromFiniteSource(rdr io.Reader) ([]byte, error) {
	switch rdr.(type) {
	case *bytes.Reader, *bytes.Buffer, *strings.Reader:
		data, err := io.ReadAll(rdr)
		if err != nil {
			return nil, err
		}
		return data, nil
	default:
		return nil, errNonFiniteSource
	}
}
