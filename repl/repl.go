// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package repl implements a Read-Eval-Print-Loop (REPL) for interacting with the policy engine.
//
// The REPL is typically used from the command line, however, it can also be used as a library.
package repl

import (
	"bytes"
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
	nextID          int
	txn             storage.Transaction

	// TODO(tsandall): replace this state with rule definitions
	// inside the default module.
	outputFormat string
	explain      explainMode
	historyPath  string
	initPrompt   string
	bufferPrompt string
	banner       string

	bufferDisabled    bool
	undefinedDisabled bool
}

type explainMode int

const (
	explainOff   explainMode = iota
	explainTrace explainMode = iota
	explainTruth explainMode = iota
)

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
func (r *REPL) Loop() {

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

	for true {

		input, err := line.Prompt(r.getPrompt())

		if err == liner.ErrPromptAborted || err == io.EOF {
			fmt.Fprintln(r.output, "Exiting")
			break
		}

		if err != nil {
			fmt.Fprintln(r.output, "error (fatal):", err)
			os.Exit(1)
		}

		if err := r.OneShot(input); err != nil {
			switch err := err.(type) {
			case stop:
				break
			default:
				fmt.Fprintln(r.output, "error:", err)
			}
		}

		line.AppendHistory(input)
	}

	r.saveHistory(line)
}

