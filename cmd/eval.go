// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/cover"
	fileurl "github.com/open-policy-agent/opa/internal/file/url"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/internal/runtime"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/profiler"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/lineage"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type evalCommandParams struct {
	coverage          bool
	partial           bool
	unknowns          []string
	disableInlining   []string
	dataPaths         repeatedStringFlag
	inputPath         string
	imports           repeatedStringFlag
	pkg               string
	stdin             bool
	stdinInput        bool
	explain           *util.EnumFlag
	metrics           bool
	instrument        bool
	ignore            []string
	outputFormat      *util.EnumFlag
	profile           bool
	profileTopResults bool
	profileCriteria   repeatedStringFlag
	profileLimit      intFlag
	prettyLimit       intFlag
	fail              bool
	failDefined       bool
	bundlePaths       repeatedStringFlag
}

func newEvalCommandParams() evalCommandParams {
	return evalCommandParams{
		outputFormat: util.NewEnumFlag(evalJSONOutput, []string{
			evalJSONOutput,
			evalValuesOutput,
			evalBindingsOutput,
			evalPrettyOutput,
		}),
		explain: newExplainFlag([]string{explainModeOff, explainModeFull, explainModeNotes, explainModeFails}),
	}
}

const (
	evalJSONOutput     = "json"
	evalValuesOutput   = "values"
	evalBindingsOutput = "bindings"
	evalPrettyOutput   = "pretty"

	// number of profile results to return by default
	defaultProfileLimit = 10

	defaultPrettyLimit = 80
)

type regoError struct{}

func (regoError) Error() string {
	return "rego"
}

