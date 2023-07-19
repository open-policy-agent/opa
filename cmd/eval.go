// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/ast/location"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/compile"
	"github.com/open-policy-agent/opa/cover"
	fileurl "github.com/open-policy-agent/opa/internal/file/url"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/internal/runtime"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/profiler"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/lineage"
	"github.com/open-policy-agent/opa/util"
)

type evalCommandParams struct {
	capabilities        *capabilitiesFlag
	coverage            bool
	partial             bool
	unknowns            []string
	disableInlining     []string
	shallowInlining     bool
	disableIndexing     bool
	disableEarlyExit    bool
	strictBuiltinErrors bool
	showBuiltinErrors   bool
	dataPaths           repeatedStringFlag
	inputPath           string
	imports             repeatedStringFlag
	pkg                 string
	stdin               bool
	stdinInput          bool
	explain             *util.EnumFlag
	metrics             bool
	instrument          bool
	ignore              []string
	outputFormat        *util.EnumFlag
	profile             bool
	profileCriteria     repeatedStringFlag
	profileLimit        intFlag
	count               int
	prettyLimit         intFlag
	fail                bool
	failDefined         bool
	bundlePaths         repeatedStringFlag
	schema              *schemaFlags
	target              *util.EnumFlag
	timeout             time.Duration
	optimizationLevel   int
	entrypoints         repeatedStringFlag
	strict              bool
}

func newEvalCommandParams() evalCommandParams {
	return evalCommandParams{
		capabilities: newcapabilitiesFlag(),
		outputFormat: util.NewEnumFlag(evalJSONOutput, []string{
			evalJSONOutput,
			evalValuesOutput,
			evalBindingsOutput,
			evalPrettyOutput,
			evalSourceOutput,
			evalRawOutput,
			evalDiscardOutput,
		}),
		explain:         newExplainFlag([]string{explainModeOff, explainModeFull, explainModeNotes, explainModeFails, explainModeDebug}),
		target:          util.NewEnumFlag(compile.TargetRego, []string{compile.TargetRego, compile.TargetWasm}),
		count:           1,
		profileCriteria: newrepeatedStringFlag([]string{}),
		profileLimit:    newIntFlag(defaultProfileLimit),
		prettyLimit:     newIntFlag(defaultPrettyLimit),
		schema:          &schemaFlags{},
	}
}

func validateEvalParams(p *evalCommandParams, cmdArgs []string) error {
	if len(cmdArgs) > 0 && p.stdin {
		return errors.New("specify query argument or --stdin but not both")
	} else if len(cmdArgs) == 0 && !p.stdin {
		return errors.New("specify query argument or --stdin")
	} else if len(cmdArgs) > 1 {
		return errors.New("specify at most one query argument")
	}
	if p.stdin && p.stdinInput {
		return errors.New("specify --stdin or --stdin-input but not both")
	}
	if p.stdinInput && p.inputPath != "" {
		return errors.New("specify --stdin-input or --input but not both")
	}
	if p.fail && p.failDefined {
		return errors.New("specify --fail or --fail-defined but not both")
	}
	of := p.outputFormat.String()
	if p.partial && of != evalPrettyOutput && of != evalJSONOutput && of != evalSourceOutput {
		return errors.New("invalid output format for partial evaluation")
	} else if !p.partial && of == evalSourceOutput {
		return errors.New("invalid output format for evaluation")
	}

	if p.optimizationLevel > 0 {
		if len(p.dataPaths.v) > 0 && p.bundlePaths.isFlagSet() {
			return fmt.Errorf("specify either --data or --bundle flag with optimization level greater than 0")
		}
	}

	if p.profileLimit.isFlagSet() || p.profileCriteria.isFlagSet() {
		p.profile = true
	}
	if p.profile {
		p.metrics = true
	}
	if p.instrument {
		p.metrics = true
	}
	return nil
}

