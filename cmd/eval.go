// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/profiler"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type evalCommandParams struct {
	dataPaths         repeatedStringFlag
	inputPath         string
	imports           repeatedStringFlag
	pkg               string
	stdin             bool
	stdinInput        bool
	explain           *util.EnumFlag
	metrics           bool
	ignore            []string
	outputFormat      *util.EnumFlag
	profile           bool
	profileTopResults bool
	profileCriteria   repeatedStringFlag
	profileLimit      intFlag
	prettyLimit       intFlag
}

const (
	explainModeOff     = ""
	explainModeFull    = "full"
	evalJSONOutput     = "json"
	evalValuesOutput   = "values"
	evalBindingsOutput = "bindings"
	evalPrettyOutput   = "pretty"

	// number of profile results to return by default
	defaultProfileLimit = 10

	defaultPrettyLimit = 80
)

// default sorting order for profiler results
var defaultSortOrder = []string{"total_time_ns", "num_eval", "num_redo", "file", "line"}

func init() {

	var params evalCommandParams

	params.outputFormat = util.NewEnumFlag(evalJSONOutput, []string{
		evalJSONOutput,
		evalValuesOutput,
		evalBindingsOutput,
		evalPrettyOutput,
	})
	params.explain = util.NewEnumFlag(explainModeOff, []string{explainModeFull})

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

File Loading
------------

The --data flag will recursively load data files and Rego files contained in
sub-directories under the path. For example, given /some/path:

	$ opa eval --data /some/path 'data'

Where /some/path contains:

	foo/
	  |
	  +-- bar/
	  |     |
	  |     +-- data.json
	  |
	  +-- baz.rego

The JSON file 'foo/bar/data.json' would be loaded and rooted under
'data.foo.bar' and the 'foo/baz.rego' would be loaded and rooted under the
package path contained inside the file.

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
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := eval(args, params); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		},
	}

	evalCommand.Flags().VarP(&params.dataPaths, "data", "d", "set data file(s) or directory path(s)")
	evalCommand.Flags().StringVarP(&params.inputPath, "input", "i", "", "set input file path")
	evalCommand.Flags().VarP(&params.imports, "import", "", "set query import(s)")
	evalCommand.Flags().StringVarP(&params.pkg, "package", "", "", "set query package")
	evalCommand.Flags().BoolVarP(&params.stdin, "stdin", "", false, "read query from stdin")
	evalCommand.Flags().BoolVarP(&params.stdinInput, "stdin-input", "I", false, "read input document from stdin")
	evalCommand.Flags().BoolVarP(&params.metrics, "metrics", "", false, "report query performance metrics")
	evalCommand.Flags().VarP(params.explain, "explain", "", "enable query explainations")
	evalCommand.Flags().VarP(params.outputFormat, "format", "f", "set output format")
	evalCommand.Flags().BoolVarP(&params.profile, "profile", "", false, "perform expression profiling")
	evalCommand.Flags().VarP(&params.profileCriteria, "profile-sort", "", "set sort order of expression profiler results")
	evalCommand.Flags().VarP(&params.profileLimit, "profile-limit", "", "set number of profiling results to show")
	evalCommand.Flags().VarP(&params.prettyLimit, "pretty-limit", "", "set limit after which pretty output gets truncated")
	setIgnore(evalCommand.Flags(), &params.ignore)

	RootCommand.AddCommand(evalCommand)
}

