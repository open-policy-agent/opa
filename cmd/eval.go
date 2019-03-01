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
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/internal/runtime"
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
	coverage          bool
	partial           bool
	unknowns          []string
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
	fail              bool
	failDefined       bool
}

func newEvalCommandParams() evalCommandParams {
	return evalCommandParams{
		outputFormat: util.NewEnumFlag(evalJSONOutput, []string{
			evalJSONOutput,
			evalValuesOutput,
			evalBindingsOutput,
			evalPrettyOutput,
		}),
		explain: util.NewEnumFlag(explainModeOff, []string{explainModeFull}),
	}
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
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {

			defined, err := eval(args, params, os.Stdout)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
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
	evalCommand.Flags().BoolVarP(&params.fail, "fail", "", false, "exits with non-zero exit code on undefined/empty result and errors")
	evalCommand.Flags().BoolVarP(&params.failDefined, "fail-defined", "", false, "exits with non-zero exit code on defined/non-empty result and errors")
	setIgnore(evalCommand.Flags(), &params.ignore)

	RootCommand.AddCommand(evalCommand)
}

func eval(args []string, params evalCommandParams, w io.Writer) (bool, error) {

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

	parsedModules := map[string]*ast.Module{}

	if len(params.dataPaths.v) > 0 {

		f := loaderFilter{
			Ignore: checkParams.ignore,
		}

		loadResult, err := loader.Filtered(params.dataPaths.v, f.Apply)
		if err != nil {
			return false, err
		}
		regoArgs = append(regoArgs, rego.Store(inmem.NewFromObject(loadResult.Documents)))
		for _, file := range loadResult.Modules {
			parsedModules[file.Name] = file.Parsed
			regoArgs = append(regoArgs, rego.Module(file.Name, string(file.Raw)))
		}
	}

	bs, err := readInputBytes(params)
	if err != nil {
		return false, err
	} else if bs != nil {
		term, err := ast.ParseTerm(string(bs))
		if err != nil {
			return false, err
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
	if params.profile {
		p = profiler.New()
		regoArgs = append(regoArgs, rego.Tracer(p))
	}

	if params.partial {
		regoArgs = append(regoArgs, rego.Unknowns(params.unknowns))
	}

	var c *cover.Cover

	if params.coverage {
		c = cover.New()
		regoArgs = append(regoArgs, rego.Tracer(c))
	}

	eval := rego.New(regoArgs...)
	ctx := context.Background()

	var result pr.Output

	if !params.partial {
		result.Result, result.Error = eval.Eval(ctx)
	} else {
		result.Partial, result.Error = eval.Partial(ctx)
	}

	if tracer != nil {
		result.Explanation = *tracer
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
		return false, result.Error
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
