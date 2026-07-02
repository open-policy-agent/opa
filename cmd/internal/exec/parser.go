package exec

import (
	"io"

	"github.com/open-policy-agent/opa/v1/util"
)

type parser interface {
	Parse(io.Reader) (any, error)
}

type utilParser struct {
}

func (utilParser) Parse(r io.Reader) (any, error) {
	bs, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var x any
	return x, util.Unmarshal(bs, &x)
}
