// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package repl implements a Read-Eval-Print-Loop (REPL) for interacting with the policy engine.
//
// The REPL is typically used from the command line, however, it can also be used as a library.
package repl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/rego"

	"github.com/olekukonko/tablewriter"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/format"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/version"
	"github.com/peterh/liner"
)

// REPL represents an instance of the interactive shell.
type REPL struct {
	output io.Writer
	store  storage.Store

	modules         map[string]*ast.Module
	currentModuleID string
	buffer          []string
	txn             storage.Transaction
	metrics         metrics.Metrics

	// TODO(tsandall): replace this state with rule definitions
	// inside the default module.
	outputFormat      string
	explain           explainMode
	instrument        bool
	historyPath       string
	initPrompt        string
	bufferPrompt      string
	banner            string
	types             bool
	unknowns          []*ast.Term
	bufferDisabled    bool
	undefinedDisabled bool
	errLimit          int
	prettyLimit       int
}

type explainMode int

const (
	explainOff   explainMode = iota
	explainTrace explainMode = iota
)

const defaultPrettyLimit = 80

const exitPromptMessage = "Do you want to exit ([y]/n)? "

// New returns a new instance of the REPL.
func New(store storage.Store, historyPath string, output io.Writer, outputFormat string, errLimit int, banner string) *REPL {

	module := defaultModule()
	moduleID := module.Package.Path.String()

	return &REPL{
		output: output,
		store:  store,
		modules: map[string]*ast.Module{
			moduleID: module,
		},
		currentModuleID: moduleID,
		outputFormat:    outputFormat,
		explain:         explainOff,
		historyPath:     historyPath,
		initPrompt:      "> ",
		bufferPrompt:    "| ",
		banner:          banner,
		errLimit:        errLimit,
		prettyLimit:     defaultPrettyLimit,
	}
}

const (
	defaultREPLModuleID = "repl"
)

func defaultModule() *ast.Module {

	module := `
	package {{.ModuleID}}

	version = {
		"Version": "{{.Version}}",
		"BuildCommit": "{{.BuildCommit}}",
		"BuildTimestamp": "{{.BuildTimestamp}}",
		"BuildHostname": "{{.BuildHostname}}"
	}
	`

	tmpl, err := template.New("").Parse(module)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, struct {
		ModuleID       string
		Version        string
		BuildCommit    string
		BuildTimestamp string
		BuildHostname  string
	}{
		ModuleID:       defaultREPLModuleID,
		Version:        version.Version,
		BuildCommit:    version.Vcs,
		BuildTimestamp: version.Timestamp,
		BuildHostname:  version.Hostname,
	})

	if err != nil {
		panic(err)
	}

	return ast.MustParseModule(buf.String())
}

// Loop will run until the user enters "exit", Ctrl+C, Ctrl+D, or an unexpected error occurs.
func (r *REPL) Loop(ctx context.Context) {

	// Initialize the liner library.
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)
	line.SetMultiLineMode(true)
	r.loadHistory(line)

	if len(r.banner) > 0 {
		fmt.Fprintln(r.output, r.banner)
	}

	line.SetCompleter(r.complete)

loop:
	for true {

		input, err := line.Prompt(r.getPrompt())

		// prompt on ctrl+d
		if err == io.EOF {
			goto exitPrompt
		}

		// reset on ctrl+c
		if err == liner.ErrPromptAborted {
			continue
		}

		// exit on unknown error
		if err != nil {
			fmt.Fprintln(r.output, "error (fatal):", err)
			os.Exit(1)
		}

		if err := r.OneShot(ctx, input); err != nil {
			switch err := err.(type) {
			case stop:
				goto exit
			default:
				fmt.Fprintln(r.output, err)
			}
		}

		line.AppendHistory(input)
	}

exitPrompt:
	fmt.Fprintln(r.output)

	for true {
		input, err := line.Prompt(exitPromptMessage)

		// exit on ctrl+d
		if err == io.EOF {
			break
		}

		// reset on ctrl+c
		if err == liner.ErrPromptAborted {
			goto loop
		}

		// exit on unknown error
		if err != nil {
			fmt.Fprintln(r.output, "error (fatal):", err)
			os.Exit(1)
		}

		switch strings.ToLower(input) {
		case "", "y", "yes":
			goto exit
		case "n", "no":
			goto loop
		}
	}

