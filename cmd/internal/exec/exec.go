package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/open-policy-agent/opa/sdk"
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
	Fail                bool           // exits with non-zero exit code on undefined/empty result and errors
	FailDefined         bool           // exits with non-zero exit code on defined/non-empty result and errors
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
	return nil
}

// Exec executes OPA against the supplied files and outputs each result.
//
// NOTE(tsandall): consider expanding functionality:
//
//   - specialized output formats (e.g., pretty/non-JSON outputs)
//   - exit codes set by convention or policy (e.g,. non-empty set => error)
//   - support for new input file formats beyond JSON and YAML
func Exec(ctx context.Context, opa *sdk.OPA, params *Params) error {

	err := params.validateParams()
	if err != nil {
		return err
	}

	now := time.Now()
	r := &jsonReporter{w: params.Output, buf: make([]result, 0)}

	failCount := 0
	errorCount := 0

	for item := range listAllPaths(params.Paths) {

		if item.Error != nil {
			return item.Error
		}

		input, err := parse(item.Path)

		if err != nil {
			if err2 := r.Report(result{Path: item.Path, Error: err}); err2 != nil {
				return err2
			}
			if params.FailDefined || params.Fail {
				errorCount++
			}
			continue
		} else if input == nil {
			continue
		}

		rs, err := opa.Decision(ctx, sdk.DecisionOptions{
			Path:  params.Decision,
			Now:   now,
			Input: input,
		})
		if err != nil {
			if err2 := r.Report(result{Path: item.Path, Error: err}); err2 != nil {
				return err2
			}
			if (params.FailDefined && !sdk.IsUndefinedErr(err)) || (params.Fail && sdk.IsUndefinedErr(err)) {
				errorCount++
			}
			continue
		}

		if err := r.Report(result{Path: item.Path, Result: &rs.Result}); err != nil {
			return err
		}

		if (params.FailDefined && rs.Result != nil) || (params.Fail && rs.Result == nil) {
			failCount++
		}
	}

	if err := r.Close(); err != nil {
		return err
	}

	if (params.Fail || params.FailDefined) && (failCount > 0 || errorCount > 0) {
		if params.Fail {
			return fmt.Errorf("there were %d failures and %d errors counted in the results list, and --fail is set", failCount, errorCount)
		}
		return fmt.Errorf("there were %d failures and %d errors counted in the results list, and --fail-defined is set", failCount, errorCount)
	}

	return nil
}

type result struct {
	Path   string       `json:"path"`
	Error  error        `json:"error,omitempty"`
	Result *interface{} `json:"result,omitempty"`
}

type jsonReporter struct {
	w   io.Writer
	buf []result
}

func (jr *jsonReporter) Report(r result) error {
	jr.buf = append(jr.buf, r)
	return nil
}

func (jr *jsonReporter) Close() error {
	enc := json.NewEncoder(jr.w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Result []result `json:"result"`
	}{
		Result: jr.buf,
	})
}

type fileListItem struct {
	Path  string
	Error error
}

func listAllPaths(roots []string) chan fileListItem {
	ch := make(chan fileListItem)
	go func() {
		for _, path := range roots {
			err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() {
					return nil
				}
				ch <- fileListItem{Path: path}
				return nil
			})
			if err != nil {
				ch <- fileListItem{Path: path, Error: err}
			}
		}
		close(ch)
	}()
	return ch
}

var parsers = map[string]parser{
	".json": utilParser{},
	".yaml": utilParser{},
	".yml":  utilParser{},
}

type parser interface {
	Parse(io.Reader) (interface{}, error)
}

type utilParser struct {
}

func (utilParser) Parse(r io.Reader) (interface{}, error) {
	bs, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var x interface{}
	return x, util.Unmarshal(bs, &x)
}

func parse(p string) (*interface{}, error) {

	parser, ok := parsers[path.Ext(p)]
	if !ok {
		return nil, nil
	}

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	val, err := parser.Parse(f)
	if err != nil {
		return nil, err
	}

	return &val, nil
}
