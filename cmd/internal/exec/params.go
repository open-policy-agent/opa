package exec

import (
	"errors"
	"io"
	"time"

	"github.com/open-policy-agent/opa/logging"
	"github.com/open-policy-agent/opa/util"
)

type Params struct {
	Paths               []string       // file paths to execute against
	Output              io.Writer      // output stream to write normal output to
	ConfigFile          string         // OPA configuration file path
	ConfigOverrides     []string       // OPA configuration overrides (--set arguments)
	ConfigOverrideFiles []string       // OPA configuration overrides (--set-file arguments)
	OutputFormat        *util.EnumFlag // output format (default: pretty)
	LogLevel            *util.EnumFlag // log level for plugins
	LogFormat           *util.EnumFlag // log format for plugins
	LogTimestampFormat  string         // log timestamp format for plugins
	BundlePaths         []string       // explicit paths of bundles to inject into the configuration
	Decision            string         // decision to evaluate (overrides default decision set by configuration)
	Fail                bool           // exits with non-zero exit code on undefined policy decision or empty policy decision result or other errors
	FailDefined         bool           // exits with non-zero exit code on 'not undefined policy decisiondefined' or 'not empty policy decision result' or other errors
	FailNonEmpty        bool           // exits with non-zero exit code on non-empty set (array) results
	StdIn               bool           // pull input from std-in, rather than input files
	Timeout             time.Duration  // timeout to prevent infinite hangs. If set to 0, the command will never time out
	V1Compatible        bool           // use OPA 1.0 compatibility mode
	Logger              logging.Logger // Logger override. If set to nil, the default logger is used.
}

func NewParams(w io.Writer) *Params {
	return &Params{
		Output:       w,
		OutputFormat: util.NewEnumFlag("pretty", []string{"pretty", "json"}),
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