exit:
	r.saveHistory(line)
}

// OneShot evaluates the line and prints the result. If an error occurs it is
// returned for the caller to display.
func (r *REPL) OneShot(ctx context.Context, line string) error {

	var err error
	r.txn, err = r.store.NewTransaction(ctx)
	if err != nil {
		return err
	}

	defer r.store.Abort(ctx, r.txn)

	if r.metrics != nil {
		defer r.metrics.Clear()
	}

	if len(r.buffer) == 0 {
		if cmd := newCommand(line); cmd != nil {
			switch cmd.op {
			case "dump":
				return r.cmdDump(ctx, cmd.args)
			case "json":
				return r.cmdFormat("json")
			case "show":
				return r.cmdShow()
			case "unset":
				return r.cmdUnset(ctx, cmd.args)
			case "pretty":
				return r.cmdFormat("pretty")
			case "pretty-limit":
				return r.cmdPrettyLimit(cmd.args)
			case "trace":
				return r.cmdTrace()
			case "metrics":
				return r.cmdMetrics()
			case "instrument":
				return r.cmdInstrument()
			case "types":
				return r.cmdTypes()
			case "partial":
				return r.cmdPartial(cmd.args)
			case "help":
				return r.cmdHelp(cmd.args)
			case "exit":
				return r.cmdExit()
			}
		}

		r.buffer = append(r.buffer, line)
		return r.evalBufferOne(ctx)
	}

	r.buffer = append(r.buffer, line)
	if len(line) == 0 {
		return r.evalBufferMulti(ctx)
	}

	return nil
}

// DisableMultiLineBuffering causes the REPL to not buffer lines when a parse
// error occurs. Instead, the error will be returned to the caller.
func (r *REPL) DisableMultiLineBuffering(yes bool) *REPL {
	r.bufferDisabled = yes
	return r
}

// DisableUndefinedOutput causes the REPL to not print any output when the query
// is undefined.
func (r *REPL) DisableUndefinedOutput(yes bool) *REPL {
	r.undefinedDisabled = yes
	return r
}

func (r *REPL) complete(line string) []string {
	c := []string{}
	set := map[string]struct{}{}
	ctx := context.Background()
	txn, err := r.store.NewTransaction(ctx)

	if err != nil {
		fmt.Fprintln(r.output, "error:", err)
		return c
	}

	defer r.store.Abort(ctx, txn)

	// add imports
	for _, mod := range r.modules {
		for _, imp := range mod.Imports {
			path := imp.Name().String()
			if strings.HasPrefix(path, line) {
				set[path] = struct{}{}
			}
		}
	}

	// add virtual docs defined in repl
	for _, mod := range r.modules {
		for _, rule := range mod.Rules {
			path := rule.Path().String()
			if strings.HasPrefix(path, line) {
				set[path] = struct{}{}
			}
		}
	}

	mods, err := r.loadModules(ctx, txn)
	if err != nil {
		fmt.Fprintln(r.output, "error:", err)
		return c
	}

	// add virtual docs defined by policies
	for _, mod := range mods {
		for _, rule := range mod.Rules {
			path := rule.Path().String()
			if strings.HasPrefix(path, line) {
				set[path] = struct{}{}
			}
		}
	}

	for path := range set {
		c = append(c, path)
	}
	return c
}

func (r *REPL) cmdDump(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return r.cmdDumpOutput(ctx)
	}
	return r.cmdDumpPath(ctx, args[0])
}

func (r *REPL) cmdDumpOutput(ctx context.Context) error {
	return dumpStorage(ctx, r.store, r.txn, r.output)
}

func (r *REPL) cmdDumpPath(ctx context.Context, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return dumpStorage(ctx, r.store, r.txn, f)
}

func (r *REPL) cmdExit() error {
	return stop{}
}

func (r *REPL) cmdFormat(s string) error {
	r.outputFormat = s
	return nil
}

func (r *REPL) cmdPrettyLimit(s []string) error {
	if len(s) != 1 {
		return fmt.Errorf("usage: pretty-limit <n>")
	}
	i64, err := strconv.ParseInt(s[0], 10, 0)
	if err != nil {
		return err
	}
	r.prettyLimit = int(i64)
	return nil
}