// OneShot evaluates the line and prints the result. If an error occurs it is
// returned for the caller to display.
func (r *REPL) OneShot(line string) error {

	var err error
	r.txn, err = r.store.NewTransaction()
	if err != nil {
		return err
	}

	defer r.store.Close(r.txn)

	if len(r.buffer) == 0 {
		if cmd := newCommand(line); cmd != nil {
			switch cmd.op {
			case "dump":
				return r.cmdDump(cmd.args)
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
			case "help":
				return r.cmdHelp()
			case "exit":
				return r.cmdExit()
			}
		}
		r.buffer = append(r.buffer, line)
		return r.evalBufferOne()
	}

	r.buffer = append(r.buffer, line)
	if len(line) == 0 {
		return r.evalBufferMulti()
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

	txn, err := r.store.NewTransaction()

	if err != nil {
		fmt.Fprintln(r.output, "error:", err)
		return c
	}

	defer r.store.Close(txn)

	mods := r.store.ListPolicies(txn)

	for _, mod := range mods {
		for _, rule := range mod.Rules {
			path := mod.Package.Path.String() + "." + rule.Name.String()
			if strings.HasPrefix(path, line) {
				c = append(c, path)
			}
		}
	}

	for _, mod := range r.modules {
		for _, rule := range mod.Rules {
			if r.isGeneratedRuleName(rule.Name) {
				continue
			}
			path := mod.Package.Path.String() + "." + rule.Name.String()
			if strings.HasPrefix(path, line) {
				c = append(c, path)
			}
		}
	}

	return c
}

func (r *REPL) cmdDump(args []string) error {
	if len(args) == 0 {
		return r.cmdDumpOutput()
	}
	return r.cmdDumpPath(args[0])
}

func (r *REPL) cmdDumpOutput() error {
	return dumpStorage(r.store, r.txn, r.output)
}

func (r *REPL) cmdDumpPath(filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return dumpStorage(r.store, r.txn, f)
}

func (r *REPL) cmdExit() error {
	return stop{}
}

func (r *REPL) cmdFormat(s string) error {
	r.outputFormat = s
	return nil
}

func (r *REPL) cmdHelp() error {
	fmt.Fprintln(r.output, "")
	printHelpExamples(r.output, r.initPrompt)
	printHelpCommands(r.output)
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
		return newBadArgsErr("argument must identify a rule")
	}

	mod := r.modules[r.currentModuleID]
	rules := []*ast.Rule{}

	for _, r := range mod.Rules {
		if !r.Name.Equal(v) {
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

func (r *REPL) compileBody(body ast.Body) (ast.Body, error) {

	policies := r.store.ListPolicies(r.txn)

	for id, mod := range r.modules {
		policies[id] = mod
	}

	compiler := ast.NewCompiler()

	if compiler.Compile(policies); compiler.Failed() {
		return nil, compiler.Errors
	}

	qctx := ast.NewQueryContextForModule(r.modules[r.currentModuleID])
	return compiler.QueryCompiler().
		WithContext(qctx).
		Compile(body)
}

func (r *REPL) compileRule(rule *ast.Rule) error {

	mod := r.modules[r.currentModuleID]
	prev := mod.Rules
	mod.Rules = append(mod.Rules, rule)

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

func (r *REPL) evalBufferOne() error {

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
		if err := r.evalStatement(stmt); err != nil {
			return err
		}
	}

	return nil
}

func (r *REPL) evalBufferMulti() error {

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
		if err := r.evalStatement(stmt); err != nil {
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

// loadGlobals returns the globals mapping currently defined in the REPL. The
// REPL loads globals from the data.repl.globals document.
func (r *REPL) loadGlobals(compiler *ast.Compiler) (*ast.ValueMap, error) {

	params := topdown.NewQueryParams(compiler, r.store, r.txn, nil, ast.MustParseRef("data.repl.globals"))

	result, err := topdown.Query(params)
	if err != nil {
		return nil, err
	}

	if result.Undefined() {
		return nil, nil
	}

	pairs := [][2]*ast.Term{}

	obj, ok := result[0].Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("globals is %T but expected object", result)
	}

	for k, v := range obj {

		gk, err := ast.ParseTerm(k)
		if err != nil {
			return nil, err
		}

		gv, err := ast.InterfaceToValue(v)
		if err != nil {
			return nil, err
		}

		pairs = append(pairs, [...]*ast.Term{gk, &ast.Term{Value: gv}})
	}

	return topdown.MakeGlobals(pairs)
}

func (r *REPL) evalStatement(stmt interface{}) error {
	switch s := stmt.(type) {
	case ast.Body:
		body, err := r.compileBody(s)
		if err != nil {
			return err
		}
		if rule := ast.ParseConstantRule(body); rule != nil {
			if err := r.compileRule(rule); err != nil {
				return err
			}
			return nil
		}
		compiler, err := r.loadCompiler()
		if err != nil {
			return err
		}
		globals, err := r.loadGlobals(compiler)
		if err != nil {
			return err
		}
		return r.evalBody(compiler, globals, body)
	case *ast.Rule:
		if err := r.compileRule(s); err != nil {
			fmt.Fprintln(r.output, "error:", err)
		}
	case *ast.Import:
		return r.evalImport(s)
	case *ast.Package:
		return r.evalPackage(s)
	}
	return nil
}

func (r *REPL) evalBody(compiler *ast.Compiler, globals *ast.ValueMap, body ast.Body) error {

	// Special case for positive, single term inputs.
	if len(body) == 1 {
		expr := body[0]
		if !expr.Negated {
			if _, ok := expr.Terms.(*ast.Term); ok {
				if singleValue(body) {
					return r.evalTermSingleValue(compiler, globals, body)
				}
				return r.evalTermMultiValue(compiler, globals, body)
			}
		}
	}

	ctx := topdown.NewContext(body, compiler, r.store, r.txn)
	ctx.Globals = globals

	var buf *topdown.BufferTracer

	if r.explain != explainOff {
		buf = topdown.NewBufferTracer()
		ctx.Tracer = buf
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
	err := topdown.Eval(ctx, func(ctx *topdown.Context) error {
		var err error
		row := map[string]interface{}{}
		ctx.Locals.Iter(func(k, v ast.Value) bool {
			kv, ok := k.(ast.Var)
			if !ok {
				return false
			}
			if kv.IsWildcard() {
				return false
			}
			r, e := topdown.ValueToInterface(v, ctx)
			if e != nil {
				err = e
				return true
			}
			row[k.String()] = r
			return false
		})

		if err != nil {
			return err
		}

		isTrue = true

		if len(row) > 0 {
			results = append(results, row)
		}

		return nil
	})

	if buf != nil {
		r.printTrace(compiler, *buf)
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
func (r *REPL) evalTermSingleValue(compiler *ast.Compiler, globals *ast.ValueMap, body ast.Body) error {

	term := body[0].Terms.(*ast.Term)
	outputVar := ast.Wildcard
	body = ast.NewBody(ast.Equality.Expr(term, outputVar))

	ctx := topdown.NewContext(body, compiler, r.store, r.txn)
	ctx.Globals = globals

	var buf *topdown.BufferTracer

	if r.explain != explainOff {
		buf = topdown.NewBufferTracer()
		ctx.Tracer = buf
	}

	var result interface{}
	isTrue := false

	err := topdown.Eval(ctx, func(ctx *topdown.Context) error {
		p := ctx.Locals.Get(outputVar.Value)
		v, err := topdown.ValueToInterface(p, ctx)
		if err != nil {
			return err
		}
		result = v
		isTrue = true
		return nil
	})

	if buf != nil {
		r.printTrace(compiler, *buf)
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
func (r *REPL) evalTermMultiValue(compiler *ast.Compiler, globals *ast.ValueMap, body ast.Body) error {

	// Mangle the expression in the same way we do for evalTermSingleValue. When handling the
	// evaluation result below, we will ignore this variable.
	term := body[0].Terms.(*ast.Term)
	outputVar := ast.Wildcard
	body = ast.NewBody(ast.Equality.Expr(term, outputVar))

	ctx := topdown.NewContext(body, compiler, r.store, r.txn)
	ctx.Globals = globals

	var buf *topdown.BufferTracer

	if r.explain != explainOff {
		buf = topdown.NewBufferTracer()
		ctx.Tracer = buf
	}

	vars := map[string]struct{}{}
	results := []map[string]interface{}{}
	resultKey := string(term.Location.Text)

	// Do not include the value of the input term if the input term was a set reference. E.g.,
	// for "p[x]", the value users are interested in is "x" not p[x] which is always defined
	// as true.
	includeValue := !r.isSetReference(compiler, term)

	err := topdown.Eval(ctx, func(ctx *topdown.Context) error {

		result := map[string]interface{}{}

		var err error

		ctx.Locals.Iter(func(k, v ast.Value) bool {
			if k, ok := k.(ast.Var); ok {
				if k.IsWildcard() || k.Equal(outputVar.Value) {
					return false
				}
				x, e := topdown.ValueToInterface(v, ctx)
				if e != nil {
					err = e
					return true
				}
				s := string(k)
				result[s] = x
				vars[s] = struct{}{}
			}
			return false
		})

		if err != nil {
			return err
		}

		if includeValue {
			p := topdown.PlugTerm(term, ctx.Binding)
			v, err := topdown.ValueToInterface(p.Value, ctx)
			if err != nil {
				return err
			}
			result[resultKey] = v
		}

		results = append(results, result)

		return nil
	})

	if buf != nil {
		r.printTrace(compiler, *buf)
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
	if rs[0].DocKind() == ast.PartialSetDoc {
		return true
	}
	if rs[0].Value == nil {
		return false
	}
	_, ok = rs[0].Value.Value.(*ast.Set)
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

func (r *REPL) printTrace(compiler *ast.Compiler, trace []*topdown.Event) {
	if r.explain == explainTruth {
		answer, err := explain.Truth(compiler, trace)
		if err != nil {
			fmt.Fprintf(r.output, "error: %v\n", err)
		}
		trace = answer
	}
	mangleTrace(r.store, r.txn, trace)
	topdown.PrettyTrace(r.output, trace)
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

func (r *REPL) generateRuleName() ast.Var {
	name := fmt.Sprintf("repl%d", r.nextID)
	r.nextID++
	return ast.Var(name)
}

func (r *REPL) isGeneratedRuleName(name ast.Var) bool {
	return strings.HasPrefix(string(name), "repl")
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
	{"truth", []string{}, "toggle truth explanation"},
	{"dump", []string{"[path]"}, "dump raw data in storage"},
	{"help", []string{}, "print this message"},
	{"exit", []string{}, "exit back to shell (or ctrl+c, ctrl+d)"},
	{"ctrl+l", []string{}, "clear the screen"},
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

func dumpStorage(store *storage.Storage, txn storage.Transaction, w io.Writer) error {
	data, err := store.Read(txn, ast.Ref{ast.DefaultRootDocument})
	if err != nil {
		return err
	}
	e := json.NewEncoder(w)
	return e.Encode(data)
}

func mangleTrace(store *storage.Storage, txn storage.Transaction, trace []*topdown.Event) error {
	for _, event := range trace {
		if err := mangleEvent(store, txn, event); err != nil {
			return err
		}
	}
	return nil
}

func mangleEvent(store *storage.Storage, txn storage.Transaction, event *topdown.Event) error {

	// Replace bindings with ref values with the values from storage.
	cpy := event.Locals.Copy()
	var err error
	event.Locals.Iter(func(k, v ast.Value) bool {
		if r, ok := v.(ast.Ref); ok {
			var doc interface{}
			doc, err = store.Read(txn, r)
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
		event.Node = topdown.PlugHead(node.Head(), event.Locals.Get)
	case *ast.Expr:
		event.Node = topdown.PlugExpr(node, event.Locals.Get)
	}
	return nil
}

func printHelpExamples(output io.Writer, promptSymbol string) {

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

	fmt.Fprintln(output, "Commands")
	fmt.Fprintln(output, "========")
	fmt.Fprintln(output, "")

	all := extra[:]
	all = append(all, builtin[:]...)

	maxLength := 0

	for _, c := range all {
		length := len(c.syntax())
		if length > maxLength {
			maxLength = length
		}
	}

	f := fmt.Sprintf("%%%dv : %%v\n", maxLength)

	for _, c := range all {
		fmt.Fprintf(output, f, c.syntax(), c.help)
	}

	fmt.Fprintln(output, "")
}