func init() {

	params := newEvalCommandParams()
	params.profileCriteria = newrepeatedStringFlag([]string{})
	params.profileLimit = newIntFlag(defaultProfileLimit)
	params.prettyLimit = newIntFlag(defaultPrettyLimit)

	evalCommand := &cobra.Command{
		Use:   "eval <query>",
		Short: "Evaluate a Rego query",
		Long: `Evaluate a Rego query and print the result.

Examples
--------

To evaluate a simple query:

	$ opa eval 'x = 1; y = 2; x < y'

To evaluate a query against JSON data:

	$ opa eval --data data.json 'data.names[_] = name'

To evaluate a query against JSON data supplied with a file:// URL:

	$ opa eval --data file:///path/to/file.json 'data'


File & Bundle Loading
---------------------

The --bundle flag will load data files and Rego files contained
the bundle specified by the path. It can be either a compressed
tar archive bundle file or a directory tree.

	$ opa eval --bundle /some/path 'data'

Where /some/path contains:

	foo/
	  |
	  +-- bar/
	  |     |
	  |     +-- data.json
	  |
	  +-- baz_test.rego
	  |
	  +-- manifest.yaml

The JSON file 'foo/bar/data.json' would be loaded and rooted under
'data.foo.bar' and the 'foo/baz.rego' would be loaded and rooted under the
package path contained inside the file. Only data files named data.json or
data.yaml will be loaded. In the example above the manifest.yaml would be
ignored.

See https://www.openpolicyagent.org/docs/latest/bundles/ for more details
on bundle directory structures.

The --data flag can be used to recursively load ALL *.rego, *.json, and
*.yaml files under the specified directory.

Output Formats
--------------

Set the output format with the --format flag.

	--format=json      : output raw query results as JSON
	--format=values    : output line separated JSON arrays containing expression values
	--format=bindings  : output line separated JSON objects containing variable bindings
	--format=pretty    : output query results in a human-readable format
`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && params.stdin {
				return errors.New("specify query argument or --stdin but not both")
			} else if len(args) == 0 && !params.stdin {
				return errors.New("specify query argument or --stdin")
			} else if len(args) > 1 {
				return errors.New("specify at most one query argument")
			}
			if params.stdin && params.stdinInput {
				return errors.New("specify --stdin or --stdin-input but not both")
			}
			if params.stdinInput && params.inputPath != "" {
				return errors.New("specify --stdin-input or --input but not both")
			}
			if params.fail && params.failDefined {
				return errors.New("specify --fail or --fail-defined but not both")
			}
			of := params.outputFormat.String()
			if params.partial && of != evalPrettyOutput && of != evalJSONOutput {
				return errors.New("invalid output format for partial evaluation")
			}
			if params.profileLimit.isFlagSet() || params.profileCriteria.isFlagSet() {
				params.profile = true
			}
			if params.profile {
				params.metrics = true
			}
			if params.instrument {
				params.metrics = true
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {

			defined, err := eval(args, params, os.Stdout)
			if err != nil {
				if _, ok := err.(regoError); !ok {
					fmt.Fprintln(os.Stderr, err)
				}
				os.Exit(2)
			}

			if (params.fail && !defined) || (params.failDefined && defined) {
				os.Exit(1)
			}
		},
	}

	evalCommand.Flags().BoolVarP(&params.coverage, "coverage", "", false, "report coverage")
	evalCommand.Flags().BoolVarP(&params.partial, "partial", "p", false, "perform partial evaluation")
	evalCommand.Flags().StringSliceVarP(&params.unknowns, "unknowns", "u", []string{"input"}, "set paths to treat as unknown during partial evaluation")
	evalCommand.Flags().StringSliceVarP(&params.disableInlining, "disable-inlining", "", []string{}, "set paths of documents to exclude from inlining")
	evalCommand.Flags().VarP(&params.dataPaths, "data", "d", "set data file(s) or directory path(s)")
	evalCommand.Flags().VarP(&params.bundlePaths, "bundle", "b", "set bundle file(s) or directory path(s)")
	evalCommand.Flags().StringVarP(&params.inputPath, "input", "i", "", "set input file path")
	evalCommand.Flags().VarP(&params.imports, "import", "", "set query import(s)")
	evalCommand.Flags().StringVarP(&params.pkg, "package", "", "", "set query package")
	evalCommand.Flags().BoolVarP(&params.stdin, "stdin", "", false, "read query from stdin")
	evalCommand.Flags().BoolVarP(&params.stdinInput, "stdin-input", "I", false, "read input document from stdin")
	evalCommand.Flags().BoolVarP(&params.metrics, "metrics", "", false, "report query performance metrics")
	evalCommand.Flags().BoolVarP(&params.instrument, "instrument", "", false, "enable query instrumentation metrics (implies --metrics)")
	evalCommand.Flags().VarP(params.outputFormat, "format", "f", "set output format")
	evalCommand.Flags().BoolVarP(&params.profile, "profile", "", false, "perform expression profiling")
	evalCommand.Flags().VarP(&params.profileCriteria, "profile-sort", "", "set sort order of expression profiler results")
	evalCommand.Flags().VarP(&params.profileLimit, "profile-limit", "", "set number of profiling results to show")
	evalCommand.Flags().VarP(&params.prettyLimit, "pretty-limit", "", "set limit after which pretty output gets truncated")
	evalCommand.Flags().BoolVarP(&params.fail, "fail", "", false, "exits with non-zero exit code on undefined/empty result and errors")
	evalCommand.Flags().BoolVarP(&params.failDefined, "fail-defined", "", false, "exits with non-zero exit code on defined/non-empty result and errors")
	setIgnore(evalCommand.Flags(), &params.ignore)
	setExplain(evalCommand.Flags(), params.explain)
	RootCommand.AddCommand(evalCommand)
}

func eval(args []string, params evalCommandParams, w io.Writer) (bool, error) {

	ctx := context.Background()

	var query string

	if params.stdin {
		bs, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return false, err
		}
		query = string(bs)
	} else {
		query = args[0]
	}

	info, err := runtime.Term(runtime.Params{})
	if err != nil {
		return false, err
	}

	regoArgs := []func(*rego.Rego){rego.Query(query), rego.Runtime(info)}

	if len(params.imports.v) > 0 {
		regoArgs = append(regoArgs, rego.Imports(params.imports.v))
	}

	if params.pkg != "" {
		regoArgs = append(regoArgs, rego.Package(params.pkg))
	}

	if len(params.dataPaths.v) > 0 {
		f := loaderFilter{
			Ignore: checkParams.ignore,
		}
		regoArgs = append(regoArgs, rego.Load(params.dataPaths.v, f.Apply))
	}

	if params.bundlePaths.isFlagSet() {
		for _, bundleDir := range params.bundlePaths.v {
			regoArgs = append(regoArgs, rego.LoadBundle(bundleDir))
		}
	}

	inputBytes, err := readInputBytes(params)
	if err != nil {
		return false, err
	} else if inputBytes != nil {
		var input interface{}
		err := util.Unmarshal(inputBytes, &input)
		if err != nil {
			return false, fmt.Errorf("unable to parse input: %s", err.Error())
		}
		inputValue, err := ast.InterfaceToValue(input)
		if err != nil {
			return false, fmt.Errorf("unable to process input: %s", err.Error())
		}
		regoArgs = append(regoArgs, rego.ParsedInput(inputValue))
	}

	var tracer *topdown.BufferTracer

	if params.explain.String() != explainModeOff {
		tracer = topdown.NewBufferTracer()
		regoArgs = append(regoArgs, rego.Tracer(tracer))
	}

	var m metrics.Metrics

	if params.metrics {
		m = metrics.New()
		regoArgs = append(regoArgs, rego.Metrics(m))
	}

	if params.instrument {
		regoArgs = append(regoArgs, rego.Instrument(true))
	}

	var p *profiler.Profiler
	if params.profile {
		p = profiler.New()
		regoArgs = append(regoArgs, rego.Tracer(p))
	}

	if params.partial {
		regoArgs = append(regoArgs, rego.Unknowns(params.unknowns))
	}

	regoArgs = append(regoArgs, rego.DisableInlining(params.disableInlining))

	var c *cover.Cover

	if params.coverage {
		c = cover.New()
		regoArgs = append(regoArgs, rego.Tracer(c))
	}

	eval := rego.New(regoArgs...)

	var result pr.Output

	var parsedModules map[string]*ast.Module

	if !params.partial {
		var pq rego.PreparedEvalQuery
		pq, result.Error = eval.PrepareForEval(ctx)
		if result.Error == nil {
			parsedModules = pq.Modules()
			result.Result, result.Error = pq.Eval(ctx)
		}
	} else {
		var pq rego.PreparedPartialQuery
		pq, result.Error = eval.PrepareForPartial(ctx)
		if result.Error == nil {
			parsedModules = pq.Modules()
			result.Partial, result.Error = eval.Partial(ctx)
		}
	}

	switch params.explain.String() {
	case explainModeFull:
		result.Explanation = *tracer
	case explainModeNotes:
		result.Explanation = lineage.Notes(*tracer)
	case explainModeFails:
		result.Explanation = lineage.Fails(*tracer)
	}

	if m != nil {
		result.Metrics = m
	}

	if params.profile {
		var sortOrder = pr.DefaultProfileSortOrder

		if len(params.profileCriteria.v) != 0 {
			sortOrder = getProfileSortOrder(strings.Split(params.profileCriteria.String(), ","))
		}

		result.Profile = p.ReportTopNResults(params.profileLimit.v, sortOrder)
	}

	if params.coverage {
		report := c.Report(parsedModules)
		result.Coverage = &report
	}

	switch params.outputFormat.String() {
	case evalBindingsOutput:
		err = pr.Bindings(w, result)
	case evalValuesOutput:
		err = pr.Values(w, result)
	case evalPrettyOutput:
		err = pr.Pretty(w, result)
	default:
		err = pr.JSON(w, result)
	}

	if err != nil {
		return false, err
	} else if result.Error != nil {
		// If the rego package returned an error, return a special error here so
		// that the command doesn't print the same error twice. The error will
		// have been printed above by the presentation package.
		return false, regoError{}
	} else if len(result.Result) == 0 {
		return false, nil
	} else {
		return true, nil
	}
}