func (r *REPL) cmdHelp(args []string) error {
	if len(args) == 0 {
		printHelp(r.output, r.initPrompt)
	} else {
		if desc, ok := topics[args[0]]; ok {
			return desc.fn(r.output)
		}
		return fmt.Errorf("unknown topic '%v'", args[0])
	}
	return nil
}

func (r *REPL) cmdShow() error {
	module := r.modules[r.currentModuleID]

	bs, err := format.Ast(module)
	if err != nil {
		return err
	}

	fmt.Fprint(r.output, string(bs))
	return nil
}

func (r *REPL) cmdTrace() error {
	if r.explain == explainTrace {
		r.explain = explainOff
	} else {
		r.explain = explainTrace
	}
	return nil
}

func (r *REPL) cmdMetrics() error {
	if r.metrics == nil {
		r.metrics = metrics.New()
	} else {
		r.metrics = nil
	}
	r.instrument = false
	return nil
}

func (r *REPL) cmdInstrument() error {
	if r.instrument {
		r.metrics = nil
		r.instrument = false
	} else {
		r.metrics = metrics.New()
		r.instrument = true
	}
	return nil
}

func (r *REPL) cmdTypes() error {
	r.types = !r.types
	return nil
}

func (r *REPL) cmdPartial(s []string) error {
	if len(s) == 0 && len(r.unknowns) == 0 {
		return fmt.Errorf("usage: partial <unknown-1> [<unknown-2> [...]] (hint: try just 'input')")
	}
	r.unknowns = make([]*ast.Term, len(s))
	for i := range r.unknowns {
		ref, err := ast.ParseRef(s[i])
		if err != nil {
			return err
		}
		r.unknowns[i] = ast.NewTerm(ref)
	}
	return nil
}

func (r *REPL) cmdUnset(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return newBadArgsErr("unset <var>: expects exactly one argument")
	}

	term, err := ast.ParseTerm(args[0])
	if err != nil {
		return newBadArgsErr("argument must identify a rule")
	}

	v, ok := term.Value.(ast.Var)

	if !ok {
		ref, ok := term.Value.(ast.Ref)
		if !ok || !ast.RootDocumentNames.Contains(ref[0]) {
			return newBadArgsErr("arguments must identify a rule")
		}
		v = ref[0].Value.(ast.Var)
	}

	unset, err := r.unsetRule(ctx, v)
	if err != nil {
		return err
	} else if !unset {
		fmt.Fprintln(r.output, "warning: no matching rules in current module")
	}

	return nil
}

