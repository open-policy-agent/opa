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
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/explain"
	"github.com/open-policy-agent/opa/version"
	"github.com/peterh/liner"
)

// REPL represents an instance of the interactive shell.
type REPL struct {
	output io.Writer
	store  *storage.Storage

	modules         map[string]*ast.Module
	currentModuleID string
	buffer          []string
	txn             storage.Transaction

	// TODO(tsandall): replace this state with rule definitions
	// inside the default module.
	outputFormat      string
	explain           explainMode
	historyPath       string
	initPrompt        string
	bufferPrompt      string
	banner            string
	types             bool
	bufferDisabled    bool
	undefinedDisabled bool
}

type explainMode int

const (
	explainOff   explainMode = iota
	explainTrace explainMode = iota
	explainTruth explainMode = iota
)

const exitPromptMessage = "Do you want to exit ([y]/n)? "

// New returns a new instance of the REPL.
func New(store *storage.Storage, historyPath string, output io.Writer, outputFormat string, banner string) *REPL {

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

	defer r.store.Close(ctx, r.txn)

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
				return r.cmdUnset(cmd.args)
			case "pretty":
				return r.cmdFormat("pretty")
			case "trace":
				return r.cmdTrace()
			case "truth":
				return r.cmdTruth()
			case "types":
				return r.cmdTypes()
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

func (r *REPL) complete(line string) (c []string) {

	ctx := context.Background()
	txn, err := r.store.NewTransaction(ctx)

	if err != nil {
		fmt.Fprintln(r.output, "error:", err)
		return c
	}

	defer r.store.Close(ctx, txn)

	// add imports
	for _, mod := range r.modules {
		for _, imp := range mod.Imports {
			path := imp.Name().String()
			if strings.HasPrefix(path, line) {
				c = append(c, path)
			}
		}
	}

	// add virtual docs defined in repl
	for _, mod := range r.modules {
		for _, rule := range mod.Rules {
			path := rule.Path().String()
			if strings.HasPrefix(path, line) {
				c = append(c, path)
			}
		}
	}

	mods := r.store.ListPolicies(txn)

	// add virtual docs defined by policies
	for _, mod := range mods {
		for _, rule := range mod.Rules {
			path := rule.Path().String()
			if strings.HasPrefix(path, line) {
				c = append(c, path)
			}
		}
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
	fmt.Fprintln(r.output, module)
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

func (r *REPL) cmdTruth() error {
	if r.explain == explainTruth {
		r.explain = explainOff
	} else {
		r.explain = explainTruth
	}
	return nil
}

func (r *REPL) cmdTypes() error {
	r.types = !r.types
	return nil
}

func (r *REPL) cmdUnset(args []string) error {

	if len(args) != 1 {
		return newBadArgsErr("unset <var>: expects exactly one argument")
	}

	term, err := ast.ParseTerm(args[0])

	if err != nil {
		return newBadArgsErr("argument must identify a rule")
	}

	v, ok := term.Value.(ast.Var)

	if !ok {
		if !ast.RootDocumentRefs.Contains(term) {
			return newBadArgsErr("argument must identify a rule")
		}
		v = term.Value.(ast.Ref)[0].Value.(ast.Var)
	}

	mod := r.modules[r.currentModuleID]
	rules := []*ast.Rule{}

	for _, r := range mod.Rules {
		if !r.Head.Name.Equal(v) {
			rules = append(rules, r)
		}
	}

	if len(rules) == len(mod.Rules) {
		fmt.Fprintln(r.output, "warning: no matching rules in current module")
		return nil
	}

	cpy := mod.Copy()
	cpy.Rules = rules

	policies := r.store.ListPolicies(r.txn)
	policies[r.currentModuleID] = cpy

	for id, mod := range r.modules {
		if id != r.currentModuleID {
			policies[id] = mod
		}
	}

	compiler := ast.NewCompiler()

	if compiler.Compile(policies); compiler.Failed() {
		return compiler.Errors
	}

	r.modules[r.currentModuleID] = cpy

	return nil
}

func (r *REPL) compileBody(body ast.Body, input ast.Value) (ast.Body, *ast.TypeEnv, error) {

	policies := r.store.ListPolicies(r.txn)

	for id, mod := range r.modules {
		policies[id] = mod
	}

	compiler := ast.NewCompiler()

	if compiler.Compile(policies); compiler.Failed() {
		return nil, nil, compiler.Errors
	}

	qctx := ast.NewQueryContext().
		WithPackage(r.modules[r.currentModuleID].Package).
		WithImports(r.modules[r.currentModuleID].Imports).
		WithInput(input)

	qc := compiler.QueryCompiler()
	body, err := qc.WithContext(qctx).Compile(body)
	return body, qc.TypeEnv(), err
}

func (r *REPL) compileRule(rule *ast.Rule) error {

	mod := r.modules[r.currentModuleID]
	prev := mod.Rules
	mod.Rules = append(mod.Rules, rule)
	rule.Module = mod

	policies := r.store.ListPolicies(r.txn)
	for id, mod := range r.modules {
		policies[id] = mod
	}

	compiler := ast.NewCompiler()

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
	stmts, err := ast.ParseStatements("", line)
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

	stmts, err := ast.ParseStatements("", line)

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

func (r *REPL) loadCompiler() (*ast.Compiler, error) {

	policies := r.store.ListPolicies(r.txn)
	for id, mod := range r.modules {
		policies[id] = mod
	}

	compiler := ast.NewCompiler()

	if compiler.Compile(policies); compiler.Failed() {
		return nil, compiler.Errors
	}

	return compiler, nil
}

// loadInput returns the input defined in the REPL. The REPL loads the
// input from the data.repl.input document.
func (r *REPL) loadInput(ctx context.Context) (ast.Value, error) {

	compiler, err := r.loadCompiler()
	if err != nil {
		return nil, err
	}

	params := topdown.NewQueryParams(ctx, compiler, r.store, r.txn, nil, ast.MustParseRef("data.repl.input"))
	result, err := topdown.Query(params)

	if err != nil {
		return nil, err
	}

	if result.Undefined() {
		return nil, nil
	}

	return ast.InterfaceToValue(result[0].Result)
}

func (r *REPL) evalStatement(ctx context.Context, stmt interface{}) error {
	switch s := stmt.(type) {
	case ast.Body:
		input, err := r.loadInput(ctx)
		if err != nil {
			return err
		}

		body, typeEnv, err := r.compileBody(s, input)

		if err != nil {
			// The compile step can fail if input is undefined. In that case,
			// we still want to allow users to define an input document (e.g.,
			// "input = { ... }") so try to parse a rule nonetheless.
			if ast.IsError(ast.InputErr, err) {
				rule, err2 := ast.ParseRuleFromBody(r.modules[r.currentModuleID], s)
				if err2 != nil {
					// The statement cannot be understood as a rule, so the original
					// error returned from compiling the query should be given the
					// caller.
					return err
				}
				return r.compileRule(rule)
			}
			return err
		}

		rule, err3 := ast.ParseRuleFromBody(r.modules[r.currentModuleID], body)
		if err3 == nil {
			return r.compileRule(rule)
		}

		compiler, err := r.loadCompiler()
		if err != nil {
			return err
		}

		err = r.evalBody(ctx, compiler, input, body)

		if r.types {
			r.printTypes(ctx, typeEnv, body)
		}

		return err
	case *ast.Rule:
		return r.compileRule(s)
	case *ast.Import:
		return r.evalImport(s)
	case *ast.Package:
		return r.evalPackage(s)
	}
	return nil
}

func (r *REPL) evalBody(ctx context.Context, compiler *ast.Compiler, input ast.Value, body ast.Body) error {

	// Special case for positive, single term inputs.
	if len(body) == 1 {
		expr := body[0]
		if !expr.Negated {
			if _, ok := expr.Terms.(*ast.Term); ok {
				if singleValue(body) {
					return r.evalTermSingleValue(ctx, compiler, input, body)
				}
				return r.evalTermMultiValue(ctx, compiler, input, body)
			}
		}
	}

	t := topdown.New(ctx, body, compiler, r.store, r.txn)
	t.Input = input

	var buf *topdown.BufferTracer

	if r.explain != explainOff {
		buf = topdown.NewBufferTracer()
		t.Tracer = buf
	}

	// Flag indicates whether the query was defined for some context.
	// If the query does not include any ground terms, the results will
	// be empty, but we still want to output "true". If there are
	// no results, this will remain "false" and we will output "false".
	var isTrue = false

	// Store bindings as slice of maps where map keys are variables
	// and values are the underlying Go values.
	var results []map[string]interface{}

	// Execute query and accumulate results.
	err := topdown.Eval(t, func(t *topdown.Topdown) error {

		row := map[string]interface{}{}

		for k, v := range t.Vars() {
			if !k.IsWildcard() {
				x, err := ast.ValueToInterface(v, t)
				if err != nil {
					return err
				}
				row[k.String()] = x
			}
		}

		isTrue = true

		if len(row) > 0 {
			results = append(results, row)
		}

		return nil
	})

	if buf != nil {
		r.printTrace(ctx, compiler, *buf)
	}

	if err != nil {
		return err
	}

	if isTrue {
		if len(results) >= 1 {
			r.printResults(getHeaderForBody(body), results)
		} else {
			fmt.Fprintln(r.output, "true")
		}
	} else {
		fmt.Fprintln(r.output, "false")
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

// evalTermSingleValue evaluates and prints terms in cases where the term evaluates to a
// single value, e.g., "1", true, [1,2,"foo"], [x | x = a[i], a = [1,2,3]], etc. Ground terms
// and comprehensions always evaluate to a single value. To handle references, this function
// still executes the query, except it does so by rewriting the body to assign the term
// to a variable. This allows the REPL to obtain the result even if the term is false.
func (r *REPL) evalTermSingleValue(ctx context.Context, compiler *ast.Compiler, input ast.Value, body ast.Body) error {

	term := body[0].Terms.(*ast.Term)
	outputVar := ast.Wildcard
	expr := ast.Equality.Expr(term, outputVar)
	for _, with := range body[0].With {
		expr = expr.IncludeWith(with.Target, with.Value)
	}
	expr.Location = body.Loc()
	body = ast.NewBody(expr)

	t := topdown.New(ctx, body, compiler, r.store, r.txn)
	t.Input = input

	var buf *topdown.BufferTracer

	if r.explain != explainOff {
		buf = topdown.NewBufferTracer()
		t.Tracer = buf
	}

	var result interface{}
	isTrue := false

	err := topdown.Eval(t, func(t *topdown.Topdown) error {
		p := t.Binding(outputVar.Value)
		v, err := ast.ValueToInterface(p, t)
		if err != nil {
			return err
		}
		result = v
		isTrue = true
		return nil
	})

	if buf != nil {
		r.printTrace(ctx, compiler, *buf)
	}

	if err != nil {
		return err
	}

	if isTrue {
		r.printJSON(result)
	} else if !r.undefinedDisabled {
		r.printUndefined()
	}

	return nil
}

// evalTermMultiValue evaluates and prints terms in cases where the term may evaluate to multiple
// ground values, e.g., a[i], [servers[x]], etc.
func (r *REPL) evalTermMultiValue(ctx context.Context, compiler *ast.Compiler, input ast.Value, body ast.Body) error {

	// Mangle the expression in the same way we do for evalTermSingleValue. When handling the
	// evaluation result below, we will ignore this variable.
	term := body[0].Terms.(*ast.Term)
	outputVar := ast.Wildcard
	expr := ast.Equality.Expr(term, outputVar)
	for _, with := range body[0].With {
		expr = expr.IncludeWith(with.Target, with.Value)
	}
	expr.Location = body.Loc()
	body = ast.NewBody(expr)

	t := topdown.New(ctx, body, compiler, r.store, r.txn)
	t.Input = input

	var buf *topdown.BufferTracer

	if r.explain != explainOff {
		buf = topdown.NewBufferTracer()
		t.Tracer = buf
	}

	vars := map[string]struct{}{}
	results := []map[string]interface{}{}
	resultKey := string(term.Location.Text)

	// Do not include the value of the input term if the input term was a set reference. E.g.,
	// for "p[x]", the value users are interested in is "x" not p[x] which is always defined
	// as true.
	includeValue := !r.isSetReference(compiler, term)

	err := topdown.Eval(t, func(t *topdown.Topdown) error {

		result := map[string]interface{}{}

		for k, v := range t.Vars() {
			if !k.IsWildcard() && !k.Equal(outputVar.Value) {
				x, err := ast.ValueToInterface(v, t)
				if err != nil {
					return err
				}
				result[k.String()] = x
				vars[k.String()] = struct{}{}
			}
		}

		if includeValue {
			p := topdown.PlugTerm(term, t.Binding)
			v, err := ast.ValueToInterface(p.Value, t)
			if err != nil {
				return err
			}
			result[resultKey] = v
		}

		results = append(results, result)

		return nil
	})

	if buf != nil {
		r.printTrace(ctx, compiler, *buf)
	}

	if err != nil {
		return err
	}

	if len(results) > 0 {
		keys := []string{}
		for v := range vars {
			keys = append(keys, v)
		}
		sort.Strings(keys)
		if includeValue {
			keys = append(keys, resultKey)
		}
		r.printResults(keys, results)
	} else if !r.undefinedDisabled {
		r.printUndefined()
	}

	return nil
}

func (r *REPL) getPrompt() string {
	if len(r.buffer) > 0 {
		return r.bufferPrompt
	}
	return r.initPrompt
}

// isSetReference returns true if term is a reference that refers to a set document.
func (r *REPL) isSetReference(compiler *ast.Compiler, term *ast.Term) bool {
	ref, ok := term.Value.(ast.Ref)
	if !ok {
		return false
	}
	rs := compiler.GetRulesExact(ref.GroundPrefix())
	if rs == nil {
		return false
	}
	if rs[0].Head.DocKind() == ast.PartialSetDoc {
		return true
	}
	if rs[0].Head.Value == nil {
		return false
	}
	_, ok = rs[0].Head.Value.Value.(*ast.Set)
	return ok
}

func (r *REPL) loadHistory(prompt *liner.State) {
	if f, err := os.Open(r.historyPath); err == nil {
		prompt.ReadHistory(f)
		f.Close()
	}
}

func (r *REPL) printResults(keys []string, results []map[string]interface{}) {
	switch r.outputFormat {
	case "json":
		r.printJSON(results)
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

func (r *REPL) printPretty(keys []string, results []map[string]interface{}) {
	table := tablewriter.NewWriter(r.output)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoFormatHeaders(false)
	table.SetHeader(keys)
	for _, row := range results {
		r.printPrettyRow(table, keys, row)
	}
	table.Render()
}

func (r *REPL) printPrettyRow(table *tablewriter.Table, keys []string, row map[string]interface{}) {

	buf := []string{}
	for _, k := range keys {
		js, err := json.Marshal(row[k])
		if err != nil {
			buf = append(buf, err.Error())
		} else {
			buf = append(buf, string(js))
		}
	}

	// Add fields to table in sorted order.
	table.Append(buf)
}

func (r *REPL) printTrace(ctx context.Context, compiler *ast.Compiler, trace []*topdown.Event) {
	if r.explain == explainTruth {
		answer, err := explain.Truth(compiler, trace)
		if err != nil {
			fmt.Fprintf(r.output, "error: %v\n", err)
		}
		trace = answer
	}
	mangleTrace(ctx, r.store, r.txn, trace)
	topdown.PrettyTrace(r.output, trace)
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

func (r *REPL) printUndefined() {
	fmt.Fprintln(r.output, "undefined")
}

func (r *REPL) saveHistory(prompt *liner.State) {
	if f, err := os.Create(r.historyPath); err == nil {
		prompt.WriteHistory(f)
		f.Close()
	}
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
	{"trace", []string{}, "toggle full trace"},
	{"types", []string{}, "toggle type information"},
	{"truth", []string{}, "toggle truth explanation"},
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
	"input": {printHelpInput, "how to set input document"},
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

func buildHeader(fields map[string]struct{}, term *ast.Term) {
	switch v := term.Value.(type) {
	case ast.Ref:
		for _, t := range v[1:] {
			buildHeader(fields, t)
		}
	case ast.Var:
		if !v.IsWildcard() {
			s := string(v)
			fields[s] = struct{}{}
		}
	case ast.Object:
		for _, i := range v {
			buildHeader(fields, i[0])
			buildHeader(fields, i[1])
		}
	case ast.Array:
		for _, e := range v {
			buildHeader(fields, e)
		}
	}
}

func getHeaderForBody(body ast.Body) []string {
	// Build set of fields for the output. The fields are the variables from inside the body.
	// If the variable appears multiple times, we only want a single field so store them in a
	// map/set.
	fields := map[string]struct{}{}

	// TODO(tsandall): perhaps we could refactor this to use a "walk" function on the body.
	for _, expr := range body {
		switch ts := expr.Terms.(type) {
		case []*ast.Term:
			for _, t := range ts[1:] {
				buildHeader(fields, t)
			}
		case *ast.Term:
			buildHeader(fields, ts)
		}
	}

	// Sort/display fields by name.
	keys := []string{}
	for k := range fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	return keys
}

// singleValue returns true if body can be evaluated to a single term.
func singleValue(body ast.Body) bool {
	if len(body) != 1 {
		return false
	}
	term, ok := body[0].Terms.(*ast.Term)
	if !ok {
		return false
	}
	switch term.Value.(type) {
	case *ast.ArrayComprehension:
		return true
	default:
		return term.IsGround()
	}
}

func dumpStorage(ctx context.Context, store *storage.Storage, txn storage.Transaction, w io.Writer) error {
	data, err := store.Read(ctx, txn, storage.Path{})
	if err != nil {
		return err
	}
	e := json.NewEncoder(w)
	return e.Encode(data)
}

func mangleTrace(ctx context.Context, store *storage.Storage, txn storage.Transaction, trace []*topdown.Event) error {
	for _, event := range trace {
		if err := mangleEvent(ctx, store, txn, event); err != nil {
			return err
		}
	}
	return nil
}

func mangleEvent(ctx context.Context, store *storage.Storage, txn storage.Transaction, event *topdown.Event) error {

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
		event.Node = topdown.PlugHead(node.Head, event.Locals.Get)
	case *ast.Expr:
		event.Node = topdown.PlugExpr(node, event.Locals.Get)
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
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, "Input")
	fmt.Fprintln(output, "=======")
	fmt.Fprintln(output, "")

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
