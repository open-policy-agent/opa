package exec

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/sdk"
)

var (
	r       *jsonReporter
	parsers = map[string]parser{
		".json": utilParser{},
		".yaml": utilParser{},
		".yml":  utilParser{},
	}
)

const stdInPath = "--stdin-input"

// Exec executes OPA against the supplied files and outputs each result.
//
// NOTE(tsandall): consider expanding functionality:
//
//   - specialized output formats (e.g., pretty/non-JSON outputs)
//   - exit codes set by convention or policy (e.g,. non-empty set => error)
//   - support for new input file formats beyond JSON and YAML
func Exec(ctx context.Context, opa *sdk.OPA, params *Params) error {
	if err := params.validateParams(); err != nil {
		return err
	}

	r = &jsonReporter{w: params.Output, buf: make([]result, 0), ctx: &ctx, opa: opa, params: params, decisionFunc: opa.Decision}

	if params.StdIn {
		if err := execOnStdIn(); err != nil {
			return err
		}
	}

	if err := execOnInputFiles(params); err != nil {
		return err
	}

	if err := r.Close(); err != nil {
		return err
	}

	return r.ReportFailure()
}

func execOnStdIn() error {
	sr := stdInReader{Reader: os.Stdin}
	p := utilParser{}
	raw := sr.ReadInput()
	input, err := p.Parse(strings.NewReader(raw))
	if err != nil {
		return err
	} else if input == nil {
		return errors.New("cannot execute on empty input; please enter valid json or yaml when using the --stdin-input flag")
	}
	r.StoreDecision(&input, stdInPath)
	return nil
}

type fileListItem struct {
	Path  string
	Error error
}

func execOnInputFiles(params *Params) error {
	for item := range listAllPaths(params.Paths) {

		if item.Error != nil {
			return item.Error
		}

		input, err := parse(item.Path)

		if err != nil {
			r.Report(result{Path: item.Path, Error: err})
			if params.FailDefined || params.Fail || params.FailNonEmpty {
				r.errorCount++
			}
			continue
		} else if input == nil {
			continue
		}
		r.StoreDecision(input, item.Path)
	}
	return nil
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

func parse(p string) (*interface{}, error) {
	selectedParser, ok := parsers[path.Ext(p)]
	if !ok {
		return nil, nil
	}

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	val, err := selectedParser.Parse(f)
	if err != nil {
		return nil, err
	}

	return &val, nil
}
