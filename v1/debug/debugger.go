// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package debug
// EXPERIMENTAL: This package is under active development and is subject to change.
package debug

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	fileurl "github.com/open-policy-agent/opa/internal/file/url"
	"github.com/open-policy-agent/opa/v1/ast/location"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown"
	prnt "github.com/open-policy-agent/opa/v1/topdown/print"
	"github.com/open-policy-agent/opa/v1/util"
)

// Debugger is the interface for launching OPA debugger Session(s).
// This implementation is similar in structure to the Debug Adapter Protocol (DAP)
// to make such integrations easier, but is not intended to be a direct implementation.
// See: https://microsoft.github.io/debug-adapter-protocol/specification
//
// EXPERIMENTAL: These interfaces are under active development and is subject to change.
type Debugger interface {
	// LaunchEval starts a new eval debug session with the given LaunchEvalProperties.
	// The returned session is in a stopped state, and must be resumed to start execution.
	LaunchEval(ctx context.Context, props LaunchEvalProperties, opts ...LaunchOption) (Session, error)
}

type debugger struct {
	logger       logging.Logger
	printHook    *printHook
	eventHandler EventHandler
}

type Session interface {
	// Resume resumes execution of the thread with the given ID.
	Resume(threadID ThreadID) error

	// ResumeAll resumes execution of all threads in the session.
	ResumeAll() error

	// StepOver executes the next expression in the current scope and then stops on the next expression in the same scope,
	// not stopping on expressions in sub-scopes; e.g. execution of referenced rule, called function, comprehension, or every expression.
	//
	// Example 1:
	//
	//	allow if {
	//	  x := f(input) >-+
	//	  x == 1        <-+
	//	}
	//
	// Example 2:
	//
	//	allow if {
	//	  every x in l { >-+
	//	    x < 10         |
	//	  }                |
	//	  input.x == 1   <-+
	//	}
	StepOver(threadID ThreadID) error

	// StepIn executes the next expression in the current scope and then stops on the next expression in the same scope or sub-scope;
	// stepping into any referenced rule, called function, comprehension, or every expression.
	//
	// Example 1:
	//
	//	allow if {
	//	  x := f(input) >-+
	//	  x == 1          |
	//	}                 |
	//	                  |
	//	f(x) := y if {  <-+
	//	  y := x + 1
	//	}
	//
	// Example 2:
	//
	//	allow if {
	//	  every x in l { >-+
	//	    x < 10       <-+
	//	  }
	//	  input.x == 1
	//	}
	StepIn(threadID ThreadID) error

	// StepOut steps out of the current scope (rule, function, comprehension, every expression) and stops on the next expression in the parent scope.
	//
	// Example 1:
	//
	//	allow if {
	//	  x := f(input) <-+
	//	  x == 1          |
	//	}                 |
	//	                  |
	//	f(x) := y if {    |
	//	  y := x + 1    >-+
	//	}
	//
	// Example 2:
	//
	//	allow if {
	//	  every x in l {
	//	    x < 10       >-+
	//	  }                |
	//	  input.x == 1   <-+
	//	}
	StepOut(threadID ThreadID) error

	// Threads returns a list of all threads in the session.
	Threads() ([]Thread, error)

	// Breakpoints returns a list of all set breakpoints.
	Breakpoints() ([]Breakpoint, error)

	// AddBreakpoint sets a breakpoint at the given location.
	AddBreakpoint(loc location.Location) (Breakpoint, error)

	// RemoveBreakpoint removes a given breakpoint.
	// The removed breakpoint is returned. If the breakpoint does not exist, nil is returned.
	RemoveBreakpoint(ID BreakpointID) (Breakpoint, error)

	// ClearBreakpoints removes all breakpoints.
	ClearBreakpoints() error

	// StackTrace returns the StackTrace for the thread with the given ID.
	// The stack trace is ordered from the most recent frame to the least recent frame.
	StackTrace(threadID ThreadID) (StackTrace, error)

	// Scopes returns the Scope list for the frame with the given ID.
	Scopes(frameID FrameID) ([]Scope, error)

	// Variables returns the Variable list for the given reference.
	Variables(varRef VarRef) ([]Variable, error)

	// Terminate stops all threads in the session.
	Terminate() error
}

type printHook struct {
	prnt.Hook
	d *debugger
}

func (h *printHook) Print(_ prnt.Context, str string) error {
	if h == nil || h.d == nil {
		return nil
	}
	h.d.eventHandler(Event{Type: StdoutEventType, Message: str})
	return nil
}

type DebuggerOption func(*debugger)

