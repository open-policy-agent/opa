// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package repl implements a Read-Eval-Print-Loop (REPL) for interacting with the policy engine.
//
// The REPL is typically used from the command line, however, it can also be used as a library.
// nolint: goconst // String reuse here doesn't make sense to deduplicate.
package repl

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/reeflective/readline"
	"golang.org/x/term"

	"github.com/open-policy-agent/opa/internal/future"
	pr "github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/compile"
	"github.com/open-policy-agent/opa/v1/format"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/profiler"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/lineage"
	"github.com/open-policy-agent/opa/v1/version"
)

// REPL represents an instance of the interactive shell.
type REPL struct {
	output  io.Writer
	stderr  io.Writer
	input   io.Reader
	store   storage.Store
	runtime *ast.Term

	modules             map[string]*ast.Module
	currentModuleID     string
	buffer              []string
	txn                 storage.Transaction
	metrics             metrics.Metrics
	profiler            bool
	strictBuiltinErrors bool
	capabilities        *ast.Capabilities
	regoVersion         ast.RegoVersion
	initBundles         map[string]*bundle.Bundle

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
	report            [][2]string
	target            string // target type (wasm, rego, etc.)
	history           *replHistory
	mtx               sync.Mutex
}

type explainMode string

const (
	explainOff   explainMode = "off"
	explainFull  explainMode = "full"
	explainNotes explainMode = "notes"
	explainFails explainMode = "fails"
	explainDebug explainMode = "debug"
)

func parseExplainMode(str string) (explainMode, error) {
	validExplainModes := []string{
		string(explainOff),
		string(explainFull),
		string(explainNotes),
		string(explainFails),
		string(explainDebug),
	}

	for _, mode := range validExplainModes {
		if mode == str {
			return explainMode(mode), nil
		}
	}

	return "", fmt.Errorf("invalid explain mode, expected one of: %s", strings.Join(validExplainModes, ", "))
}

const defaultPrettyLimit = 80

var allowedTargets = map[string]bool{compile.TargetRego: true, compile.TargetWasm: true}

const exitPromptMessage = "Do you want to exit ([y]/n)? "

// New returns a new instance of the REPL.
func New(store storage.Store, historyPath string, output io.Writer, outputFormat string, errLimit int, banner string) *REPL {

	return &REPL{
		output:       output,
		input:        os.Stdin,
		store:        store,
		modules:      map[string]*ast.Module{},
		capabilities: ast.CapabilitiesForThisVersion(),
		outputFormat: outputFormat,
		explain:      explainOff,
		historyPath:  historyPath,
		initPrompt:   "> ",
		bufferPrompt: "| ",
		banner:       banner,
		errLimit:     errLimit,
		prettyLimit:  defaultPrettyLimit,
		target:       "",
		regoVersion:  ast.DefaultRegoVersion,
	}
}

func (r *REPL) WithCapabilities(capabilities *ast.Capabilities) *REPL {
	r.capabilities = capabilities
	return r
}

func (r *REPL) WithInitBundles(b map[string]*bundle.Bundle) *REPL {
	r.initBundles = b
	return r
}

func defaultModule() *ast.Module {
	return ast.MustParseModule(`package repl`)
}

func defaultPackage() *ast.Package {
	return ast.MustParsePackage(`package repl`)
}

func (r *REPL) getCurrentOrDefaultModule() *ast.Module {
	if r.currentModuleID == "" {
		return defaultModule()
	}
	return r.modules[r.currentModuleID]
}

func (r *REPL) initModule(ctx context.Context) error {
	if r.currentModuleID != "" {
		return nil
	}
	return r.evalStatement(ctx, defaultPackage())
}

func (r *REPL) WithStderrWriter(w io.Writer) *REPL {
	r.stderr = w
	return r
}

// WithConsoleInput sets the reader the REPL reads query input from. It defaults
// to os.Stdin; a nil reader is ignored. Non-terminal input (a pipe, a file,
// /dev/null) uses a plain line reader instead of the terminal editor.
func (r *REPL) WithConsoleInput(in io.Reader) *REPL {
	if in != nil {
		r.input = in
	}
	return r
}

// newShell initializes the readline line-reader used by Loop. Bracketed paste
// is enabled so that pasted tabs/newlines are inserted literally instead of
// triggering tab-completion or submitting the line (issue #962).
func (r *REPL) newShell() *readline.Shell {
	line := readline.NewShell()
	// Enabling bracketed paste is the fix for #962; surface a warning if the
	// config key ever stops being honored so the bug can't silently return.
	if err := line.Config.Set("enable-bracketed-paste", true); err != nil {
		fmt.Fprintln(r.stderrWriter(), "warning: failed to enable bracketed paste:", err)
	}
	line.Prompt.Primary(r.getPrompt)
	line.Completer = r.complete
	r.loadHistory(line)
	return line
}

