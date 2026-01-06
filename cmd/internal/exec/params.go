package exec

import (
	"errors"
	"io"
	"time"

	"github.com/open-policy-agent/opa/cmd/formats"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/util"
)

type Params struct {
	Output              io.Writer
	Logger              logging.Logger
	OutputFormat        *util.EnumFlag
	LogLevel            *util.EnumFlag
	LogFormat           *util.EnumFlag
	LogTimestampFormat  string
	ConfigFile          string
	Decision            string
	ConfigOverrideFiles []string
	BundlePaths         []string
	Paths               []string
	ConfigOverrides     []string
	Timeout             time.Duration
	Fail                bool
	FailDefined         bool
	FailNonEmpty        bool
	StdIn               bool
	V0Compatible        bool
	V1Compatible        bool
}

func NewParams(w io.Writer) *Params {
	return &Params{
		Output:       w,
		OutputFormat: formats.Flag(formats.JSON),
		LogLevel:     util.NewEnumFlag("error", []string{"debug", "info", "error"}),
		LogFormat:    util.NewEnumFlag("json", []string{"text", "json", "json-pretty"}),
	}
}

func (p *Params) validateParams() error {
	if p.Fail && p.FailDefined {
		return errors.New("specify --fail or --fail-defined but not both")
	}
	if p.FailNonEmpty && p.Fail {
		return errors.New("specify --fail-non-empty or --fail but not both")
	}
	if p.FailNonEmpty && p.FailDefined {
		return errors.New("specify --fail-non-empty or --fail-defined but not both")
	}
	return nil
}
