// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sort"
	"strings"

	"github.com/apcera/termtables"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/eval"
	"github.com/peterh/liner"
)

const (
	replStateInit   = iota
	replStateBuffer = iota
)

// Repl represeents an instance of the interactive shell.
type Repl struct {
	Output       io.Writer
	Trace        bool
	Runtime      *Runtime
	HistoryPath  string
	InitPrompt   string
	BufferPrompt string
	Buffer       []string
}

// NewRepl creates a new Repl.
func NewRepl(rt *Runtime, historyPath string, output io.Writer) *Repl {
	return &Repl{
		Output:       output,
		Trace:        false,
		Runtime:      rt,
		HistoryPath:  historyPath,
		InitPrompt:   "> ",
		BufferPrompt: "| ",
	}
}

// Loop will run until the user enters "exit", Ctrl+C, Ctrl+D, or an unexpected error occurs.
func (r *Repl) Loop() {

	// Initialize the liner library.
	line := liner.NewLiner()
	defer line.Close()
	line.SetCtrlCAborts(true)
	line.SetMultiLineMode(true)
	r.loadHistory(line)

	for true {

		input, err := line.Prompt(r.getPrompt())

		if err == liner.ErrPromptAborted || err == io.EOF {
			fmt.Fprintln(r.Output, "Exiting")
			break
		}

		if err != nil {
			fmt.Fprintln(r.Output, "error (fatal):", err)
			os.Exit(1)
		}

		if r.OneShot(input) {
			fmt.Fprintln(r.Output, "Exiting")
			break
		}

		line.AppendHistory(input)
	}

	r.saveHistory(line)
}