func NewDebugger(options ...DebuggerOption) Debugger {
	return newDebugger(options...)
}

func newDebugger(options ...DebuggerOption) *debugger {
	d := &debugger{
		eventHandler: newNopEventHandler(),
		logger:       logging.NewNoOpLogger(),
	}
	d.printHook = &printHook{d: d}

	for _, option := range options {
		option(d)
	}

	return d
}

func SetLogger(logger logging.Logger) DebuggerOption {
	return func(d *debugger) {
		d.logger = logger
	}
}

func SetEventHandler(handler EventHandler) DebuggerOption {
	return func(d *debugger) {
		d.eventHandler = handler
	}
}

type LaunchEvalProperties struct {
	LaunchProperties
	Query     string
	Input     interface{}
	InputPath string
}

type LaunchTestProperties struct {
	LaunchProperties
	Run string
}

type LaunchProperties struct {
	BundlePaths         []string
	DataPaths           []string
	StopOnResult        bool
	StopOnEntry         bool
	StopOnFail          bool
	EnablePrint         bool
	SkipOps             []topdown.Op
	StrictBuiltinErrors bool
	RuleIndexing        bool
}

type LaunchOption func(options *launchOptions)

type launchOptions struct {
	regoOptions []func(*rego.Rego)
}

func newLaunchOptions(opts []LaunchOption) *launchOptions {
	options := &launchOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

// RegoOption adds a rego option to the internal Rego instance.
// Options may be overridden by the debugger, and it is recommended to
// use LaunchEvalProperties for commonly used options.
func RegoOption(opt func(*rego.Rego)) LaunchOption {
	return func(options *launchOptions) {
		options.regoOptions = append(options.regoOptions, opt)
	}
}

func (lp LaunchProperties) String() string {
	b, err := json.Marshal(lp)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (d *debugger) LaunchEval(ctx context.Context, props LaunchEvalProperties, opts ...LaunchOption) (Session, error) {
	options := newLaunchOptions(opts)

	store := inmem.New()
	txn, err := store.NewTransaction(ctx, storage.TransactionParams{Write: true})
	if err != nil {
		return nil, fmt.Errorf("failed to create store transaction: %v", err)
	}

	regoArgs := make([]func(*rego.Rego), 0, 4)

	// We apply all user options first, so the debugger can make overrides if necessary.
	regoArgs = append(regoArgs, options.regoOptions...)

	regoArgs = append(regoArgs, rego.Query(props.Query))
	regoArgs = append(regoArgs, rego.Store(store))
	regoArgs = append(regoArgs, rego.Transaction(txn))
	regoArgs = append(regoArgs, rego.StrictBuiltinErrors(props.StrictBuiltinErrors))

	if props.SkipOps == nil {
		props.SkipOps = []topdown.Op{topdown.IndexOp, topdown.RedoOp, topdown.SaveOp, topdown.UnifyOp}
	}

	if props.EnablePrint {
		regoArgs = append(regoArgs, rego.EnablePrintStatements(true),
			rego.PrintHook(d.printHook))
	}

	if len(props.DataPaths) > 0 {
		regoArgs = append(regoArgs, rego.Load(props.DataPaths, nil))
	}

	for _, bundlePath := range props.BundlePaths {
		regoArgs = append(regoArgs, rego.LoadBundle(bundlePath))
	}

	if props.InputPath != "" && props.Input != nil {
		return nil, errors.New("cannot specify both input and input path")
	}

	if props.Input != nil {
		regoArgs = append(regoArgs, rego.Input(props.Input))
	} else if props.InputPath != "" {
		input, err := readInput(props.InputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %v", err)
		}
		regoArgs = append(regoArgs, rego.Input(input))
	}

	r := rego.New(regoArgs...)

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare query for evaluation: %v", err)
	}

	// Committing the store transaction here to make any data added in previous steps are available during eval.
	if err := store.Commit(ctx, txn); err != nil {
		return nil, fmt.Errorf("failed to commit store transaction: %v", err)
	}

	tracer := newDebugTracer()

	vc := topdown.NewVirtualCache()

	evalArgs := []rego.EvalOption{
		rego.EvalRuleIndexing(true),
		rego.EvalEarlyExit(true),
		rego.EvalQueryTracer(tracer),
		rego.EvalRuleIndexing(props.RuleIndexing),
		rego.EvalVirtualCache(vc),
	}

	varManager := newVariableManager()
	// Threads are 1-indexed.
	t := newThread(1, "main", tracer, varManager, vc, store, d.logger)
	s := newSession(ctx, d, varManager, props.LaunchProperties, []*thread{t})

	go func() {
		defer func() { _ = tracer.Close() }()
		rs, evalErr := pq.Eval(s.ctx, evalArgs...)
		if evalErr != nil {
			var topdownErr *topdown.Error
			if errors.As(evalErr, &topdownErr) && topdownErr.Code == topdown.CancelErr {
				return
			}
			d.logger.Error("Evaluation failed: %v", evalErr)
			return
		}

		tracer.resultSet = rs
		s.result(t, rs)
	}()

	if err := s.start(); err != nil {
		return nil, err
	}
	return s, nil
}

func readInput(path string) (any, error) {
	path, err := fileurl.Clean(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var input any
	if err := util.Unmarshal(data, &input); err != nil {
		return nil, err
	}

	return input, nil
}

type session struct {
	d              *debugger
	properties     LaunchProperties
	threads        []*thread
	frames         []*stackFrame
	framesByThread map[ThreadID][]*stackFrame
	breakpoints    *breakpointCollection
	ctx            context.Context
	cancel         context.CancelFunc
	varManager     *variableManager
	mtx            sync.Mutex
}

func newSession(ctx context.Context, debugger *debugger, varManager *variableManager, props LaunchProperties, threads []*thread) *session {
	ctx, cancel := context.WithCancel(ctx)
	s := &session{
		d:              debugger,
		varManager:     varManager,
		properties:     props,
		threads:        threads,
		frames:         []*stackFrame{},
		framesByThread: map[ThreadID][]*stackFrame{},
		breakpoints:    newBreakpointCollection(),
		ctx:            ctx,
		cancel:         cancel,
	}

	for _, t := range threads {
		t.eventHandler = s.handleEvent
	}

	return s
}

func (s *session) start() error {
	if s == nil {
		return errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, t := range s.threads {
		go func() {
			s.d.logger.Debug("Thread %d started", t.id)
			s.d.sendEvent(Event{Type: ThreadEventType, Thread: t.id, Message: "started"})
			if err := t.run(s.ctx); err != nil {
				s.d.logger.Error("Thread %d failed: %v", t.id, err)
			}
			s.d.logger.Debug("Thread %d stopped", t.id)
			s.d.sendEvent(Event{Type: ThreadEventType, Thread: t.id, Message: "exited"})

			allStopped := true
			for _, t := range s.threads {
				if !t.done() {
					allStopped = false
					break
				}
			}

			if allStopped {
				s.d.logger.Debug("All threads stopped")
				s.d.sendEvent(Event{Type: TerminatedEventType})
			}
		}()
	}

	return nil
}

func (s *session) thread(id ThreadID) (*thread, error) {
	if s == nil {
		return nil, errors.New("no active debug session")
	}

	index := int(id - 1)
	if index < 0 || index >= len(s.threads) {
		return nil, fmt.Errorf("invalid thread id: %d", id)
	}
	return s.threads[index], nil
}

func (s *session) Resume(threadID ThreadID) error {
	if s == nil {
		return errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	t, err := s.thread(threadID)
	if err != nil {
		return err
	}

	return t.resume()
}

func (s *session) ResumeAll() error {
	if s == nil {
		return errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	for _, t := range s.threads {
		if err := t.resume(); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) StepOver(threadID ThreadID) error {
	if s == nil {
		return errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	t, err := s.thread(threadID)
	if err != nil {
		return err
	}

	err = t.stepOver()
	if err == nil {
		i, e, _ := t.current()
		if e != nil {
			s.d.sendEvent(Event{Type: StoppedEventType, Thread: t.id, Message: "step", stackIndex: i, stackEvent: e})
		}
	}
	if t.done() {
		s.d.sendEvent(Event{Type: ThreadEventType, Thread: t.id, Message: "exited"})
		allStopped := true
		for _, t := range s.threads {
			if !t.done() {
				allStopped = false
				break
			}
		}
		if allStopped {
			s.d.sendEvent(Event{Type: TerminatedEventType})
		}
	}

	return err
}

func (s *session) StepIn(threadID ThreadID) error {
	if s == nil {
		return errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	t, err := s.thread(threadID)
	if err != nil {
		return err
	}

	_, err = t.stepIn()
	if err == nil {
		i, e, _ := t.current()
		if e != nil {
			s.d.sendEvent(Event{Type: StoppedEventType, Thread: t.id, Message: "step", stackIndex: i, stackEvent: e})
		}
	}
	if t.done() {
		s.d.sendEvent(Event{Type: ThreadEventType, Thread: t.id, Message: "exited"})
		allStopped := true
		for _, t := range s.threads {
			if !t.done() {
				allStopped = false
				break
			}
		}
		if allStopped {
			s.d.sendEvent(Event{Type: TerminatedEventType})
		}
	}

	return err
}

func (s *session) StepOut(threadID ThreadID) error {
	if s == nil {
		return errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	t, err := s.thread(threadID)
	if err != nil {
		return err
	}

	err = t.stepOut()
	if err == nil {
		i, e, _ := t.current()
		if e != nil {
			s.d.sendEvent(Event{Type: StoppedEventType, Thread: t.id, Message: "step", stackIndex: i, stackEvent: e})
		}
	}
	if t.done() {
		s.d.sendEvent(Event{Type: ThreadEventType, Thread: t.id, Message: "exited"})
		allStopped := true
		for _, t := range s.threads {
			if !t.done() {
				allStopped = false
				break
			}
		}
		if allStopped {
			s.d.sendEvent(Event{Type: TerminatedEventType})
		}
	}

	return err
}

func (s *session) Threads() ([]Thread, error) {
	if s == nil {
		return nil, errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	threads := make([]Thread, 0, len(s.threads))
	for _, t := range s.threads {
		threads = append(threads, t)
	}

	return threads, nil
}

type sessionThreadState struct {
	entered     bool
	ended       bool
	prevQueryID uint64
}

func (s *sessionThreadState) String() string {
	return fmt.Sprintf("{entered: %v, ended: %v, prevQueryID: %d}", s.entered, s.ended, s.prevQueryID)
}

func (s *session) handleEvent(t *thread, stackIndex int, e *topdown.Event, ts threadState) (eventAction, threadState, error) {
	state, ok := ts.(*sessionThreadState)
	if state != nil && !ok {
		s.d.logger.Warn("invalid thread state: %v", s)
	}
	if state == nil {
		state = &sessionThreadState{}
	}

	defer func() {
		if e != nil {
			state.prevQueryID = e.QueryID
		} else {
			state.prevQueryID = 0
		}
	}()

	if e == nil {
		handleEnd := func() (eventAction, threadState, error) {
			_ = t.close()
			return stopAction, state, nil
		}

		if state.ended {
			s.d.logger.Debug("End of trace already handled")
			return handleEnd()
		}

		s.d.logger.Debug("Handling end of trace")

		state.ended = true
		if s.properties.StopOnResult {
			s.d.logger.Info("Thread %d stopped at end of trace", t.id)
			s.d.sendEvent(Event{Type: StoppedEventType, Thread: t.id, Message: "result", stackIndex: stackIndex, stackEvent: e})
			return breakAction, state, nil
		}

		return handleEnd()
	}

	if e.Location == nil {
		s.d.logger.Debug("Handling event:\n%v\n\nstate:\n%s", e, state)
	} else {
		s.d.logger.Debug("Handling event:\n%v\n\nloc:\n%s\n\nstate:\n%s", e, e.Location, state)
	}

	if s.skipOp(e.Op) {
		s.d.logger.Debug("Skipping event (op: %v)", e.Op)
		return skipAction, state, nil
	}

	if s.properties.StopOnEntry && !state.entered && e.Location != nil && e.Location.File != "" {
		state.entered = true
		s.d.logger.Info("Thread %d stopped at entry", t.id)
		s.d.sendEvent(Event{Type: StoppedEventType, Thread: t.id, Message: "entry", stackIndex: stackIndex, stackEvent: e})
		return breakAction, state, nil
	}

	if s.properties.StopOnFail && e.Op == topdown.FailOp {
		s.d.logger.Info("Thread %d stopped on failure", t.id)
		s.d.sendEvent(Event{Type: ExceptionEventType, Thread: t.id, Message: string(e.Op), stackIndex: stackIndex, stackEvent: e})
		return breakAction, state, nil
	}

	if e.Location != nil && e.Location.File != "" {
		for _, bp := range s.breakpoints.allForFilePath(e.Location.File) {
			if bp.Location().Row == e.Location.Row {
				// if the last event also caused a breakpoint AND we're still on the same line, skip this breakpoint.
				s.d.logger.Info("Thread %d stopped at breakpoint: %s:%d", t.id, e.Location.File, e.Location.Row)
				s.d.sendEvent(Event{Type: StoppedEventType, Thread: t.id, Message: "breakpoint", stackIndex: stackIndex, stackEvent: e})
				return breakAction, state, nil
			}
		}
	}

	return nopAction, state, nil
}

func (s *session) skipOp(op topdown.Op) bool {
	for _, skip := range s.properties.SkipOps {
		if skip == op {
			return true
		}
	}
	return false
}

func (s *session) result(t *thread, rs rego.ResultSet) {
	if rsJSON, err := json.MarshalIndent(rs, "", "  "); err == nil {
		s.d.logger.Debug("Result: %s\n", rsJSON)
		s.d.sendEvent(Event{Type: StdoutEventType, Thread: t.id, Message: string(rsJSON)})
	} else {
		s.d.logger.Debug("Result: %v\n", rs)
	}
}

func (s *session) StackTrace(threadID ThreadID) (StackTrace, error) {
	if s == nil {
		return nil, errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	t, err := s.thread(threadID)
	if err != nil {
		return nil, err
	}

	threadFrames := s.framesByThread[t.id]
	if threadFrames == nil {
		threadFrames = []*stackFrame{}
	}

	stackIndex := 0
	if len(threadFrames) > 0 {
		stackIndex = threadFrames[len(threadFrames)-1].stackIndex + 1
	}
	newEvents := t.stackEvents(stackIndex)
	for _, e := range newEvents {
		info := s.newStackFrame(e, t, stackIndex)
		stackIndex++
		threadFrames = append(threadFrames, info)
	}
	s.framesByThread[t.id] = threadFrames

	frames := make([]StackFrame, 0, len(threadFrames))
	for _, f := range threadFrames {
		frames = append(frames, f)
	}
	slices.Reverse(frames)

	return frames, nil
}

func (s *session) newStackFrame(e *topdown.Event, t *thread, stackIndex int) *stackFrame {
	id := len(s.frames) + 1 // frames are 1-indexed

	var expl string
	if e.Node != nil {
		pretty := new(bytes.Buffer)
		topdown.PrettyTrace(pretty, []*topdown.Event{e})
		expl = strings.Trim(pretty.String(), "\n")
	} else {
		expl = fmt.Sprintf("%s, %s", e.Op, e.Location)
	}

	frame := &stackFrame{
		id:         FrameID(id),
		name:       fmt.Sprintf("#%d: %d %s", id, e.QueryID, expl),
		location:   e.Location,
		thread:     t.id,
		stackIndex: stackIndex,
		e:          e,
	}

	s.frames = append(s.frames, frame)
	return frame
}

func (s *session) frame(id FrameID) (*stackFrame, error) {
	index := int(id) - 1
	if index < 0 || index >= len(s.frames) {
		return nil, fmt.Errorf("invalid frame id: %d", id)
	}
	return s.frames[index], nil
}

func (s *session) Scopes(frameID FrameID) ([]Scope, error) {
	if s == nil {
		return nil, errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	f, err := s.frame(frameID)
	if err != nil {
		return nil, err
	}

	t, err := s.thread(f.thread)
	if err != nil {
		return nil, err
	}

	return t.scopes(f.stackIndex), nil
}

func (s *session) Variables(varRef VarRef) ([]Variable, error) {
	if s == nil {
		return nil, errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.d.logger.Debug("Variables requested: %d", varRef)

	vars, err := s.varManager.vars(varRef)
	if err != nil {
		return nil, err
	}

	return vars, nil
}

func (s *session) Breakpoints() ([]Breakpoint, error) {
	if s == nil {
		return nil, errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.breakpoints.all(), nil
}

func (s *session) AddBreakpoint(loc location.Location) (Breakpoint, error) {
	if s == nil {
		return nil, errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	return s.breakpoints.add(loc), nil
}

func (s *session) RemoveBreakpoint(ID BreakpointID) (Breakpoint, error) {
	if s == nil {
		return nil, errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	bp := s.breakpoints.remove(ID)
	if bp == nil {
		return nil, fmt.Errorf("breakpoint %d not found", ID)
	}

	return bp, nil
}

func (s *session) ClearBreakpoints() error {
	if s == nil {
		return errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.d.logger.Debug("Clearing existing breakpoints")
	s.breakpoints.clear()
	return nil
}

func (s *session) Terminate() error {
	if s == nil {
		return errors.New("no active debug session")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.cancel()

	var hasErrors bool
	for _, t := range s.threads {
		if err := t.close(); err != nil {
			hasErrors = true
			s.d.logger.Error("Failed to stop thread %d: %v", t.id, err)
		} else {
			s.d.sendEvent(Event{Type: ThreadEventType, Thread: t.id, Message: "exited"})
		}
	}

	if !hasErrors {
		s.d.sendEvent(Event{Type: TerminatedEventType})
	}

	return nil
}

func (d *debugger) sendEvent(e Event) {
	if d == nil || d.eventHandler == nil {
		return
	}

	d.logger.Debug("Sending event: %v", e)

	d.eventHandler(e)
}