// Loop reads, evaluates, and prints query results until the input is exhausted
// (EOF), the user exits, or an unexpected error occurs.
//
// An interactive terminal gets the full readline editor (history, completion,
// bracketed paste); any non-terminal input (a pipe, a file, /dev/null) gets a
// plain line reader that stops cleanly at EOF. The readline editor must not be
// driven against a non-TTY: on macOS it busy-loops at 100% CPU without ever
// seeing EOF.
func (r *REPL) Loop(ctx context.Context) error {
	if r.inputIsTerminal() {
		return r.loopInteractive(ctx)
	}
	return r.loopPiped(ctx)
}

// inputIsTerminal reports whether the configured input is an interactive
// terminal. Any non-*os.File reader (e.g. a pipe used in tests) is not.
func (r *REPL) inputIsTerminal() bool {
	f, ok := r.input.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// maxConsoleLineBytes bounds a single line read from non-interactive input.
const maxConsoleLineBytes = 1024 * 1024

// loopPiped reads and evaluates input line-by-line, returning at EOF.
func (r *REPL) loopPiped(ctx context.Context) error {
	if len(r.banner) > 0 {
		fmt.Fprintln(r.output, r.banner)
	}

	scanner := bufio.NewScanner(r.input)
	scanner.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), maxConsoleLineBytes)

	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := r.OneShot(ctx, scanner.Text()); err != nil {
			switch err.(type) {
			case stop:
				return nil
			default:
				fmt.Fprintln(r.output, err)
			}
		}
	}

	return scanner.Err()
}