const (
	evalJSONOutput     = "json"
	evalValuesOutput   = "values"
	evalBindingsOutput = "bindings"
	evalPrettyOutput   = "pretty"
	evalSourceOutput   = "source"
	evalRawOutput      = "raw"
	evalDiscardOutput  = "discard"

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

	evalCommand := &cobra.Command{
		Use:   "eval <query>",
		Short: "Evaluate a Rego query",
		Long: `Evaluate a Rego query and print the result.

Examples
--------

To evaluate a simple query:

    $ opa eval 'x := 1; y := 2; x < y'

To evaluate a query against JSON data:

    $ opa eval --data data.json 'name := data.names[_]'

To evaluate a query against JSON data supplied with a file:// URL:

    $ opa eval --data file:///path/to/file.json 'data'


File & Bundle Loading
---------------------

The --bundle flag will load data files and Rego files contained
in the bundle specified by the path. It can be either a
compressed tar archive bundle file or a directory tree.

    $ opa eval --bundle /some/path 'data'

Where /some/path contains:

    foo/
      |
      +-- bar/
      |     |
      |     +-- data.json
      |
      +-- baz.rego
      |
      +-- manifest.yaml

The JSON file 'foo/bar/data.json' would be loaded and rooted under
'data.foo.bar' and the 'foo/baz.rego' would be loaded and rooted under the
package path contained inside the file. Only data files named data.json or
data.yaml will be loaded. In the example above the manifest.yaml would be
ignored.

See https://www.openpolicyagent.org/docs/latest/management-bundles/ for more details
on bundle directory structures.

The --data flag can be used to recursively load ALL *.rego, *.json, and
*.yaml files under the specified directory.

The -O flag controls the optimization level. By default, optimization is disabled (-O=0).
When optimization is enabled the 'eval' command generates a bundle from the files provided
with either the --bundle or --data flag. This bundle is semantically equivalent to the input
files however the structure of the files in the bundle may have been changed by rewriting, inlining,
pruning, etc. This resulting optimized bundle is used to evaluate the query. If optimization is
enabled at least one entrypoint must be supplied, either via the -e option, or via entrypoint
metadata annotations.

Output Formats
--------------

Set the output format with the --format flag.

    --format=json      : output raw query results as JSON
    --format=values    : output line separated JSON arrays containing expression values
    --format=bindings  : output line separated JSON objects containing variable bindings
    --format=pretty    : output query results in a human-readable format
    --format=source    : output partial evaluation results in a source format
    --format=raw       : output the values from query results in a scripting friendly format
    --format=discard   : output the result field as "discarded" when non-nil

Schema
------

The -s/--schema flag provides one or more JSON Schemas used to validate references to the input or data documents.
Loads a single JSON file, applying it to the input document; or all the schema files under the specified directory.

    $ opa eval --data policy.rego --input input.json --schema schema.json
    $ opa eval --data policy.rego --input input.json --schema schemas/

Capabilities
------------

When passing a capabilities definition file via --capabilities, one can restrict which
hosts remote schema definitions can be retrieved from. For example, a capabilities.json
containing

    {
        "builtins": [ ... ],
        "allow_net": [ "kubernetesjsonschema.dev" ]
    }

would disallow fetching remote schemas from any host but "kubernetesjsonschema.dev".
Setting allow_net to an empty array would prohibit fetching any remote schemas.

Not providing a capabilities file, or providing a file without an allow_net key, will
permit fetching remote schemas from any host.

Note that the metaschemas http://json-schema.org/draft-04/schema, http://json-schema.org/draft-06/schema,
and http://json-schema.org/draft-07/schema, are always available, even without network
access.
`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateEvalParams(&params, args)
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

	// Eval specific flags
	evalCommand.Flags().BoolVarP(&params.coverage, "coverage", "", false, "report coverage")
	evalCommand.Flags().StringArrayVarP(&params.disableInlining, "disable-inlining", "", []string{}, "set paths of documents to exclude from inlining")
	evalCommand.Flags().BoolVarP(&params.shallowInlining, "shallow-inlining", "", false, "disable inlining of rules that depend on unknowns")
	evalCommand.Flags().BoolVar(&params.disableIndexing, "disable-indexing", false, "disable indexing optimizations")
	evalCommand.Flags().BoolVar(&params.disableEarlyExit, "disable-early-exit", false, "disable 'early exit' optimizations")
	evalCommand.Flags().BoolVarP(&params.strictBuiltinErrors, "strict-builtin-errors", "", false, "treat the first built-in function error encountered as fatal")
	evalCommand.Flags().BoolVarP(&params.showBuiltinErrors, "show-builtin-errors", "", false, "collect and return all encountered built-in errors, built in errors are not fatal")
	evalCommand.Flags().BoolVarP(&params.instrument, "instrument", "", false, "enable query instrumentation metrics (implies --metrics)")
	evalCommand.Flags().BoolVarP(&params.profile, "profile", "", false, "perform expression profiling")
	evalCommand.Flags().VarP(&params.profileCriteria, "profile-sort", "", "set sort order of expression profiler results. Accepts: total_time_ns, num_eval, num_redo, num_gen_expr, file, line. This flag can be repeated.")
	evalCommand.Flags().VarP(&params.profileLimit, "profile-limit", "", "set number of profiling results to show")
	evalCommand.Flags().VarP(&params.prettyLimit, "pretty-limit", "", "set limit after which pretty output gets truncated")
	evalCommand.Flags().BoolVarP(&params.failDefined, "fail-defined", "", false, "exits with non-zero exit code on defined/non-empty result and errors")
	evalCommand.Flags().DurationVar(&params.timeout, "timeout", 0, "set eval timeout (default unlimited)")

	evalCommand.Flags().IntVarP(&params.optimizationLevel, "optimize", "O", 0, "set optimization level")
	evalCommand.Flags().VarP(&params.entrypoints, "entrypoint", "e", "set slash separated entrypoint path")

	// Shared flags
	addCapabilitiesFlag(evalCommand.Flags(), params.capabilities)
	addPartialFlag(evalCommand.Flags(), &params.partial, false)
	addUnknownsFlag(evalCommand.Flags(), &params.unknowns, []string{"input"})
	addFailFlag(evalCommand.Flags(), &params.fail, false)
	addDataFlag(evalCommand.Flags(), &params.dataPaths)
	addBundleFlag(evalCommand.Flags(), &params.bundlePaths)
	addInputFlag(evalCommand.Flags(), &params.inputPath)
	addImportFlag(evalCommand.Flags(), &params.imports)
	addPackageFlag(evalCommand.Flags(), &params.pkg)
	addQueryStdinFlag(evalCommand.Flags(), &params.stdin)
	addInputStdinFlag(evalCommand.Flags(), &params.stdinInput)
	addMetricsFlag(evalCommand.Flags(), &params.metrics, false)
	addOutputFormat(evalCommand.Flags(), params.outputFormat)
	addIgnoreFlag(evalCommand.Flags(), &params.ignore)
	setExplainFlag(evalCommand.Flags(), params.explain)
	addSchemaFlags(evalCommand.Flags(), params.schema)
	addTargetFlag(evalCommand.Flags(), params.target)
	addCountFlag(evalCommand.Flags(), &params.count, "benchmark")
	addStrictFlag(evalCommand.Flags(), &params.strict, false)

	RootCommand.AddCommand(evalCommand)
}

func eval(args []string, params evalCommandParams, w io.Writer) (bool, error) {

	ctx := context.Background()
	if params.timeout != 0 {
		var cancel func()
		ctx, cancel = context.WithTimeout(ctx, params.timeout)
		defer cancel()
	}

	ectx, err := setupEval(args, params)
	if err != nil {
		return false, err
	}

	ectx.regoArgs = append(ectx.regoArgs,
		rego.EnablePrintStatements(true),
		rego.PrintHook(topdown.NewPrintHook(os.Stderr)))

	results := make([]pr.Output, ectx.params.count)
	profiles := make([][]profiler.ExprStats, ectx.params.count)
	timers := make([]map[string]interface{}, ectx.params.count)

	for i := 0; i < ectx.params.count; i++ {
		results[i] = evalOnce(ctx, ectx)
		profiles[i] = results[i].Profile
		if ts, ok := results[i].Metrics.(metrics.TimerMetrics); ok {
			timers[i] = ts.Timers()
		}
	}

	result := results[0]

	if ectx.params.count > 1 {
		result.Profile = nil
		result.Metrics = nil
		result.AggregatedProfile = profiler.AggregateProfiles(profiles...)
		timersAggregated := map[string]interface{}{}
		for name := range timers[0] {
			var vals []int64
			for _, t := range timers {
				val, ok := t[name].(int64)
				if !ok {
					return false, fmt.Errorf("missing timer for %s" + name)
				}
				vals = append(vals, val)
			}
			timersAggregated[name] = metrics.Statistics(vals...)
		}
		result.AggregatedMetrics = timersAggregated
	}

	var builtInErrorCount int
	if ectx.params.showBuiltinErrors {
		builtInErrorCount = len(*(ectx.builtInErrorList))
	}

	switch ectx.params.outputFormat.String() {
	case evalBindingsOutput:
		err = pr.Bindings(w, result)
	case evalValuesOutput:
		err = pr.Values(w, result)
	case evalPrettyOutput:
		err = pr.Pretty(w, result)
	case evalSourceOutput:
		err = pr.Source(w, result)
	case evalRawOutput:
		err = pr.Raw(w, result)
	case evalDiscardOutput:
		err = pr.Discard(w, result)
	default:
		err = pr.JSON(w, result)
	}

	if err != nil {
		return false, err
	} else if errorCount := len(result.Errors); errorCount > 0 && errorCount != builtInErrorCount {
		// if we only have built-in errors, we don't want to return an error. If
		// strict-builtin-errors is set the first built-in error will be returned
		// in a result error instead.

		// If the rego package returned an error, return a special error here so
		// that the command doesn't print the same error twice. The error will
		// have been printed above by the presentation package.
		return false, regoError{}
	} else if len(result.Result) == 0 {
		return false, nil
	}

	return true, nil
}

func evalOnce(ctx context.Context, ectx *evalContext) pr.Output {
	var result pr.Output
	var resultErr error
	var parsedModules map[string]*ast.Module

	if ectx.metrics != nil {
		ectx.metrics.Clear()
	}
	if ectx.profiler != nil {
		ectx.profiler.reset()
	}
	r := rego.New(ectx.regoArgs...)

	if !ectx.params.partial {
		var pq rego.PreparedEvalQuery
		pq, resultErr = r.PrepareForEval(ctx)
		if resultErr == nil {
			parsedModules = pq.Modules()
			result.Result, resultErr = pq.Eval(ctx, ectx.evalArgs...)
		}
	} else {
		var pq rego.PreparedPartialQuery
		pq, resultErr = r.PrepareForPartial(ctx)
		if resultErr == nil {
			parsedModules = pq.Modules()
			result.Partial, resultErr = pq.Partial(ctx, ectx.evalArgs...)
			resetExprLocations(result.Partial)
		}
	}

	result.Errors = pr.NewOutputErrors(resultErr)
	if ectx.builtInErrorList != nil {
		for _, err := range *(ectx.builtInErrorList) {
			err := err
			result.Errors = append(result.Errors, pr.NewOutputErrors(&err)...)
		}
	}

	if ectx.params.explain != nil {
		switch ectx.params.explain.String() {
		case explainModeDebug:
			result.Explanation = lineage.Debug(*(ectx.tracer))
		case explainModeFull:
			result.Explanation = lineage.Full(*(ectx.tracer))
		case explainModeNotes:
			result.Explanation = lineage.Notes(*(ectx.tracer))
		case explainModeFails:
			result.Explanation = lineage.Fails(*(ectx.tracer))
		}
	}

	if ectx.metrics != nil {
		result.Metrics = ectx.metrics
	}

	if ectx.params.profile {
		var sortOrder = pr.DefaultProfileSortOrder

		if len(ectx.params.profileCriteria.v) != 0 {
			sortOrder = getProfileSortOrder(strings.Split(ectx.params.profileCriteria.String(), ","))
		}

		result.Profile = ectx.profiler.p.ReportTopNResults(ectx.params.profileLimit.v, sortOrder)
	}

	if ectx.params.coverage {
		report := ectx.cover.Report(parsedModules)
		result.Coverage = &report
	}

	return result
}

type evalContext struct {
	params           evalCommandParams
	metrics          metrics.Metrics
	profiler         *resettableProfiler
	cover            *cover.Cover
	tracer           *topdown.BufferTracer
	regoArgs         []func(*rego.Rego)
	evalArgs         []rego.EvalOption
	builtInErrorList *[]topdown.Error
}

func setupEval(args []string, params evalCommandParams) (*evalContext, error) {
	var query string

	if params.stdin {
		bs, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		query = string(bs)
	} else {
		query = args[0]
	}

	info, err := runtime.Term(runtime.Params{})
	if err != nil {
		return nil, err
	}

	regoArgs := []func(*rego.Rego){rego.Query(query), rego.Runtime(info)}
	evalArgs := []rego.EvalOption{
		rego.EvalRuleIndexing(!params.disableIndexing),
		rego.EvalEarlyExit(!params.disableEarlyExit),
	}

	if len(params.imports.v) > 0 {
		regoArgs = append(regoArgs, rego.Imports(params.imports.v))
	}

	if params.pkg != "" {
		regoArgs = append(regoArgs, rego.Package(params.pkg))
	}

	if len(params.dataPaths.v) > 0 {
		f := loaderFilter{
			Ignore: params.ignore,
		}

		if params.optimizationLevel <= 0 {
			regoArgs = append(regoArgs, rego.Load(params.dataPaths.v, f.Apply))
		} else {
			b, err := generateOptimizedBundle(params, false, f.Apply, params.dataPaths.v)
			if err != nil {
				return nil, err
			}

			regoArgs = append(regoArgs, rego.ParsedBundle("optimized", b))
		}
	}

	if params.bundlePaths.isFlagSet() {
		if params.optimizationLevel <= 0 {
			for _, bundleDir := range params.bundlePaths.v {
				regoArgs = append(regoArgs, rego.LoadBundle(bundleDir))
			}
		} else {
			b, err := generateOptimizedBundle(params, true, buildCommandLoaderFilter(true, params.ignore), params.bundlePaths.v)
			if err != nil {
				return nil, err
			}

			regoArgs = append(regoArgs, rego.ParsedBundle("optimized", b))
		}
	}

	// skip bundle verification
	regoArgs = append(regoArgs, rego.SkipBundleVerification(true))

	regoArgs = append(regoArgs, rego.Target(params.target.String()))

	inputBytes, err := readInputBytes(params)
	if err != nil {
		return nil, err
	}
	if inputBytes != nil {
		var input interface{}
		err := util.Unmarshal(inputBytes, &input)
		if err != nil {
			return nil, fmt.Errorf("unable to parse input: %s", err.Error())
		}
		inputValue, err := ast.InterfaceToValue(input)
		if err != nil {
			return nil, fmt.Errorf("unable to process input: %s", err.Error())
		}
		regoArgs = append(regoArgs, rego.ParsedInput(inputValue))
	}

	//	-s {file} (one input schema file)
	//	-s {directory} (one schema directory with input and data schema files)
	schemaSet, err := loader.Schemas(params.schema.path)
	if err != nil {
		return nil, err
	}
	regoArgs = append(regoArgs, rego.Schemas(schemaSet))

	var tracer *topdown.BufferTracer

	if params.explain != nil && params.explain.String() != explainModeOff {
		tracer = topdown.NewBufferTracer()
		evalArgs = append(evalArgs, rego.EvalQueryTracer(tracer))

		if params.target.String() == compile.TargetWasm {
			fmt.Fprintf(os.Stderr, "warning: explain mode \"%v\" is not supported with wasm target\n", params.explain.String())
		}
	}

	var m metrics.Metrics
	if params.metrics {
		m = metrics.New()

		// Use the same metrics for preparing and evaluating
		regoArgs = append(regoArgs, rego.Metrics(m))
		evalArgs = append(evalArgs, rego.EvalMetrics(m))
	}

	if params.instrument {
		regoArgs = append(regoArgs, rego.Instrument(true))
		evalArgs = append(evalArgs, rego.EvalInstrument(true))
	}

	rp := resettableProfiler{}
	if params.profile {
		rp.p = profiler.New()
		evalArgs = append(evalArgs, rego.EvalQueryTracer(&rp))
	}

	if params.partial {
		regoArgs = append(regoArgs, rego.Unknowns(params.unknowns))
	}

	regoArgs = append(regoArgs, rego.DisableInlining(params.disableInlining), rego.ShallowInlining(params.shallowInlining))

	var c *cover.Cover

	if params.coverage {
		c = cover.New()
		evalArgs = append(evalArgs, rego.EvalQueryTracer(c))
	}

	if params.strictBuiltinErrors {
		regoArgs = append(regoArgs, rego.StrictBuiltinErrors(true))
		if params.showBuiltinErrors {
			return nil, fmt.Errorf("cannot use --show-builtin-errors with --strict-builtin-errors, --strict-builtin-errors will return the first built-in error encountered immediately")
		}
	}

	var builtInErrors []topdown.Error
	if params.showBuiltinErrors {
		regoArgs = append(regoArgs, rego.BuiltinErrorList(&builtInErrors))
	}

	if params.capabilities != nil {
		regoArgs = append(regoArgs, rego.Capabilities(params.capabilities.C))
	}

	if params.strict {
		regoArgs = append(regoArgs, rego.Strict(params.strict))
	}

	evalCtx := &evalContext{
		params:           params,
		metrics:          m,
		profiler:         &rp,
		cover:            c,
		tracer:           tracer,
		regoArgs:         regoArgs,
		evalArgs:         evalArgs,
		builtInErrorList: &builtInErrors,
	}

	return evalCtx, nil
}

type resettableProfiler struct {
	p *profiler.Profiler
}

func (r *resettableProfiler) reset() {
	r.p = profiler.New()
}

func (*resettableProfiler) Enabled() bool                 { return true }
func (r *resettableProfiler) TraceEvent(ev topdown.Event) { r.p.TraceEvent(ev) }
func (r *resettableProfiler) Config() topdown.TraceConfig { return r.p.Config() }

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
		return io.ReadAll(os.Stdin)
	} else if params.inputPath != "" {
		path, err := fileurl.Clean(params.inputPath)
		if err != nil {
			return nil, err
		}
		return os.ReadFile(path)
	}
	return nil, nil
}

