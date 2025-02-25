// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package debug

import (
	"context"
	"errors"
	"strconv"
	"sync"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ast/location"
	"github.com/open-policy-agent/opa/v1/logging"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/topdown"
)

type threadState interface{}

type eventAction string

const (
	nopAction   eventAction = "nop"
	breakAction eventAction = "break"
	skipAction  eventAction = "skip"
	stopAction  eventAction = "stop"
)

type eventHandler func(t *thread, stackIndex int, e *topdown.Event, s threadState) (eventAction, threadState, error)

type ThreadID int

// Thread represents a single thread of execution.
type Thread interface {
	// ID returns the unique identifier for the thread.
	ID() ThreadID
	// Name returns the human-readable name of the thread.
	Name() string
}

type thread struct {
	id              ThreadID
	name            string
	stack           stack
	eventHandler    eventHandler
	breakpointLatch latch
	stopped         bool
	state           threadState
	varManager      *variableManager
	virtualCache    topdown.VirtualCache
	store           storage.Store
	logger          logging.Logger
	mtx             sync.Mutex
}

func (t *thread) ID() ThreadID {
	return t.id
}

func (t *thread) Name() string {
	return t.name
}

func newThread(id ThreadID, name string, stack stack, varManager *variableManager, virtualCache topdown.VirtualCache,
	store storage.Store, logger logging.Logger) *thread {
	t := &thread{
		id:           id,
		name:         name,
		stack:        stack,
		logger:       logger,
		varManager:   varManager,
		virtualCache: virtualCache,
		store:        store,
	}

	// Threads are always created in a paused state.
	_ = t.pause()

	return t
}