func eval(args []string, params evalCommandParams) (err error) {

	var query string

	if params.stdin {
		bs, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		query = string(bs)
	} else {
		query = args[0]
	}

	regoArgs := []func(*rego.Rego){rego.Query(query)}

	if len(params.imports.v) > 0 {
		regoArgs = append(regoArgs, rego.Imports(params.imports.v))
	}

	if params.pkg != "" {
		regoArgs = append(regoArgs, rego.Package(params.pkg))
	}

	// include metrics as part of the profiler result
	if isProfilingEnabled(params) {
		params.metrics = true
	}

	if len(params.dataPaths.v) > 0 {

		f := loaderFilter{
			Ignore: checkParams.ignore,
		}

		loadResult, err := loader.Filtered(params.dataPaths.v, f.Apply)
		if err != nil {
			return err
		}
		regoArgs = append(regoArgs, rego.Store(inmem.NewFromObject(loadResult.Documents)))
		for _, file := range loadResult.Modules {
			regoArgs = append(regoArgs, rego.Module(file.Name, string(file.Raw)))
		}
	}

	bs, err := readInputBytes(params)
	if err != nil {
		return err
	} else if bs != nil {
		term, err := ast.ParseTerm(string(bs))
		if err != nil {
			return err
		}
		regoArgs = append(regoArgs, rego.ParsedInput(term.Value))
	}

	var tracer *topdown.BufferTracer

	switch params.explain.String() {
	case explainModeFull:
		tracer = topdown.NewBufferTracer()
		regoArgs = append(regoArgs, rego.Tracer(tracer))
	}

	var m metrics.Metrics

	if params.metrics {
		m = metrics.New()
		regoArgs = append(regoArgs, rego.Metrics(m))
	}

	var p *profiler.Profiler
	if isProfilingEnabled(params) {
		p = profiler.New()
		regoArgs = append(regoArgs, rego.Tracer(p))
	}

	eval := rego.New(regoArgs...)
	ctx := context.Background()

	rs, err := eval.Eval(ctx)
	if err != nil {
		return err
	}

	result := pr.EvalResult{
		Result: rs,
	}

	if params.explain.String() != explainModeOff {
		var traceBuffer bytes.Buffer
		topdown.PrettyTrace(&traceBuffer, *tracer)
		result.Explanation = strings.Split(traceBuffer.String(), "\n")
	}

	if params.metrics {
		result.Metrics = m.All()
	}

	if isProfilingEnabled(params) {
		var sortOrder = defaultSortOrder

		if len(params.profileCriteria.v) != 0 {
			sortOrder = getProfileSortOrder(strings.Split(params.profileCriteria.String(), ","))
		}

		result.Profile = p.ReportTopNResults(params.profileLimit.v, sortOrder)
	}

	switch params.outputFormat.String() {
	case evalBindingsOutput:
		for _, rs := range result.Result {
			err := pr.PrintJSON(os.Stdout, rs.Bindings)
			if err != nil {
				return err
			}
		}
	case evalValuesOutput:
		for _, rs := range result.Result {
			line := make([]interface{}, len(rs.Expressions))
			for i := range line {
				line[i] = rs.Expressions[i].Value
			}
			err := pr.PrintJSON(os.Stdout, line)
			if err != nil {
				return err
			}
		}
	case evalPrettyOutput:
		err := generateResultTable(result, params)
		if err != nil {
			return err
		}
	default:
		err := pr.PrintJSON(os.Stdout, result)
		if err != nil {
			return err
		}
	}
	return nil
}

func getProfileSortOrder(sortOrder []string) []string {

	// convert the sort order slice to a map for faster lookups
	sortOrderMap := make(map[string]bool)
	for _, cr := range sortOrder {
		sortOrderMap[cr] = true
	}

	// compare the given sort order and the default
	for _, cr := range defaultSortOrder {
		if _, ok := sortOrderMap[cr]; !ok {
			sortOrder = append(sortOrder, cr)
		}
	}
	return sortOrder
}

func isProfilingEnabled(params evalCommandParams) bool {
	if params.profile || params.profileCriteria.isFlagSet() || params.profileLimit.isFlagSet() {
		return true
	}
	return false
}

func generateResultTable(result pr.EvalResult, params evalCommandParams) error {

	if len(result.Result) == 1 {
		if len(result.Result[0].Bindings) == 0 && len(result.Result[0].Expressions) == 1 {
			err := pr.PrintJSON(os.Stdout, result.Result[0].Expressions[0].Value)
			if err != nil {
				return err
			}

			if isProfilingEnabled(params) {
				pr.PrintPrettyProfile(os.Stdout, result)
				pr.PrintPrettyMetrics(os.Stdout, result, params.prettyLimit.v)
			}
			return nil
		}
	}
	pr.PrintPretty(os.Stdout, result, params.prettyLimit.v)
	return nil
}

func readInputBytes(params evalCommandParams) ([]byte, error) {
	if params.stdinInput {
		return ioutil.ReadAll(os.Stdin)
	} else if params.inputPath != "" {
		return ioutil.ReadFile(params.inputPath)
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