func getProfileSortOrder(sortOrder []string) []string {

	// convert the sort order slice to a map for faster lookups
	sortOrderMap := make(map[string]bool)
	for _, cr := range sortOrder {
		sortOrderMap[cr] = true
	}

	// compare the given sort order and the default
	for _, cr := range pr.DefaultProfileSortOrder {
		if _, ok := sortOrderMap[cr]; !ok {
			sortOrder = append(sortOrder, cr)
		}
	}
	return sortOrder
}

func readInputBytes(params evalCommandParams) ([]byte, error) {
	if params.stdinInput {
		return ioutil.ReadAll(os.Stdin)
	} else if params.inputPath != "" {
		path, err := fileurl.Clean(params.inputPath)
		if err != nil {
			return nil, err
		}
		return ioutil.ReadFile(path)
	}
	return nil, nil
}

type repeatedStringFlag struct {
	v     []string
	isSet bool
}

func newrepeatedStringFlag(val []string) repeatedStringFlag {
	return repeatedStringFlag{
		v:     val,
		isSet: false,
	}
}

func (f *repeatedStringFlag) Type() string {
	return "string"
}

func (f *repeatedStringFlag) String() string {
	return strings.Join(f.v, ",")
}

func (f *repeatedStringFlag) Set(s string) error {
	f.v = append(f.v, s)
	f.isSet = true
	return nil
}

func (f *repeatedStringFlag) isFlagSet() bool {
	return f.isSet
}

type intFlag struct {
	v     int
	isSet bool
}

func newIntFlag(val int) intFlag {
	return intFlag{
		v:     val,
		isSet: false,
	}
}

func (f *intFlag) Type() string {
	return "int"
}

func (f *intFlag) String() string {
	return strconv.Itoa(f.v)
}

func (f *intFlag) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	f.v = int(v)
	f.isSet = true
	return err
}

func (f *intFlag) isFlagSet() bool {
	return f.isSet
}