func (t *thread) run(ctx context.Context) error {
	for {
		if t.stopped {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		t.logger.Debug("Waiting on breakpoint latch")
		t.breakpointLatch.wait()
		t.logger.Debug("Breakpoint latch released")

		// The thread could get resumed by another goroutine before the eventHandler returns, so we preemptively lock the
		// breakpoint latch and unlock it of we're not supposed to break.
		t.logger.Debug("Preemptively blocking breakpoint latch")
		t.breakpointLatch.block()

		a, err := t.stepIn()
		if err != nil {
			t.stopped = true
			return err
		}

		if a == breakAction {
			t.logger.Debug("break requested; not unblocking breakpoint latch")
		} else {
			t.logger.Debug("No break requested; unblocking breakpoint latch")
			t.breakpointLatch.unblock()
		}
	}
}

func (t *thread) pause() error {
	t.logger.Debug("Pausing thread: %d", t.id)
	t.breakpointLatch.block()
	return nil
}

func (t *thread) resume() error {
	t.logger.Debug("Resuming thread: %d", t.id)
	t.breakpointLatch.unblock()
	return nil
}

func (t *thread) current() (int, *topdown.Event, error) {
	i, e := t.stack.Current()
	return i, e, nil
}

func (t *thread) stepIn() (eventAction, error) {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	if t.stopped {
		return nopAction, errors.New("thread stopped")
	}

	var a eventAction
	for {
		i, e := t.stack.Next()
		t.logger.Debug("Step-in on event: #%d", i)

		var s threadState
		var err error
		a, s, err = t.eventHandler(t, i, e, t.state)
		if err != nil {
			return nopAction, err
		}
		t.state = s

		if a != skipAction {
			break
		}
	}

	return a, nil
}

func (t *thread) stepOver() error {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	if t.stopped {
		return errors.New("thread stopped")
	}

	_, startE, err := t.current()
	if err != nil {
		return err
	}

	hasExited := startE != nil && (startE.Op == topdown.ExitOp || startE.Op == topdown.FailOp)

	baseQueryVisited := false
Loop:
	for {
		i, e := t.stack.Next()
		t.logger.Debug("Step-over on event #%d:\n%v", i, e)

		if e != nil && e.QueryID == 0 {
			baseQueryVisited = true
		}

		a, s, err := t.eventHandler(t, i, e, t.state)
		if err != nil {
			return err
		}
		t.state = s

		if a == skipAction {
			continue
		}

		var qid uint64
		if e != nil {
			qid = e.QueryID
		}

		switch {
		case startE == nil:
			t.logger.Debug("Resuming on query: %d; first event", qid)
			break Loop
		case a == breakAction:
			t.logger.Debug("Resuming on query: %d; break-action", qid)
			break Loop
		case e == nil:
			t.logger.Debug("Resuming on query: %d; no event", qid)
			break Loop
		case e.QueryID == 0:
			t.logger.Debug("Continuing past query: %d; base-query", qid)
		case e.QueryID <= startE.QueryID:
			t.logger.Debug("Resuming on query: %d; start-query: %d", qid, startE.QueryID)
			break Loop
		case hasExited && e.Op == topdown.EnterOp && qid != startE.QueryID:
			// We have exited the current query scope, and entered a new one; probably a relative partial rule.
			t.logger.Debug("Resuming on query: %d; query-exited", qid)
			break Loop
		case baseQueryVisited:
			t.logger.Debug("Resuming on query: %d; base-query visited", qid)
			break Loop
		default:
			t.logger.Debug("Continuing past query: %d", qid)
		}
	}

	return nil
}

func (t *thread) stepOut() error {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	if t.stopped {
		return errors.New("thread stopped")
	}

	_, c, err := t.current()
	if err != nil {
		return err
	}

	for {
		i, e := t.stack.Next()
		t.logger.Debug("Step-out on event: #%d", i)

		a, s, err := t.eventHandler(t, i, e, t.state)
		if err != nil {
			return err
		}
		t.state = s

		if a == skipAction {
			continue
		}

		var qid uint64
		if e != nil {
			qid = e.QueryID
		}

		if a == breakAction || e == nil || c == nil || qid < c.QueryID {
			t.logger.Debug("Resuming on query: %d", qid)
			break
		}
		t.logger.Debug("Continuing past query: %d", qid)
	}

	return nil
}

func (t *thread) stackEvents(from int) []*topdown.Event {
	var events []*topdown.Event
	for {
		e := t.stack.Event(from)
		if e == nil {
			break
		}
		events = append(events, e)
		from++
	}
	return events
}

// Scope represents the variable state of a StackFrame.
type Scope interface {
	// Name returns the human-readable name of the scope.
	Name() string

	// NamedVariables returns the number of named variables in the scope.
	NamedVariables() int

	// VariablesReference returns a reference to the variables in the scope.
	VariablesReference() VarRef

	// Location returns the in-source location of the scope.
	Location() *location.Location
}

type scope struct {
	name               string
	namedVariables     int
	variablesReference VarRef
	location           *location.Location
}

func (s scope) Name() string {
	return s.name
}

func (s scope) NamedVariables() int {
	return s.namedVariables
}

func (s scope) VariablesReference() VarRef {
	return s.variablesReference
}

func (s scope) Location() *location.Location {
	return s.location
}

func (t *thread) scopes(stackIndex int) []Scope {
	e := t.stack.Event(stackIndex)
	if e == nil {
		return nil
	}

	scopes := make([]Scope, 0, 3)

	// TODO: Clients are expected to keep track of fetched scopes and variable references (vs-code does),
	// but it wouldn't hurt to not register the same var-getter callback more than once.
	localScope := scope{
		name:               "Locals",
		namedVariables:     e.Locals.Len(),
		variablesReference: t.localVars(e),
		location:           e.Location,
	}
	scopes = append(scopes, localScope)

	if t.virtualCache != nil {
		// We only show "global vars" from the virtual cache when at the top of the stack,
		// to not need to store a copy for every frame.
		top, _ := t.stack.Current()
		if stackIndex == top {
			keys := t.virtualCache.Keys()
			virtualCacheScope := scope{
				name:               "Virtual Cache",
				namedVariables:     len(keys),
				variablesReference: t.virtualCacheVars(keys, t.virtualCache),
			}
			scopes = append(scopes, virtualCacheScope)
		}
	}

	if e.Input() != nil {
		inputScope := scope{
			name:               "Input",
			namedVariables:     1,
			variablesReference: t.inputVars(e),
		}
		scopes = append(scopes, inputScope)
	} else {
		inputScope := scope{
			name:           "Input (not provided)",
			namedVariables: 0,
		}
		scopes = append(scopes, inputScope)
	}

	if t.store != nil {
		dataScope := scope{
			name:               "Data",
			namedVariables:     1,
			variablesReference: t.dataVars(),
		}
		scopes = append(scopes, dataScope)
	} else {
		dataScope := scope{
			name:           "Data (not provided)",
			namedVariables: 0,
		}
		scopes = append(scopes, dataScope)
	}

	if rs := t.stack.Result(); rs != nil {
		resultScope := scope{
			name:               "Result Set",
			namedVariables:     1,
			variablesReference: t.resultVars(rs),
		}
		scopes = append(scopes, resultScope)
	}

	return scopes
}

func (t *thread) localVars(e *topdown.Event) VarRef {
	return t.varManager.addVars(func() []namedVar {
		if e == nil {
			return nil
		}

		vars := make([]namedVar, 0, e.Locals.Len())

		e.Locals.Iter(func(k, v ast.Value) bool {
			name := k.(ast.Var)
			variable := namedVar{
				name:  string(name),
				value: v,
			}

			meta, ok := e.LocalMetadata[name]
			if ok {
				variable.name = string(meta.Name)
			}

			vars = append(vars, variable)
			return false
		})

		return vars
	})
}

func (t *thread) virtualCacheVars(keys []ast.Ref, cache topdown.VirtualCache) VarRef {
	return t.varManager.addVars(func() []namedVar {
		if cache == nil {
			return nil
		}

		vars := make([]namedVar, 0, len(keys))
		for _, key := range keys {
			term, undefined := cache.Get(key)
			var value ast.Value
			if undefined {
				value = ast.NullValue
			} else {
				value = term.Value
			}

			variable := namedVar{
				name:  key.String(),
				value: value,
			}
			vars = append(vars, variable)
		}

		return vars
	})
}

func (t *thread) inputVars(e *topdown.Event) VarRef {
	return t.varManager.addVars(func() []namedVar {
		input := e.Input()
		if input == nil {
			return nil
		}

		return []namedVar{{name: "input", value: input.Value}}
	})
}

func (t *thread) dataVars() VarRef {
	return t.varManager.addVars(func() []namedVar {
		ctx := context.Background()
		d, err := storage.ReadOne(ctx, t.store, storage.Path{})
		if err != nil {
			return nil
		}
		v, err := ast.InterfaceToValue(d)
		if err != nil {
			return nil
		}
		return []namedVar{{name: "data", value: v}}
	})
}

func (t *thread) resultVars(rs rego.ResultSet) VarRef {
	vars := make([]namedVar, 0, len(rs))
	for i, result := range rs {
		bindings, err := ast.InterfaceToValue(result.Bindings)
		if err != nil {
			continue
		}

		expressions := &ast.Array{}
		for _, expr := range result.Expressions {
			t := ast.StringTerm(expr.Text)
			v, err := ast.InterfaceToValue(expr.Value)
			if err != nil {
				continue
			}
			expressions = expressions.Append(ast.NewTerm(ast.NewObject(
				ast.Item(ast.StringTerm("text"), t),
				ast.Item(ast.StringTerm("value"), ast.NewTerm(v)),
			)))
		}

		res := ast.NewObject(
			ast.Item(ast.StringTerm("bindings"), ast.NewTerm(bindings)),
			ast.Item(ast.StringTerm("expressions"), ast.NewTerm(expressions)),
		)

		vars = append(vars, namedVar{
			name:  strconv.Itoa(i),
			value: res,
		})
	}

	return t.varManager.addVars(func() []namedVar {
		return vars
	})
}

func (t *thread) close() error {
	t.stopped = true
	t.breakpointLatch.Close()
	return t.stack.Close()
}

func (t *thread) done() bool {
	t.mtx.Lock()
	defer t.mtx.Unlock()

	return t.stopped || !t.stack.Enabled()
}
