// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package repl

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/peterh/liner"
)

// REPL represents an instance of the interactive shell.
type REPL struct {
	output      io.Writer
	dataStore   *storage.DataStore
	policyStore *storage.PolicyStore

	currentModuleID string
	buffer          []string
	initialized     bool
	nextID          int

	// TODO(tsandall): replace this state with rule definitions
	// inside the default module.
	outputFormat string
	trace        bool
	historyPath  string
	initPrompt   string
	bufferPrompt string
}

// New returns a new instance of the REPL.
func New(dataStore *storage.DataStore, policyStore *storage.PolicyStore, historyPath string, output io.Writer, outputFormat string) *REPL {
	return &REPL{
		output:          output,
		outputFormat:    outputFormat,
		trace:           false,
		dataStore:       dataStore,
		policyStore:     policyStore,
		currentModuleID: "repl",
		historyPath:     historyPath,
		initPrompt:      "> ",
		bufferPrompt:    "| ",
	}
}

// Loop will run until the user enters "exit", Ctrl+C, Ctrl+D, or an unexpected error occurs.
func (r *REPL) Loop() {

	// Initialize the liner library.
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)
	line.SetMultiLineMode(true)
	r.loadHistory(line)

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

		if r.OneShot(input) {
			fmt.Fprintln(r.output, "Exiting")
			break
		}

		line.AppendHistory(input)
	}

	r.saveHistory(line)
}

// OneShot evaluates a single line and prints the result. Returns true if caller should exit.
func (r *REPL) OneShot(line string) bool {

	if r.init() {
		return true
	}

	if len(r.buffer) == 0 {
		switch strings.TrimSpace(strings.ToLower(line)) {
		case "dump":
			return r.cmdDump()
		case "json":
			return r.cmdFormat("json")
		case "pretty":
			return r.cmdFormat("pretty")
		case "trace":
			return r.cmdTrace()
		case "?":
			fallthrough
		case "help":
			return r.cmdHelp()
		case "quit":
			fallthrough
		case "exit":
			return r.cmdExit()
		}
		r.buffer = append(r.buffer, line)
		return r.evalBufferOne()
	}

	r.buffer = append(r.buffer, line)
	if len(line) == 0 {
		return r.evalBufferMulti()
	}

	return false
}

func (r *REPL) cmdDump() bool {
	fmt.Fprintln(r.output, r.dataStore)
	return false
}

func (r *REPL) cmdExit() bool {
	return true
}

func (r *REPL) cmdFormat(s string) bool {
	r.outputFormat = s
	return false
}

func (r *REPL) cmdHelp() bool {

	commands := []struct {
		name string
		note string
	}{
		{"<stmt>", "evaluate the statement"},
		{"json", "set output format to JSON"},
		{"pretty", "set output format to pretty"},
		{"dump", "dump the raw storage content"},
		{"trace", "toggle stdout tracing"},
		{"help", "print this message (or ?)"},
		{"exit", "exit back to shell (or ctrl+c, ctrl+d, quit)"},
		{"ctrl+l", "clear the screen"},
	}

	maxLength := 0
	for _, command := range commands {
		length := len(command.name)
		if length > maxLength {
			maxLength = length
		}
	}

	for _, command := range commands {
		f := fmt.Sprintf("%%%dv : %%v\n", maxLength)
		fmt.Printf(f, command.name, command.note)
	}

	return false
}

func (r *REPL) cmdTrace() bool {
	r.trace = !r.trace
	return false
}

func (r *REPL) compileBody(body ast.Body) (ast.Body, error) {

	name := fmt.Sprintf("repl%d", r.nextID)
	r.nextID++

	rule := &ast.Rule{
		Name: ast.Var(name),
		Body: body,
	}

	modules := r.policyStore.List()
	mod := modules[r.currentModuleID]
	prev := mod.Rules
	mod.Rules = append(mod.Rules, rule)

	c := ast.NewCompiler()
	if c.Compile(modules); c.Failed() {
		mod.Rules = prev
		return nil, fmt.Errorf(c.FlattenErrors())
	}

	return mod.Rules[len(prev)].Body, nil
}

func (r *REPL) compileRule(rule *ast.Rule) (*ast.Module, error) {

	modules := r.policyStore.List()
	mod := modules[r.currentModuleID]
	prev := mod.Rules
	mod.Rules = append(mod.Rules, rule)

	c := ast.NewCompiler()
	if c.Compile(modules); c.Failed() {
		mod.Rules = prev
		return nil, fmt.Errorf(c.FlattenErrors())
	}

	return mod, nil
}

func (r *REPL) evalBufferOne() bool {

	line := strings.Join(r.buffer, "\n")

	if len(strings.TrimSpace(line)) == 0 {
		r.buffer = []string{}
		return false
	}

	// The user may enter lines with comments on the end or
	// multiple lines with comments interspersed. In these cases
	// the parser will return multiple statements.
	stmts, err := ast.ParseStatements(line)

	if err != nil {
		return false
	}

	r.buffer = []string{}

	for _, stmt := range stmts {
		r.evalStatement(stmt)
	}

	return false
}

func (r *REPL) evalBufferMulti() bool {

	line := strings.Join(r.buffer, "\n")
	r.buffer = []string{}

	if len(strings.TrimSpace(line)) == 0 {
		return false
	}

	stmts, err := ast.ParseStatements(line)

	if err != nil {
		fmt.Fprintln(r.output, "error:", err)
		return false
	}

	for _, stmt := range stmts {
		r.evalStatement(stmt)
	}

	return false
}