// loopInteractive will run until the user enters "exit", Ctrl+C, Ctrl+D, or an unexpected error occurs.
func (r *REPL) loopInteractive(ctx context.Context) error {

	line := r.newShell()

	if len(r.banner) > 0 {
		fmt.Fprintln(r.output, r.banner)
	}

loop:
	// Restoring the prompt (and resuming history persistence) here means every
	// path back into the main loop is covered by a single reset, so a future
	// exit path can't forget to undo the exit-prompt overrides below.
	if r.history != nil {
		r.history.resume()
	}
	line.Prompt.Primary(r.getPrompt)
	for {

		input, err := line.Readline()

		// prompt on ctrl+d
		if err == io.EOF {
			goto exitPrompt
		}

		// reset on ctrl+c
		if errors.Is(err, readline.ErrInterrupt) {
			continue
		}

		// exit on unknown error
		if err != nil {
			fmt.Fprintln(r.output, "error (fatal):", err)
			return err
		}

		if err := r.OneShot(ctx, input); err != nil {
			switch err := err.(type) {
			case stop:
				goto exit
			default:
				fmt.Fprintln(r.output, err)
			}
		}
	}

exitPrompt:
	fmt.Fprintln(r.output)

	// The exit-prompt confirmation ("y"/"n") is control input, not a query, so
	// pause persistence while it is on screen. The loop: label resumes it.
	if r.history != nil {
		r.history.pause()
	}
	line.Prompt.Primary(func() string { return exitPromptMessage })

	for {
		input, err := line.Readline()

		// exit on ctrl+d
		if err == io.EOF {
			break
		}

		// reset on ctrl+c
		if errors.Is(err, readline.ErrInterrupt) {
			goto loop
		}

		// exit on unknown error
		if err != nil {
			fmt.Fprintln(r.output, "error (fatal):", err)
			return err
		}

		switch strings.ToLower(input) {
		case "", "y", "yes":
			goto exit
		case "n", "no":
			goto loop
		}
	}

exit:
	return nil
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
				return r.cmdShow(cmd.args)
			case "unset":
				return r.cmdUnset(ctx, cmd.args)
			case "unset-package":
				return r.cmdUnsetPackage(ctx, cmd.args)
			case "pretty":
				return r.cmdFormat("pretty")
			case "pretty-limit":
				return r.cmdPrettyLimit(cmd.args)
			case "trace":
				// If an argument is specified, e.g. `trace notes`, parse that
				// argument and toggle that specific mode.  If no argument is
				// specified, toggle full explain mode since that is backwards-
				// compatible.
				if len(cmd.args) == 1 {
					explainMode, err := parseExplainMode(cmd.args[0])
					if err != nil {
						return err
					}
					return r.cmdTrace(explainMode)
				}
				return r.cmdTrace(explainFull)
			case "notes":
				return r.cmdTrace(explainNotes)
			case "fails":
				return r.cmdTrace(explainFails)
			case "metrics":
				return r.cmdMetrics()
			case "instrument":
				return r.cmdInstrument()
			case "profile":
				return r.cmdProfile()
			case "types":
				return r.cmdTypes()
			case "unknown":
				return r.cmdUnknown(cmd.args)
			case "strict-builtin-errors":
				return r.cmdStrictBuiltinErrors()
			case "target":
				return r.cmdTarget(cmd.args)
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

// WithRuntime sets the runtime data to provide to the evaluation engine.
func (r *REPL) WithRuntime(term *ast.Term) *REPL {
	r.runtime = term
	return r
}

// WithRegoVersion sets the Rego version to v.
func (r *REPL) WithRegoVersion(v ast.RegoVersion) *REPL {
	r.regoVersion = v
	return r
}

// WithV1Compatible sets the Rego version to v1.
//
// Deprecated: Use WithRegoVersion instead.
func (r *REPL) WithV1Compatible(v1Compatible bool) *REPL {
	if v1Compatible {
		r.regoVersion = ast.RegoV1
	} else {
		r.regoVersion = ast.DefaultRegoVersion
	}
	return r
}

// SetOPAVersionReport sets the information about the latest OPA release.
func (r *REPL) SetOPAVersionReport(report [][2]string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.report = report
}

// complete adapts completeCandidates to the readline completer signature.
// Only the text up to the cursor is used for prefix matching, matching the
// behaviour of the previous line-reader.
func (r *REPL) complete(line []rune, cursor int) readline.Completions {
	return readline.CompleteValues(r.completeCandidates(string(line[:cursor]))...)
}

func (r *REPL) completeCandidates(line string) []string {
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
		for _, imp := range future.FilterFutureImports(mod.Imports) {
			path := imp.Name().String()
			if strings.HasPrefix(path, line) {
				set[path] = struct{}{}
			}
		}
	}

	// add virtual docs defined in repl
	for _, mod := range r.modules {
		for _, rule := range mod.Rules {
			path := ast.RulePath(rule)
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
			if path := ast.RulePath(rule); strings.HasPrefix(path, line) {
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

func (*REPL) cmdExit() error {
	return stop{}
}

func (r *REPL) cmdFormat(s string) error {
	r.outputFormat = s
	return nil
}

func (r *REPL) cmdTarget(t []string) error {
	if len(t) != 1 {
		return newBadArgsErr("target <mode>: expects exactly one argument")
	}

	if _, ok := allowedTargets[t[0]]; !ok {
		return fmt.Errorf("invalid target \"%v\":must be one of {rego,wasm}", t[0])
	}

	r.target = t[0]

	r.checkTraceSupported()
	return nil
}

func (r *REPL) cmdPrettyLimit(s []string) error {
	if len(s) != 1 {
		return errors.New("usage: pretty-limit <n>")
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
		printHelp(r.output, r.initPrompt, r.report)
	} else {
		if desc, ok := topics[args[0]]; ok {
			return desc.fn(r.output)
		}
		return fmt.Errorf("unknown topic '%v'", args[0])
	}
	return nil
}

func (r *REPL) cmdShow(args []string) error {

	if len(args) == 0 {
		if r.currentModuleID == "" {
			fmt.Fprintln(r.output, "no rules defined")
			return nil
		}
		module := r.modules[r.currentModuleID]
		bs, err := format.AstWithOpts(module, format.Opts{RegoVersion: module.RegoVersion()})
		if err != nil {
			return err
		}
		fmt.Fprint(r.output, string(bs))
		return nil
	} else if args[0] == "debug" {
		debug := replDebugState{
			Explain:             r.explain,
			Metrics:             r.metricsEnabled(),
			Instrument:          r.instrument,
			Profile:             r.profilerEnabled(),
			StrictBuiltinErrors: r.strictBuiltinErrors,
		}
		b, err := json.MarshalIndent(debug, "", "\t")
		if err != nil {
			return fmt.Errorf("error: %v", err)
		}
		fmt.Fprintln(r.output, string(b))
		return nil
	}
	return fmt.Errorf("unknown option '%v'", args[0])
}

type replDebugState struct {
	Explain             explainMode `json:"explain"`
	Metrics             bool        `json:"metrics"`
	Instrument          bool        `json:"instrument"`
	Profile             bool        `json:"profile"`
	StrictBuiltinErrors bool        `json:"strict-builtin-errors"`
}

func (r *REPL) cmdTrace(mode explainMode) error {
	if r.explain == mode {
		r.explain = explainOff
	} else {
		r.explain = mode
	}

	r.checkTraceSupported()
	return nil
}

func (r *REPL) checkTraceSupported() {
	if r.explain != explainOff && r.target == compile.TargetWasm {
		fmt.Fprintf(r.output, "warning: trace mode \"%v\" is not supported with wasm target\n", r.explain)
	}
}

func (r *REPL) metricsEnabled() bool {
	return r.metrics != nil
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

func (r *REPL) profilerEnabled() bool {
	return r.profiler
}

func (r *REPL) cmdProfile() error {
	if r.profiler {
		r.profiler = false
	} else {
		r.profiler = true
	}
	return nil
}

func (r *REPL) cmdStrictBuiltinErrors() error {
	r.strictBuiltinErrors = !r.strictBuiltinErrors
	return nil
}

func (r *REPL) cmdTypes() error {
	r.types = !r.types
	return nil
}

var errUnknownUsage = errors.New("usage: unknown <input/data reference> [<input/data reference> [...]] (hint: try 'input')")

func (r *REPL) cmdUnknown(s []string) error {

	if len(s) == 0 && len(r.unknowns) == 0 {
		return errUnknownUsage
	}

	unknowns := make([]*ast.Term, len(s))

	for i := range unknowns {

		ref, err := ast.ParseRef(s[i])
		if err != nil {
			return errUnknownUsage
		}

		unknowns[i] = ast.NewTerm(ref)
	}

	r.unknowns = unknowns
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

func (r *REPL) cmdUnsetPackage(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return newBadArgsErr("unset-package <var>: expects exactly one argument")
	}

	pkg, err := ast.ParsePackage("package " + args[0])
	if err != nil {
		return newBadArgsErr("argument must identify a package")
	}

	unset, err := r.unsetPackage(ctx, pkg)
	if err != nil {
		return err
	} else if !unset {
		fmt.Fprintln(r.output, "warning: no matching package")
	}

	return nil
}

func (r *REPL) unsetRule(ctx context.Context, name ast.Var) (bool, error) {
	if r.currentModuleID == "" {
		return false, nil
	}

	mod := r.modules[r.currentModuleID]
	rules := []*ast.Rule{}

	for _, r := range mod.Rules {
		if r.Head.Name != name {
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

func (r *REPL) unsetPackage(_ context.Context, pkg *ast.Package) (bool, error) {
	path := pkg.Path.String()
	_, ok := r.modules[path]
	if ok {
		delete(r.modules, path)
	} else {
		return false, nil
	}

	// Change back to default module if current one is being removed
	if r.currentModuleID == path {
		r.currentModuleID = ""
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

	compiler := ast.NewCompiler().
		SetErrorLimit(r.errLimit).
		WithEnablePrintStatements(true).
		WithCapabilities(r.capabilities)

	if r.instrument {
		compiler.WithMetrics(r.metrics)
	}

	if compiler.Compile(policies); compiler.Failed() {
		return compiler.Errors
	}

	r.modules[r.currentModuleID] = cpy
	return nil
}

func (r *REPL) compileBody(_ context.Context, compiler *ast.Compiler, body ast.Body) (ast.Body, *ast.TypeEnv, error) {
	r.timerStart(metrics.RegoQueryCompile)
	defer r.timerStop(metrics.RegoQueryCompile)

	qctx := ast.NewQueryContext()

	if r.currentModuleID != "" {
		qctx = qctx.WithPackage(r.modules[r.currentModuleID].Package).
			WithImports(future.FilterFutureImports(r.modules[r.currentModuleID].Imports))
	}

	qc := compiler.QueryCompiler().WithContext(qctx).WithEnablePrintStatements(true)
	body, err := qc.Compile(body)
	return body, qc.TypeEnv(), err
}

func (r *REPL) compileRule(ctx context.Context, rule *ast.Rule) error {

	var unset bool

	if r.regoVersion == ast.RegoV1 {
		if errs := ast.CheckRegoV1(rule); errs != nil {
			return errs
		}
	}

	if rule.Head.Assign {
		var err error
		unset, err = r.unsetRule(ctx, rule.Head.Name)
		if err != nil {
			return err
		}
	}

	r.timerStart(metrics.RegoModuleCompile)
	defer r.timerStop(metrics.RegoModuleCompile)

	if err := r.initModule(ctx); err != nil {
		return err
	}

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

	maps.Copy(policies, r.modules)

	compiler := ast.NewCompiler().
		SetErrorLimit(r.errLimit).
		WithEnablePrintStatements(true).
		WithCapabilities(r.capabilities)

	if r.instrument {
		compiler.WithMetrics(r.metrics)
	}

	if compiler.Compile(policies); compiler.Failed() {
		mod.Rules = prev
		return compiler.Errors
	}

	switch r.outputFormat {
	case "json":
	default:
		msg := "defined"
		if unset {
			msg = "re-defined"
		}
		fmt.Fprintf(r.output, "Rule '%v' %v in %v. Type 'show' to see rules.\n", rule.Head.Ref().String(), msg, mod.Package)
	}

	return nil
}

func (r *REPL) evalBufferOne(ctx context.Context) error {

	line := strings.Join(r.buffer, "\n")

	if len(strings.TrimSpace(line)) == 0 {
		r.buffer = []string{}
		return nil
	}

	popts, err := r.parserOptions()
	if err != nil {
		return err
	}

	// The user may enter lines with comments on the end or
	// multiple lines with comments interspersed. In these cases
	// the parser will return multiple statements.
	r.timerStart(metrics.RegoQueryParse)
	stmts, _, err := ast.ParseStatementsWithOpts("", line, popts)
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

	popts, err := r.parserOptions()
	if err != nil {
		return err
	}

	r.timerStart(metrics.RegoQueryParse)
	stmts, _, err := ast.ParseStatementsWithOpts("", line, popts)
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

func (r *REPL) parserOptions() (ast.ParserOptions, error) {
	if r.regoVersion == ast.RegoV1 {
		return ast.ParserOptions{RegoVersion: ast.RegoV1}, nil
	}
	if r.currentModuleID != "" {
		opts, err := future.ParserOptionsFromFutureImports(r.modules[r.currentModuleID].Imports)
		if err == nil {
			for _, i := range r.modules[r.currentModuleID].Imports {
				if ast.RegoV1CompatibleRef.Equal(i.Path.Value) {
					opts.RegoVersion = ast.RegoV1
				}
			}
		}
		return opts, err
	}
	return ast.ParserOptions{RegoVersion: r.regoVersion}, nil
}

func (r *REPL) loadCompiler(ctx context.Context) (*ast.Compiler, error) {

	r.timerStart(metrics.RegoModuleCompile)
	defer r.timerStop(metrics.RegoModuleCompile)

	policies, err := r.loadModules(ctx, r.txn)
	if err != nil {
		return nil, err
	}

	maps.Copy(policies, r.modules)

	compiler := ast.NewCompiler().
		SetErrorLimit(r.errLimit).
		WithEnablePrintStatements(true).
		WithCapabilities(r.capabilities)

	if r.instrument {
		compiler.WithMetrics(r.metrics)
	}

	if compiler.Compile(policies); compiler.Failed() {
		return nil, compiler.Errors
	}

	return compiler, nil
}

// loadInput returns the input defined in the REPL. The REPL loads the
// input from the data.repl.input document.
func (r *REPL) loadInput(ctx context.Context, compiler *ast.Compiler) (ast.Value, error) {

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

func (r *REPL) evalStatement(ctx context.Context, stmt any) error {
	switch stmt := stmt.(type) {
	case ast.Body:
		compiler, err := r.loadCompiler(ctx)
		if err != nil {
			return err
		}

		input, err := r.loadInput(ctx, compiler)
		if err != nil {
			return err
		}

		if ok, err := r.interpretAsRule(ctx, compiler, stmt); ok || err != nil {
			return err
		}

		compiledBody, typeEnv, err := r.compileBody(ctx, compiler, stmt)
		if err != nil {
			return err
		}

		if len(r.unknowns) > 0 {
			err = r.evalPartial(ctx, compiler, input, compiledBody)
		} else {
			err = r.evalBody(ctx, compiler, input, stmt)
			if r.types {
				r.printTypes(ctx, typeEnv, compiledBody)
			}
		}

		return err
	case *ast.Rule:
		return r.compileRule(ctx, stmt)
	case *ast.Import:
		return r.evalImport(ctx, stmt)
	case *ast.Package:
		return r.evalPackage(stmt)
	}
	return nil
}

func (r *REPL) evalBody(ctx context.Context, compiler *ast.Compiler, input ast.Value, body ast.Body) error {

	var tracebuf *topdown.BufferTracer
	var prof *profiler.Profiler

	args := []func(*rego.Rego){
		rego.Compiler(compiler),
		rego.Store(r.store),
		rego.Transaction(r.txn),
		rego.ParsedImports(r.getCurrentOrDefaultModule().Imports),
		rego.ParsedPackage(r.getCurrentOrDefaultModule().Package),
		rego.ParsedQuery(body),
		rego.ParsedInput(input),
		rego.Metrics(r.metrics),
		rego.Instrument(r.instrument),
		rego.Runtime(r.runtime),
		rego.StrictBuiltinErrors(r.strictBuiltinErrors),
		rego.Target(r.target),
		rego.EnablePrintStatements(true),
		rego.PrintHook(topdown.NewPrintHook(r.stderrWriter())),
	}

	if r.explain != explainOff {
		tracebuf = topdown.NewBufferTracer()
		args = append(args, rego.QueryTracer(tracebuf))
	}

	if r.profiler {
		prof = profiler.New()
		args = append(args, rego.QueryTracer(prof))
	}

	eval := rego.New(args...)
	rs, err := eval.Eval(ctx)

	output := pr.Output{
		Errors:  pr.NewOutputErrors(err),
		Result:  rs,
		Metrics: r.metrics,
	}

	if r.profiler {
		output.Profile = prof.ReportTopNResults(-1, pr.DefaultProfileSortOrder)
	}

	output = output.WithLimit(r.prettyLimit)

	switch r.explain {
	case explainDebug:
		output.Explanation = lineage.Debug(*tracebuf)
	case explainFull:
		output.Explanation = lineage.Full(*tracebuf)
	case explainNotes:
		output.Explanation = lineage.Notes(*tracebuf)
	case explainFails:
		output.Explanation = lineage.Fails(*tracebuf)
	}

	switch r.outputFormat {
	case "json":
		return pr.JSON(r.output, output)
	default:
		return pr.Pretty(r.output, r.stderr, output)
	}
}

func (r *REPL) evalPartial(ctx context.Context, compiler *ast.Compiler, input ast.Value, body ast.Body) error {

	var buf *topdown.BufferTracer

	if r.explain != explainOff {
		buf = topdown.NewBufferTracer()
	}

	eval := rego.New(
		rego.Compiler(compiler),
		rego.Store(r.store),
		rego.Transaction(r.txn),
		rego.ParsedImports(r.getCurrentOrDefaultModule().Imports),
		rego.ParsedPackage(r.getCurrentOrDefaultModule().Package),
		rego.ParsedQuery(body),
		rego.ParsedInput(input),
		rego.Metrics(r.metrics),
		rego.QueryTracer(buf),
		rego.Instrument(r.instrument),
		rego.ParsedUnknowns(r.unknowns),
		rego.Runtime(r.runtime),
		rego.StrictBuiltinErrors(r.strictBuiltinErrors),
		rego.EnablePrintStatements(true),
		rego.PrintHook(topdown.NewPrintHook(r.stderrWriter())),
	)

	pq, err := eval.Partial(ctx)

	output := pr.Output{
		Metrics: r.metrics,
		Partial: pq,
		Errors:  pr.NewOutputErrors(err),
	}

	switch r.explain {
	case explainDebug:
		output.Explanation = lineage.Debug(*buf)
	case explainFull:
		output.Explanation = lineage.Full(*buf)
	case explainNotes:
		output.Explanation = lineage.Notes(*buf)
	case explainFails:
		output.Explanation = lineage.Fails(*buf)
	}

	switch r.outputFormat {
	case "json":
		return pr.JSON(r.output, output)
	default:
		return pr.Pretty(r.output, r.stderr, output)
	}
}

func (r *REPL) evalImport(ctx context.Context, i *ast.Import) error {

	if err := r.initModule(ctx); err != nil {
		return err
	}

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

	m := ast.Module{
		Package: p,
	}
	m.SetRegoVersion(r.regoVersion)
	r.modules[moduleID] = &m

	r.currentModuleID = moduleID

	return nil
}

// interpretAsRule attempts to interpret the supplied query as a rule
// definition. If the query is a single := or = statement and it can be
// converted into a rule and compiled, then it will be interpreted as such. This
// allows users to define constants in the REPL. For example:
//
//		> a = 1
//	 > a
//	 1
//
// If the expression is a = statement, then an additional check on the left
// hand side occurs. For example:
//
//		> b = 2
//	 > b = 2
//	 true      # not redefined!
func (r *REPL) interpretAsRule(ctx context.Context, compiler *ast.Compiler, body ast.Body) (bool, error) {
	if len(body) != 1 {
		return false, nil
	}

	expr := body[0]

	if !expr.IsAssignment() && !expr.IsEquality() {
		return false, nil
	}

	if len(expr.Operands()) != 2 {
		return false, nil
	}

	if expr.IsEquality() && isGlobalInModule(compiler, r.getCurrentOrDefaultModule(), expr.Operand(0)) {
		return false, nil
	}

	rule, err := ast.ParseRuleFromExpr(r.getCurrentOrDefaultModule(), expr)
	if rule == nil || err != nil {
		return false, err
	}
	// TODO(sr): support interactive ref head rule definitions
	if len(rule.Head.Ref()) > 1 {
		return false, nil
	}

	if err := r.compileRule(ctx, rule); err != nil {
		return false, err
	}

	return true, nil
}

func (r *REPL) getPrompt() string {
	if len(r.buffer) > 0 {
		return r.bufferPrompt
	}
	return r.initPrompt
}

func (r *REPL) loadHistory(line *readline.Shell) {
	if r.historyPath == "" {
		return
	}
	h, err := newREPLHistory(r.historyPath)
	if err != nil {
		// Non-fatal: newREPLHistory still returns a usable (empty, writable)
		// source, so the session continues without prior history.
		fmt.Fprintln(r.stderrWriter(), "warning: failed to load REPL history:", err)
	}
	r.history = h
	line.History.Add("repl", h)
}

// replHistoryItem is one persisted history entry. The JSON shape matches the
// file format used by reeflective/readline so the two readers stay
// interchangeable.
type replHistoryItem struct {
	DateTime time.Time `json:"datetime"`
	Block    string    `json:"block"`
}

// replHistory is a file-backed readline history source tailored to the REPL:
//
//   - Legacy plain-text history files written by the previous line-reader
//     (peterh/liner, one command per line) are migrated in place to the
//     JSON-lines format on load, so prior history survives the upgrade instead
//     of being silently dropped (see the #8882 review).
//   - REPL control input is not persisted: the "exit" command is always
//     skipped, and persistence is paused around the Ctrl+D exit prompt so its
//     confirmation answer is not recorded. This matches the previous reader,
//     which only appended a line once it had been evaluated as a query.
type replHistory struct {
	path    string
	items   []replHistoryItem
	persist bool
}

// newREPLHistory loads (migrating if necessary) the history file at path. A
// load error is returned but is not fatal: the returned source is always
// usable, falling back to empty in-memory history that still accepts writes.
func newREPLHistory(path string) (*replHistory, error) {
	h := &replHistory{path: path, persist: true}
	migrated, err := h.load()
	if err != nil {
		return h, err
	}
	if migrated {
		// Rewrite once so legacy plain-text entries are stored in the new
		// format; subsequent accepted lines are simply appended.
		return h, h.flush()
	}
	return h, nil
}

// load reads the history file into memory, reporting whether any legacy
// plain-text lines were encountered (and thus whether the file must be
// rewritten). A missing file is not an error.
func (h *replHistory) load() (migrated bool, err error) {
	f, err := os.Open(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		var item replHistoryItem
		if err := json.Unmarshal([]byte(line), &item); err == nil && item.Block != "" {
			h.items = append(h.items, item)
			continue
		}

		// Not JSON: a legacy one-command-per-line entry. Preserve it and flag
		// the file for rewriting into the new format.
		if block := strings.TrimSpace(line); block != "" {
			h.items = append(h.items, replHistoryItem{Block: block})
			migrated = true
		}
	}

	return migrated, scanner.Err()
}

// flush rewrites the entire history file from the in-memory items.
func (h *replHistory) flush() error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for i := range h.items {
		if err := enc.Encode(h.items[i]); err != nil {
			return err
		}
	}
	return os.WriteFile(h.path, buf.Bytes(), 0o600)
}

// pause and resume toggle whether accepted lines are persisted; they keep the
// exit-prompt confirmation out of the history.
func (h *replHistory) pause()  { h.persist = false }
func (h *replHistory) resume() { h.persist = true }

// Write implements the readline history source interface. readline calls it for
// every accepted line, before the REPL inspects the returned input.
func (h *replHistory) Write(line string) (int, error) {
	block := strings.TrimSpace(line)
	if !h.persist || block == "" {
		return len(h.items), nil
	}

	// Never persist the exit command: it is control input, not a query.
	if c := newCommand(block); c != nil && c.op == "exit" {
		return len(h.items), nil
	}

	// Skip consecutive duplicates, mirroring readline's own file source.
	if n := len(h.items); n > 0 && h.items[n-1].Block == block {
		return n, nil
	}

	item := replHistoryItem{DateTime: time.Now(), Block: block}
	h.items = append(h.items, item)

	if h.path == "" {
		return len(h.items), nil
	}

	data, err := json.Marshal(item)
	if err != nil {
		return len(h.items), err
	}

	f, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return len(h.items), err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return len(h.items), err
}

// GetLine implements the readline history source interface.
func (h *replHistory) GetLine(pos int) (string, error) {
	if pos < 0 || pos >= len(h.items) {
		return "", fmt.Errorf("history index %d out of range", pos)
	}
	return h.items[pos].Block, nil
}

// Len implements the readline history source interface.
func (h *replHistory) Len() int { return len(h.items) }

// Dump implements the readline history source interface.
func (h *replHistory) Dump() any { return h.items }

func (r *REPL) loadModules(ctx context.Context, txn storage.Transaction) (map[string]*ast.Module, error) {
	modules := make(map[string]*ast.Module)

	if len(r.initBundles) > 0 {
		for bundleName, b := range r.initBundles {
			maps.Copy(modules, b.ParsedModules(bundleName))
		}
	}

	ids, err := r.store.ListPolicies(ctx, txn)
	if err != nil {
		return nil, err
	}

	for _, id := range ids {
		// skip re-parsing
		if _, haveMod := modules[id]; haveMod {
			continue
		}

		bs, err := r.store.GetPolicy(ctx, txn, id)
		if err != nil {
			return nil, err
		}

		popts := ast.ParserOptions{
			RegoVersion: r.regoVersion,
		}

		parsed, err := ast.ParseModuleWithOpts(id, string(bs), popts)
		if err != nil {
			return nil, err
		}

		modules[id] = parsed
	}

	return modules, nil
}

func (r *REPL) printTypes(_ context.Context, typeEnv *ast.TypeEnv, body ast.Body) {
	ast.WalkRefs(body, func(ref ast.Ref) bool {
		fmt.Fprintf(r.output, "# %v: %v\n", ref, typeEnv.GetByRef(ref))
		return false
	})

	vis := ast.NewVarVisitor().WithParams(ast.VarVisitorParams{
		SkipRefHead: true,
	})

	vis.Walk(body)

	for v := range vis.Vars() {
		fmt.Fprintf(r.output, "# %v: %v\n", v, typeEnv.GetByValue(v))
	}
}

func (r *REPL) stderrWriter() io.Writer {
	if r.stderr != nil {
		return r.stderr
	}
	return os.Stderr
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
	{"data.system.version", "drill into specific document"},
}

var extra = [...]commandDesc{
	{"<stmt>", []string{}, "evaluate the statement"},
	{"package", []string{"<term>"}, "change active package"},
	{"import", []string{"<term>"}, "add import to active module"},
}

var builtin = [...]commandDesc{
	{"show", []string{""}, "show active module definition"},
	{"show debug", []string{""}, "show REPL settings"},
	{"unset", []string{"<var>"}, "unset rules in currently active module"},
	{"unset-package", []string{"<var>"}, "unset packages in currently active module"},
	{"json", []string{}, "set output format to JSON"},
	{"pretty", []string{}, "set output format to pretty"},
	{"pretty-limit", []string{}, "set pretty value output limit"},
	{"trace", []string{"[mode]"}, "toggle full trace or specific mode"},
	{"notes", []string{}, "toggle notes trace"},
	{"fails", []string{}, "toggle fails trace"},
	{"metrics", []string{}, "toggle metrics"},
	{"instrument", []string{}, "toggle instrumentation"},
	{"profile", []string{}, "toggle profiler and turns off trace"},
	{"types", []string{}, "toggle type information"},
	{"unknown", []string{"[ref-1 [ref-2 [...]]]"}, "toggle partial evaluation mode"},
	{"strict-builtin-errors", []string{}, "toggle strict built-in error mode"},
	{"dump", []string{"[path]"}, "dump raw data in storage"},
	{"help", []string{"[topic]"}, "print this message"},
	{"target", []string{"[mode]"}, "set the runtime to exercise {rego,wasm} (default rego)"},
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
	p := strings.Fields(strings.TrimSpace(line))
	if len(p) == 0 {
		return nil
	}
	inputCommand := strings.ToLower(p[0])
	for i := range builtin {
		if builtin[i].name == inputCommand {
			return &command{
				op:   builtin[i].name,
				args: p[1:],
			}
		}
	}
	return nil
}

func dumpStorage(ctx context.Context, store storage.Store, txn storage.Transaction, w io.Writer) error {
	data, err := store.Read(ctx, txn, storage.RootPath)
	if err != nil {
		return err
	}
	e := json.NewEncoder(w)
	return e.Encode(data)
}

func isGlobalInModule(compiler *ast.Compiler, module *ast.Module, term *ast.Term) bool {

	var name ast.Var

	if ast.RootDocumentRefs.Contains(term) {
		name = term.Value.(ast.Ref)[0].Value.(ast.Var)
	} else if v, ok := term.Value.(ast.Var); ok {
		name = v
	} else {
		return false
	}

	for _, imp := range module.Imports {
		if imp.Name().Compare(name) == 0 {
			return true
		}
	}

	path := module.Package.Path.Copy().Append(ast.StringTerm(string(name)))
	node := compiler.RuleTree

	for _, elem := range path {
		node = node.Child(elem.Value)
		if node == nil {
			return false
		}
	}

	return len(node.Values) > 0
}

func printHelp(output io.Writer, initPrompt string, report [][2]string) {
	printHelpExamples(output, initPrompt)
	printHelpCommands(output)
	if len(report) != 0 {
		printOPAReleaseInfo(output, report)
	}
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

	all := append(extra[:], builtin[:]...)

	// Compute max length of all command and topic names.
	names := make([]string, 0, len(all)+len(topics))

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

func printOPAReleaseInfo(output io.Writer, report [][2]string) {

	fmt.Fprintln(output, "Version Info")
	fmt.Fprintln(output, "============")
	fmt.Fprintln(output)

	maxLen := 0

	for _, pair := range report {
		if len(pair[0]) > maxLen {
			maxLen = len(pair[0])
		}
	}

	fmtStr := fmt.Sprintf("%%-%dv : %%v\n", maxLen)

	fmt.Fprintf(output, fmtStr, "Current Version", version.Version)
	for _, pair := range report {
		fmt.Fprintf(output, fmtStr, pair[0], pair[1])
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

	# Import a future keyword.
	> import future.keywords.in
	> 1 in [0, 2, 1]
	true

	# Define rule that refers to "params".
	> is_post { params.method = "POST" }

	# Test evaluation.
	> is_post
	true`) + "\n"

	fmt.Fprintln(output, txt)
	return nil
}

func printHelpPartial(output io.Writer) error {

	printHelpTitle(output, "Partial Evaluation")

	txt := strings.TrimSpace(`
Rego queries can be partially evaluated with respect to the specific unknown
variables, inputs, or any document rooted under data. The result of partial
evaluation is a new set of queries that can be evaluated later.

For example:

	> allowed_methods = ["GET", "HEAD"]

	# Enable partial evaluation. Treat input document as unknown.
	> unknown input

	# Partially evaluate a query.
	> method = allowed_methods[i]; input.method = method
	input.method = "GET"; i = 0; method = "GET"
	input.method = "HEAD"; i = 1; method = "HEAD"

	# Turn off partial evaluation by running the 'unknown' command with no arguments.
	> unknown`) + "\n"

	fmt.Fprintln(output, txt)
	return nil
}

func printHelpTitle(output io.Writer, title string) {
	fmt.Fprintln(output, "")
	fmt.Fprintln(output, title)
	fmt.Fprintln(output, strings.Repeat("=", len(title)))
	fmt.Fprintln(output, "")
}