func (r *REPL) unsetRule(ctx context.Context, name ast.Var) (bool, error) {

	mod := r.modules[r.currentModuleID]
	rules := []*ast.Rule{}

	for _, r := range mod.Rules {
		if !r.Head.Name.Equal(name) {
			rules = append(rules, r)
		}
	}

	if len(rules) == len(mod.Rules) {
		return false, nil
	}

	cpy := mod.Copy()
	cpy.Rules = rules
	err := r.recompile(ctx, cpy)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (r *REPL) timerStart(msg string) {
	if r.metrics != nil {
		r.metrics.Timer(msg).Start()
	}
}

func (r *REPL) timerStop(msg string) {
	if r.metrics != nil {
		r.metrics.Timer(msg).Stop()
	}
}

func (r *REPL) recompile(ctx context.Context, cpy *ast.Module) error {
	policies, err := r.loadModules(ctx, r.txn)
	if err != nil {
		return err
	}

	policies[r.currentModuleID] = cpy

	for id, mod := range r.modules {
		if id != r.currentModuleID {
			policies[id] = mod
		}
	}

	compiler := ast.NewCompiler().SetErrorLimit(r.errLimit)

	if compiler.Compile(policies); compiler.Failed() {
		return compiler.Errors
	}

	r.modules[r.currentModuleID] = cpy
	return nil
}

func (r *REPL) compileBody(ctx context.Context, body ast.Body, input ast.Value) (ast.Body, *ast.TypeEnv, error) {
	r.timerStart(metrics.RegoQueryCompile)
	defer r.timerStop(metrics.RegoQueryCompile)

	policies, err := r.loadModules(ctx, r.txn)
	if err != nil {
		return nil, nil, err
	}

	for id, mod := range r.modules {
		policies[id] = mod
	}

	compiler := ast.NewCompiler().SetErrorLimit(r.errLimit)

	if compiler.Compile(policies); compiler.Failed() {
		return nil, nil, compiler.Errors
	}

	qctx := ast.NewQueryContext().
		WithPackage(r.modules[r.currentModuleID].Package).
		WithImports(r.modules[r.currentModuleID].Imports).
		WithInput(input)

	qc := compiler.QueryCompiler()
	body, err = qc.WithContext(qctx).Compile(body)
	return body, qc.TypeEnv(), err
}

func (r *REPL) compileRule(ctx context.Context, rule *ast.Rule) error {
	r.timerStart(metrics.RegoQueryCompile)
	defer r.timerStop(metrics.RegoQueryCompile)

	mod := r.modules[r.currentModuleID]
	prev := mod.Rules
	mod.Rules = append(mod.Rules, rule)
	ast.WalkRules(rule, func(r *ast.Rule) bool {
		r.Module = mod
		return false
	})

	policies, err := r.loadModules(ctx, r.txn)
	if err != nil {
		return err
	}

	for id, mod := range r.modules {
		policies[id] = mod
	}

	compiler := ast.NewCompiler().SetErrorLimit(r.errLimit)

	if compiler.Compile(policies); compiler.Failed() {
		mod.Rules = prev
		return compiler.Errors
	}

	return nil
}

func (r *REPL) evalBufferOne(ctx context.Context) error {

	line := strings.Join(r.buffer, "\n")

	if len(strings.TrimSpace(line)) == 0 {
		r.buffer = []string{}
		return nil
	}

	// The user may enter lines with comments on the end or
	// multiple lines with comments interspersed. In these cases
	// the parser will return multiple statements.
	r.timerStart(metrics.RegoQueryParse)
	stmts, _, err := ast.ParseStatements("", line)
	r.timerStop(metrics.RegoQueryParse)

	if err != nil {
		if r.bufferDisabled {
			return err
		}
		return nil
	}

	r.buffer = []string{}

	for _, stmt := range stmts {
		if err := r.evalStatement(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

func (r *REPL) evalBufferMulti(ctx context.Context) error {

	line := strings.Join(r.buffer, "\n")
	r.buffer = []string{}

	if len(strings.TrimSpace(line)) == 0 {
		return nil
	}

	r.timerStart(metrics.RegoQueryParse)
	stmts, _, err := ast.ParseStatements("", line)
	r.timerStop(metrics.RegoQueryParse)

	if err != nil {
		return err
	}

	for _, stmt := range stmts {
		if err := r.evalStatement(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

func (r *REPL) loadCompiler(ctx context.Context) (*ast.Compiler, error) {

	policies, err := r.loadModules(ctx, r.txn)
	if err != nil {
		return nil, err
	}

	for id, mod := range r.modules {
		policies[id] = mod
	}

	compiler := ast.NewCompiler().SetErrorLimit(r.errLimit)

	if compiler.Compile(policies); compiler.Failed() {
		return nil, compiler.Errors
	}

	return compiler, nil
}

// loadInput returns the input defined in the REPL. The REPL loads the
// input from the data.repl.input document.
func (r *REPL) loadInput(ctx context.Context) (ast.Value, error) {

	compiler, err := r.loadCompiler(ctx)
	if err != nil {
		return nil, err
	}

	q := topdown.NewQuery(ast.MustParseBody("data.repl.input = x")).
		WithCompiler(compiler).
		WithStore(r.store).
		WithTransaction(r.txn)

	qrs, err := q.Run(ctx)
	if err != nil {
		return nil, err
	}

	if len(qrs) != 1 {
		return nil, nil
	}

	return qrs[0][ast.Var("x")].Value, nil
}

func (r *REPL) evalStatement(ctx context.Context, stmt interface{}) error {
	switch s := stmt.(type) {
	case ast.Body:
		input, err := r.loadInput(ctx)
		if err != nil {
			return err
		}

		parsedBody := s

		if len(parsedBody) == 1 && parsedBody[0].IsAssignment() {
			expr := parsedBody[0]
			rule, err := ast.ParseCompleteDocRuleFromEqExpr(r.modules[r.currentModuleID], expr.Operand(0), expr.Operand(1))
			if err == nil {
				if _, err := r.unsetRule(ctx, rule.Head.Name); err != nil {
					return err
				}
				return r.compileRule(ctx, rule)
			}
		}

		compiledBody, typeEnv, err := r.compileBody(ctx, parsedBody, input)
		if err != nil {
			return err
		}

		if len(compiledBody) == 1 && compiledBody[0].IsEquality() {
			expr := compiledBody[0]
			rule, err := ast.ParseCompleteDocRuleFromEqExpr(r.modules[r.currentModuleID], expr.Operand(0), expr.Operand(1))
			if err == nil {
				return r.compileRule(ctx, rule)
			}
		}

		compiler, err := r.loadCompiler(ctx)
		if err != nil {
			return err
		}

		if len(r.unknowns) > 0 {
			err = r.evalPartial(ctx, compiler, input, compiledBody)
		} else {
			err = r.evalBody(ctx, compiler, input, parsedBody)
			if r.types {
				r.printTypes(ctx, typeEnv, compiledBody)
			}
		}

		return err
	case *ast.Rule:
		return r.compileRule(ctx, s)
	case *ast.Import:
		return r.evalImport(s)
	case *ast.Package:
		return r.evalPackage(s)
	}
	return nil
}

func (r *REPL) evalBody(ctx context.Context, compiler *ast.Compiler, input ast.Value, body ast.Body) error {

	var buf *topdown.BufferTracer

	if r.explain != explainOff {
		buf = topdown.NewBufferTracer()
	}

	eval := rego.New(
		rego.Compiler(compiler),
		rego.Store(r.store),
		rego.Transaction(r.txn),
		rego.ParsedImports(r.modules[r.currentModuleID].Imports),
		rego.ParsedPackage(r.modules[r.currentModuleID].Package),
		rego.ParsedQuery(body),
		rego.ParsedInput(input),
		rego.Metrics(r.metrics),
		rego.Tracer(buf),
		rego.Instrument(r.instrument),
	)

	rs, err := eval.Eval(ctx)

	if buf != nil {
		r.printTrace(ctx, compiler, *buf)
	}

	if r.metrics != nil {
		r.printMetrics(r.metrics)
	}

	if err != nil {
		return err
	}

	if len(rs) == 0 {
		fmt.Fprintln(r.output, "undefined")
		return nil
	}

	if len(rs) == 1 {
		if len(rs[0].Bindings) == 0 && len(rs[0].Expressions) == 1 {
			r.printJSON(rs[0].Expressions[0].Value)
			return nil
		}
	}

	keys := []resultKey{}

	for k := range rs[0].Bindings {
		keys = append(keys, resultKey{
			varName: k,
		})
	}

	for i, expr := range rs[0].Expressions {
		if _, ok := expr.Value.(bool); !ok {
			keys = append(keys, resultKey{
				exprIndex: i,
				exprText:  expr.Text,
			})
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		return resultKeyLess(keys[i], keys[j])
	})

	r.printResults(keys, rs)

	return nil
}

func (r *REPL) evalPartial(ctx context.Context, compiler *ast.Compiler, input ast.Value, body ast.Body) error {

	q := topdown.NewQuery(body).
		WithCompiler(compiler).
		WithStore(r.store).
		WithTransaction(r.txn).
		WithUnknowns(r.unknowns)

	if input != nil {
		q = q.WithInput(ast.NewTerm(input))
	}

	if r.metrics != nil {
		q = q.WithMetrics(r.metrics)
	}

	if r.instrument {
		q = q.WithInstrumentation(topdown.NewInstrumentation(r.metrics))
	}

	var buf *topdown.BufferTracer

	if r.explain != explainOff {
		buf = topdown.NewBufferTracer()
		q = q.WithTracer(buf)
	}

	queries, support, err := q.PartialRun(ctx)
	if err != nil {
		return err
	}

	if buf != nil {
		r.printTrace(ctx, compiler, *buf)
	}

	for i := range queries {
		fmt.Fprintln(r.output, queries[i])
	}

	for i := range support {
		fmt.Fprintln(r.output)
		fmt.Fprintf(r.output, "# support module %d\n", i+1)
		fmt.Fprintln(r.output, support[i])
	}

	return nil
}

func (r *REPL) evalImport(i *ast.Import) error {
	mod := r.modules[r.currentModuleID]

	for _, other := range mod.Imports {
		if other.Equal(i) {
			return nil
		}
	}

	mod.Imports = append(mod.Imports, i)

	return nil
}

func (r *REPL) evalPackage(p *ast.Package) error {
	moduleID := p.Path.String()

	if _, ok := r.modules[moduleID]; ok {
		r.currentModuleID = moduleID
		return nil
	}

	r.modules[moduleID] = &ast.Module{
		Package: p,
	}

	r.currentModuleID = moduleID

	return nil
}

func (r *REPL) getPrompt() string {
	if len(r.buffer) > 0 {
		return r.bufferPrompt
	}
	return r.initPrompt
}

func (r *REPL) loadHistory(prompt *liner.State) {
	if f, err := os.Open(r.historyPath); err == nil {
		prompt.ReadHistory(f)
		f.Close()
	}
}

func (r *REPL) loadModules(ctx context.Context, txn storage.Transaction) (map[string]*ast.Module, error) {

	ids, err := r.store.ListPolicies(ctx, txn)
	if err != nil {
		return nil, err
	}

	modules := make(map[string]*ast.Module, len(ids))

	for _, id := range ids {
		bs, err := r.store.GetPolicy(ctx, txn, id)
		if err != nil {
			return nil, err
		}

		parsed, err := ast.ParseModule(id, string(bs))
		if err != nil {
			return nil, err
		}

		modules[id] = parsed
	}

	return modules, nil
}

func (r *REPL) printResults(keys []resultKey, results rego.ResultSet) {
	switch r.outputFormat {
	case "json":
		output := make([]map[string]interface{}, len(results))
		for i := range output {
			output[i] = results[i].Bindings
		}
		r.printJSON(output)
	default:
		r.printPretty(keys, results)
	}
}

func (r *REPL) printJSON(x interface{}) {
	buf, err := json.MarshalIndent(x, "", "  ")
	if err != nil {
		fmt.Fprintln(r.output, err)
		return
	}
	fmt.Fprintln(r.output, string(buf))
}

func (r *REPL) printPretty(keys []resultKey, results rego.ResultSet) {
	table := tablewriter.NewWriter(r.output)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoFormatHeaders(false)
	header := make([]string, len(keys))
	for i := range header {
		header[i] = keys[i].String()
	}
	table.SetHeader(header)
	for _, row := range results {
		r.printPrettyRow(table, keys, row)
	}
	table.Render()
}

func (r *REPL) printPrettyRow(table *tablewriter.Table, keys []resultKey, result rego.Result) {
	buf := []string{}
	for _, k := range keys {
		v, ok := k.Select(result)
		if ok {
			js, err := json.Marshal(v)
			if err != nil {
				buf = append(buf, err.Error())
			} else {
				s := string(js)
				if r.prettyLimit > 0 && len(s) > r.prettyLimit {
					s = s[:r.prettyLimit] + "..."
				}
				buf = append(buf, s)
			}
		}
	}
	table.Append(buf)
}

func (r *REPL) printTrace(ctx context.Context, compiler *ast.Compiler, trace []*topdown.Event) {
	mangleTrace(ctx, r.store, r.txn, trace)
	topdown.PrettyTrace(r.output, trace)
}

func (r *REPL) printMetrics(metrics metrics.Metrics) {
	buf, err := json.MarshalIndent(metrics.All(), "", "  ")
	if err != nil {
		panic("not reached")
	}

	r.output.Write(buf)
	fmt.Fprintln(r.output)
}

func (r *REPL) printTypes(ctx context.Context, typeEnv *ast.TypeEnv, body ast.Body) {

	ast.WalkRefs(body, func(ref ast.Ref) bool {
		fmt.Fprintf(r.output, "# %v: %v\n", ref, typeEnv.Get(ref))
		return false
	})

	vis := ast.NewVarVisitor().WithParams(ast.VarVisitorParams{
		SkipRefHead: true,
	})

	ast.Walk(vis, body)

	for v := range vis.Vars() {
		fmt.Fprintf(r.output, "# %v: %v\n", v, typeEnv.Get(v))
	}
}

func (r *REPL) saveHistory(prompt *liner.State) {
	if f, err := os.Create(r.historyPath); err == nil {
		prompt.WriteHistory(f)
		f.Close()
	}
}

type resultKey struct {
	varName   string
	exprIndex int
	exprText  string
}

func resultKeyLess(a, b resultKey) bool {
	if a.varName != "" {
		if b.varName == "" {
			return true
		}
		return a.varName < b.varName
	}
	return a.exprIndex < b.exprIndex
}

func (rk resultKey) String() string {
	if rk.varName != "" {
		return rk.varName
	}
	return rk.exprText
}

func (rk resultKey) Select(result rego.Result) (interface{}, bool) {
	if rk.varName != "" {
		return result.Bindings[rk.varName], true
	}
	val := result.Expressions[rk.exprIndex].Value
	if _, ok := val.(bool); ok {
		return nil, false
	}
	return val, true
}

type commandDesc struct {
	name string
	args []string
	help string
}

func (c commandDesc) syntax() string {
	if len(c.args) > 0 {
		return fmt.Sprintf("%v %v", c.name, strings.Join(c.args, " "))
	}
	return c.name
}

type exampleDesc struct {
	example string
	comment string
}

var examples = [...]exampleDesc{
	{"data", "show all documents"},
	{"data[x] = _", "show all top level keys"},
	{"data.repl.version", "drill into specific document"},
}

var extra = [...]commandDesc{
	{"<stmt>", []string{}, "evaluate the statement"},
	{"package", []string{"<term>"}, "change active package"},
	{"import", []string{"<term>"}, "add import to active module"},
}

var builtin = [...]commandDesc{
	{"show", []string{}, "show active module definition"},
	{"unset", []string{"<var>"}, "undefine rules in currently active module"},
	{"json", []string{}, "set output format to JSON"},
	{"pretty", []string{}, "set output format to pretty"},
	{"pretty-limit", []string{}, "set pretty value output limit"},
	{"trace", []string{}, "toggle full trace"},
	{"metrics", []string{}, "toggle metrics"},
	{"instrument", []string{}, "toggle instrumentation"},
	{"types", []string{}, "toggle type information"},
	{"partial", []string{"[ref-1 [ref-2 [...]]]"}, "toggle partial evaluation mode"},
	{"dump", []string{"[path]"}, "dump raw data in storage"},
	{"help", []string{"[topic]"}, "print this message"},
	{"exit", []string{}, "exit out of shell (or ctrl+d)"},
	{"ctrl+l", []string{}, "clear the screen"},
}

type topicDesc struct {
	fn      func(io.Writer) error
	comment string
}

var topics = map[string]topicDesc{
	"input":   {printHelpInput, "how to set input document"},
	"partial": {printHelpPartial, "how to use partial evaluation"},
}

type command struct {
	op   string
	args []string
}

func newCommand(line string) *command {
	p := strings.Fields(strings.TrimSpace(strings.ToLower(line)))
	if len(p) == 0 {
		return nil
	}
	for _, c := range builtin {
		if c.name == p[0] {
			return &command{
				op:   c.name,
				args: p[1:],
			}
		}
	}
	return nil
}

func dumpStorage(ctx context.Context, store storage.Store, txn storage.Transaction, w io.Writer) error {
	data, err := store.Read(ctx, txn, storage.Path{})
	if err != nil {
		return err
	}
	e := json.NewEncoder(w)
	return e.Encode(data)
}

func mangleTrace(ctx context.Context, store storage.Store, txn storage.Transaction, trace []*topdown.Event) error {
	for _, event := range trace {
		if err := mangleEvent(ctx, store, txn, event); err != nil {
			return err
		}
	}
	return nil
}

func mangleEvent(ctx context.Context, store storage.Store, txn storage.Transaction, event *topdown.Event) error {

	// Replace bindings with ref values with the values from storage.
	cpy := event.Locals.Copy()
	var err error
	event.Locals.Iter(func(k, v ast.Value) bool {
		if r, ok := v.(ast.Ref); ok {
			var path storage.Path
			path, err = storage.NewPathForRef(r)
			if err != nil {
				return true
			}
			var doc interface{}
			doc, err = store.Read(ctx, txn, path)
			if err != nil {
				return true
			}
			v, err = ast.InterfaceToValue(doc)
			if err != nil {
				return true
			}
			cpy.Put(k, v)
		}
		return false
	})
	event.Locals = cpy

	switch node := event.Node.(type) {
	case *ast.Rule:
		event.Node = node.Head //topdown.PlugHead(node.Head, event.Locals.Get)
	case *ast.Expr:
		event.Node = node // topdown.PlugExpr(node, event.Locals.Get)
	}
	return nil
}

func printHelp(output io.Writer, initPrompt string) {
	printHelpExamples(output, initPrompt)
	printHelpCommands(output)
}

func printHelpExamples(output io.Writer, promptSymbol string) {

	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "Examples")
	fmt.Fprintln(output, "========")
	fmt.Fprintln(output, "")

	maxLength := 0
	for _, ex := range examples {
		if len(ex.example) > maxLength {
			maxLength = len(ex.example)
		}
	}

	f := fmt.Sprintf("%v%%-%dv # %%v\n", promptSymbol, maxLength+1)

	for _, ex := range examples {
		fmt.Fprintf(output, f, ex.example, ex.comment)
	}

	fmt.Fprintln(output, "")
}

func printHelpCommands(output io.Writer) {

	all := extra[:]
	all = append(all, builtin[:]...)

	// Compute max length of all command and topic names.
	names := []string{}

	for _, x := range all {
		names = append(names, x.syntax())
	}
	for x := range topics {
		names = append(names, "help "+x)
	}

	maxLength := 0

	for _, name := range names {
		length := len(name)
		if length > maxLength {
			maxLength = length
		}
	}

	f := fmt.Sprintf("%%%dv : %%v\n", maxLength)

	// Print out command help.
	fmt.Fprintln(output, "Commands")
	fmt.Fprintln(output, "========")
	fmt.Fprintln(output, "")

	for _, c := range all {
		fmt.Fprintf(output, f, c.syntax(), c.help)
	}

	fmt.Fprintln(output, "")

	// Print out topic help.
	fmt.Fprintln(output, "Additional Topics")
	fmt.Fprintln(output, "=================")
	fmt.Fprintln(output, "")

	for key, desc := range topics {
		fmt.Fprintf(output, f, "help "+key, desc.comment)
	}

	fmt.Fprintln(output, "")
}

func printHelpInput(output io.Writer) error {

	printHelpTitle(output, "Input")

	txt := strings.TrimSpace(`
Rego allows queries to refer to documents outside of the storage layer. These
documents must be provided as inputs to the query engine. In Rego, these values
are nested under the root "input" document.

In the interactive shell, users can set the value for the "input" document by
defining documents under the repl.input package.

For example:

	# Change to the repl.input package.
	> package repl.input

	# Define a new document called "params".
	> params = {"method": "POST", "path": "/some/path"}

	# Switch back to another package to test access to input.
	> package opa.example

	# Import "params" defined above.
	> import input.params

	# Define rule that refers to "params".
	> is_post { params.method = "POST" }

	# Test evaluation.
	> is_post
	true`) + "\n"

	fmt.Fprintln(output, txt)
	return nil
}

func printHelpPartial(output io.Writer) error {

	printHelpTitle(output, "Partial")

	txt := strings.TrimSpace(`
Rego queries can be partially evaluated with respect to the specific variables,
inputs, or any document rooted under data. The result of partial evaluation is
a new set of queries that can be evaluated later.

For example:

	> allowed_methods = ["GET", "HEAD"]

	# Enable partial evaluation. Treat input document as unknown.
	> partial input

	# Partially evaluate a query.
	> method = allowed_methods[i]; input.method = method
	input.method = "GET"; i = 0; method = "GET"
	input.method = "HEAD"; i = 1; method = "HEAD"

	# Turn off partial evaluation by running the 'partial' command with no arguments.
	> partial`) + "\n"

	fmt.Fprintln(output, txt)
	return nil
}

func printHelpTitle(output io.Writer, title string) {
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, title)
	fmt.Fprintln(output, "=======")
	fmt.Fprintln(output, "")
}