func (r *REPL) evalStatement(stmt interface{}) bool {
	switch s := stmt.(type) {
	case ast.Body:
		s, err := r.compileBody(s)
		if err != nil {
			fmt.Fprintln(r.output, "error:", err)
			return false
		}
		return r.evalBody(s)
	case *ast.Rule:
		mod, err := r.compileRule(s)
		if err != nil {
			fmt.Fprintln(r.output, "error:", err)
			return false
		}
		return r.evalModule(mod)
	case *ast.Import:
		return r.evalImport(s)
	case *ast.Package:
		return r.evalPackage(s)
	}
	return false
}

func (r *REPL) evalBody(body ast.Body) bool {

	ctx := topdown.NewContext(body, r.dataStore)
	if r.trace {
		ctx.Tracer = &topdown.StdoutTracer{}
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
			if _, isVar := k.(ast.Var); !isVar {
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

	if err != nil {
		fmt.Fprintf(r.output, "error: %v\n", err)
		return false
	}

	if isTrue {
		if len(results) >= 1 {
			r.printResults(body, results)
		} else {
			fmt.Fprintln(r.output, "true")
		}
	} else {
		fmt.Fprintln(r.output, "false")
	}

	return false
}

func (r *REPL) evalModule(mod *ast.Module) bool {

	err := r.policyStore.Add(r.currentModuleID, mod, nil, false)
	if err != nil {
		fmt.Fprintln(r.output, "error:", err)
		return true
	}

	fmt.Fprintln(r.output, "defined")
	return false
}

func (r *REPL) evalImport(i *ast.Import) bool {

	modules := r.policyStore.List()
	mod := modules[r.currentModuleID]

	for _, x := range mod.Imports {
		if x.Equal(i) {
			return false
		}
	}

	prev := mod.Imports
	mod.Imports = append(mod.Imports, i)

	c := ast.NewCompiler()
	if c.Compile(modules); c.Failed() {
		mod.Imports = prev
		fmt.Fprintln(r.output, "error:", c.FlattenErrors())
		return false
	}

	err := r.policyStore.Add(r.currentModuleID, c.Modules[r.currentModuleID], nil, false)
	if err != nil {
		fmt.Fprint(r.output, "error:", err)
		return true
	}

	return false
}

func (r *REPL) evalPackage(p *ast.Package) bool {

	modules := r.policyStore.List()
	moduleID := p.Path[1:].String()
	if _, ok := modules[moduleID]; ok {
		r.currentModuleID = moduleID
		return false
	}

	err := r.policyStore.Add(moduleID, &ast.Module{Package: p}, nil, false)
	if err != nil {
		fmt.Fprint(r.output, "error:", err)
		return true
	}

	r.currentModuleID = moduleID

	return false
}

func (r *REPL) getPrompt() string {
	if len(r.buffer) > 0 {
		return r.bufferPrompt
	}
	return r.initPrompt
}

func (r *REPL) init() bool {

	if r.initialized {
		return false
	}

	mod := ast.MustParseModule(fmt.Sprintf(`
	package %s
	`, r.currentModuleID))

	modules := r.policyStore.List()
	modules[r.currentModuleID] = mod

	c := ast.NewCompiler()
	if c.Compile(modules); c.Failed() {
		fmt.Fprintln(r.output, "error:", c.FlattenErrors())
		return true
	}

	if err := r.policyStore.Add(r.currentModuleID, c.Modules[r.currentModuleID], nil, false); err != nil {
		fmt.Fprintln(r.output, "error:", err)
		return true
	}

	r.initialized = true

	return false
}

func (r *REPL) loadHistory(prompt *liner.State) {
	if f, err := os.Open(r.historyPath); err == nil {
		prompt.ReadHistory(f)
		f.Close()
	}
}

func (r *REPL) printResults(body ast.Body, results []map[string]interface{}) {

	switch r.outputFormat {
	case "json":
		r.printJSON(results)
	default:
		r.printPretty(body, results)
	}

}

func (r *REPL) printJSON(results []map[string]interface{}) {
	buf, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Fprintln(r.output, err)
		return
	}
	fmt.Fprintln(r.output, string(buf))
}

func (r *REPL) printPretty(body ast.Body, results []map[string]interface{}) {
	table := tablewriter.NewWriter(r.output)
	r.printPrettyHeader(table, body)
	for _, row := range results {
		r.printPrettyRow(table, row)
	}
	table.Render()
}

func (r *REPL) printPrettyHeader(table *tablewriter.Table, body ast.Body) {

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
	table.SetHeader(keys)
}

func (r *REPL) printPrettyRow(table *tablewriter.Table, row map[string]interface{}) {

	// Arrange fields in same order as header.
	keys := []string{}
	for k := range row {
		keys = append(keys, k)
	}

	sort.Strings(keys)

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

func (r *REPL) saveHistory(prompt *liner.State) {
	if f, err := os.Create(r.historyPath); err == nil {
		prompt.WriteHistory(f)
		f.Close()
	}
}

func buildHeader(fields map[string]struct{}, term *ast.Term) {
	switch v := term.Value.(type) {
	case ast.Ref:
		for _, t := range v[1:] {
			buildHeader(fields, t)
		}
	case ast.Var:
		fields[string(v)] = struct{}{}
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