// OneShot evaluates a single line and prints the result. Returns true if caller should exit.
func (r *Repl) OneShot(line string) bool {

	if len(r.Buffer) == 0 {
		switch strings.TrimSpace(strings.ToLower(line)) {
		case "dump":
			return r.cmdDump()
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
		r.Buffer = append(r.Buffer, line)
		return r.evalBufferOne()
	}

	r.Buffer = append(r.Buffer, line)
	if len(line) == 0 {
		return r.evalBufferMulti()
	}

	return false
}

func (r *Repl) cmdDump() bool {
	fmt.Fprintln(r.Output, r.Runtime.Store)
	return false
}

func (r *Repl) cmdExit() bool {
	return false
}

func (r *Repl) cmdHelp() bool {

	commands := []struct {
		name string
		note string
	}{
		{"<stmt>", "evaluate the statement"},
		{"dump", "dump the raw storage content"},
		{"trace", "toggle stdout tracing"},
		{"ctrl+l", "clear the screen"},
		{"help", "print this message (or ?)"},
		{"exit", "exit back to shell (or ctrl+c, ctrl+d, quit)"},
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

func (r *Repl) cmdTrace() bool {
	r.Trace = !r.Trace
	return false
}

func (r *Repl) compileBody(body ast.Body) (ast.Body, error) {
	rule := &ast.Rule{
		Name: ast.Var(randString(32)),
		Body: body,
	}
	// TODO(tsandall): refactor to use current implicit module
	p := ast.Ref{ast.DefaultRootDocument}
	m := &ast.Module{
		Package: &ast.Package{
			Path: p,
		},
		Rules: []*ast.Rule{rule},
	}
	c := ast.NewCompiler()
	c.Compile(map[string]*ast.Module{"tmp": m})
	if len(c.Errors) > 0 {
		return nil, fmt.Errorf(c.FlattenErrors())
	}
	return c.Modules["tmp"].Rules[0].Body, nil
}

func (r *Repl) compileRule(rule *ast.Rule) (*ast.Rule, error) {
	// TODO(tsandall): refactor to use current implicit module
	// TODO(tsandall): refactor to update current implicit module
	p := ast.Ref{ast.DefaultRootDocument}
	m := &ast.Module{
		Package: &ast.Package{
			Path: p,
		},
		Rules: []*ast.Rule{rule},
	}
	c := ast.NewCompiler()
	c.Compile(map[string]*ast.Module{"tmp": m})
	if len(c.Errors) > 0 {
		return nil, fmt.Errorf(c.FlattenErrors())
	}
	return c.Modules["tmp"].Rules[0], nil
}

func (r *Repl) evalBufferOne() bool {

	line := strings.Join(r.Buffer, "\n")

	if len(strings.TrimSpace(line)) == 0 {
		r.Buffer = []string{}
		return false
	}

	// The user may enter lines with comments on the end or
	// multiple lines with comments interspersed. In these cases
	// the parser will return multiple statements.
	stmts, err := ast.ParseStatements(line)

	if err != nil {
		return false
	}

	r.Buffer = []string{}

	for _, stmt := range stmts {
		r.evalStatement(stmt)
	}

	return false
}

func (r *Repl) evalBufferMulti() bool {

	line := strings.Join(r.Buffer, "\n")
	r.Buffer = []string{}

	if len(strings.TrimSpace(line)) == 0 {
		return false
	}

	stmts, err := ast.ParseStatements(line)

	if err != nil {
		fmt.Fprintln(r.Output, "error:", err)
		return false
	}

	for _, stmt := range stmts {
		r.evalStatement(stmt)
	}

	return false
}

func (r *Repl) evalStatement(stmt interface{}) bool {
	switch s := stmt.(type) {
	case ast.Body:
		s, err := r.compileBody(s)
		if err != nil {
			fmt.Fprintln(r.Output, "error:", err)
			return false
		}
		return r.evalBody(s)
	case *ast.Rule:
		s, err := r.compileRule(s)
		if err != nil {
			fmt.Fprintln(r.Output, "error:", err)
			return false
		}
		return r.evalRule(s)
	}
	return false
}

func (r *Repl) evalBody(body ast.Body) bool {

	ctx := eval.NewTopDownContext(body, r.Runtime.Store)
	if r.Trace {
		ctx.Tracer = &eval.StdoutTracer{}
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
	err := eval.TopDown(ctx, func(ctx *eval.TopDownContext) error {
		var err error
		row := map[string]interface{}{}
		ctx.Bindings.Iter(func(k, v ast.Value) bool {
			if _, isVar := k.(ast.Var); !isVar {
				return false
			}
			r, e := eval.ValueToInterface(v, ctx)
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
		fmt.Fprintf(r.Output, "error: %v\n", err)
		return false
	}

	// Print results.
	if isTrue {
		if len(results) >= 1 {
			r.printResults(body, results)
		} else {
			fmt.Fprintln(r.Output, "true")
		}
	} else {
		fmt.Fprintln(r.Output, "false")
	}

	return false
}

func (r *Repl) printResults(body ast.Body, results []map[string]interface{}) {
	table := termtables.CreateTable()
	r.printHeader(table, body)
	for _, row := range results {
		r.printRow(table, row)
	}
	fmt.Fprintf(r.Output, table.Render())
}

func (r *Repl) printHeader(table *termtables.Table, body ast.Body) {

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

	// Store fields by name.
	keys := []string{}
	for k := range fields {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	s := []interface{}{}
	for _, k := range keys {
		s = append(s, k)
	}

	// Add fields to table in sorted order.
	table.AddHeaders(s...)
}

func (r *Repl) printRow(table *termtables.Table, row map[string]interface{}) {

	// Arrange fields in same order as header.
	keys := []string{}
	for k := range row {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	buf := []interface{}{}
	for _, k := range keys {
		js, err := json.Marshal(row[k])
		if err != nil {
			buf = append(buf, err)
		} else {
			buf = append(buf, string(js))
		}
	}

	// Add fields to table in sorted order.
	table.AddRow(buf...)
}

func (r *Repl) evalRule(rule *ast.Rule) bool {

	path := []interface{}{string(rule.Name)}

	if err := r.Runtime.Store.Patch(eval.StorageAdd, path, []*ast.Rule{rule}); err != nil {
		fmt.Fprintln(r.Output, "error:", err)
		return true
	}

	fmt.Fprintln(r.Output, "defined")
	return false
}

func (r *Repl) getPrompt() string {
	if len(r.Buffer) > 0 {
		return r.BufferPrompt
	}
	return r.InitPrompt
}

func (r *Repl) loadHistory(prompt *liner.State) {
	if f, err := os.Open(r.HistoryPath); err == nil {
		prompt.ReadHistory(f)
		f.Close()
	}
}

func (r *Repl) saveHistory(prompt *liner.State) {
	if f, err := os.Create(r.HistoryPath); err == nil {
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

// randString returns a random string of letters.
// http://stackoverflow.com/a/31832326
func randString(length int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	s := make([]rune, length)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