const stringType = "string"

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
	return stringType
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
	v, err := strconv.ParseInt(s, 0, 32)
	f.v = int(v)
	f.isSet = true
	return err
}

func (f *intFlag) isFlagSet() bool {
	return f.isSet
}

// resetExprLocations overwrites the row in the location info for every expression contained in pq.
// The location on every expression is shallow copied to avoid mutating shared state. Overwriting
// the rows ensures that the formatting package does not leave blank lines in between expressions (e.g.,
// if expression 1 was saved on L10 and expression 2 was saved on L20 then the formatting package would
// squash the blank lines.)
func resetExprLocations(pq *rego.PartialQueries) {
	if pq == nil {
		return
	}

	vis := &astLocationResetVisitor{}

	for i := range pq.Queries {
		ast.NewGenericVisitor(vis.visit).Walk(pq.Queries[i])
	}

	for i := range pq.Support {
		ast.NewGenericVisitor(vis.visit).Walk(pq.Support[i])
	}
}

type astLocationResetVisitor struct {
	n int
}

func (vis *astLocationResetVisitor) visit(x interface{}) bool {
	if expr, ok := x.(*ast.Expr); ok {
		if expr.Location != nil {
			cpy := *expr.Location
			cpy.Row = vis.n
			expr.Location = &cpy
		} else {
			expr.Location = location.NewLocation(nil, "", vis.n, 1)
		}
		vis.n++
	}
	return false
}

func generateOptimizedBundle(params evalCommandParams, asBundle bool, filter loader.Filter, paths []string) (*bundle.Bundle, error) {
	buf := bytes.NewBuffer(nil)

	var capabilities *ast.Capabilities
	if params.capabilities.C != nil {
		capabilities = params.capabilities.C
	} else {
		capabilities = ast.CapabilitiesForThisVersion()
	}

	compiler := compile.New().
		WithCapabilities(capabilities).
		WithTarget(params.target.String()).
		WithAsBundle(asBundle).
		WithOptimizationLevel(params.optimizationLevel).
		WithOutput(buf).
		WithEntrypoints(params.entrypoints.v...).
		WithRegoAnnotationEntrypoints(true).
		WithPaths(paths...).
		WithFilter(filter)

	err := compiler.Build(context.Background())
	if err != nil {
		return nil, err
	}

	return compiler.Bundle(), nil
}
