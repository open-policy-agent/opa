// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package watch is deprecated.
package watch

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/dependencies"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
)

// Watcher allows for watches to be registered on queries.
type Watcher struct {
	store    storage.Store
	compiler *ast.Compiler
	ctx      context.Context
	trigger  storage.TriggerHandle

	handles   map[*Handle]struct{}
	dataWatch map[string]map[signal]struct{} // map from path -> set of signals

	closed bool

	l sync.Mutex // The lock around creating and ending watches.
}

// Handle allows a user to listen to and end a watch on a query.
type Handle struct {
	C <-chan Event

	instrument bool      // whether this query should be instrumented
	query      string    // the original query, used for migration
	runtime    *ast.Term // runtime info to provide to evaluation engine

	out    chan Event // out is the same channel as C, but without directional constraints
	notify signal     // channel to receive new data change alerts on.

	done signal // closed by the watcher to signal the sending goroutine to stop sending query results.
	ack  signal // closed by the sending goroutine to tell the watcher it has stopped sending query results.

	watcher *Watcher
	l       sync.Mutex
}

// Event represents a change to a query. Query is the query in question and Value is the
// JSON encoded results of the new query evaluation. Error will be populated if
// evaluating the new query results encountered an error for any reason. If Error is not
// nil, the contents of Value are undefined.
//
// Metrics and Tracer represent the metrics and trace from the evaluation of the query.
type Event struct {
	Query string         `json:"query"`
	Value rego.ResultSet `json:"value"`
	Error error          `json:"error,omitempty"`

	Metrics metrics.Metrics      `json:"-"`
	Tracer  topdown.BufferTracer `json:"-"`
}

type signal chan struct{}

// New creates and returns a new Watcher on the store using the compiler provided.
// Once a compiler is provided to create a Watcher, it must not be modified, or else
// the results produced by the Watcher are undefined.
func New(ctx context.Context, s storage.Store, c *ast.Compiler, txn storage.Transaction) (w *Watcher, err error) {
	w = create(ctx, s, c)
	w.trigger, err = s.Register(ctx, txn, storage.TriggerConfig{OnCommit: w.notify})
	return w, err
}

// NewQuery returns a new watch Handle that can be run. Callers must invoke the
// Run function on the handle to start the watch.
func (w *Watcher) NewQuery(query string) *Handle {
	out := make(chan Event)
	h := &Handle{
		C:       out,
		query:   query,
		out:     out,
		notify:  make(signal, 1),
		done:    make(signal),
		ack:     make(signal),
		watcher: w,
	}
	return h
}

// Query registers a watch on the provided Rego query. Whenever changes are made to a
// base or virtual document that the query depends on, an Event describing the new result
// of the query will be sent through the Handle.
//
// Query will return an error if registering the watch fails for any reason.
func (w *Watcher) Query(query string) (*Handle, error) {
	h := w.NewQuery(query)
	return h, h.Start()
}

// WithInstrumentation enables instrumentation on the query to diagnose
// performance issues.
func (h *Handle) WithInstrumentation(yes bool) *Handle {
	h.instrument = yes
	return h
}

// WithRuntime sets the runtime data to provide to the evaluation engine.
func (h *Handle) WithRuntime(term *ast.Term) *Handle {
	h.runtime = term
	return h
}

// Start registers and starts the watch.
func (h *Handle) Start() error {
	if err := h.watcher.registerHandle(h); err != nil {
		return err
	}
	go h.deliver()
	return nil
}

// Stop ends the watch on the query associated with the Handle. It will close the channel
// that was delivering notifications through the Handle. This may happen before or after
// Stop returns.
func (h *Handle) Stop() {
	h.l.Lock()
	defer h.l.Unlock()

	h.watcher.endQuery(h)
}

// Close ends the watches on all queries this Watcher has.
//
// Further attempts to register or end watches will result in an error after Close() is
// called.
func (w *Watcher) Close(txn storage.Transaction) {
	w.l.Lock()
	defer w.l.Unlock()

	if !w.closed {
		w.close(txn, false)
	}
}

// Migrate creates a new Watcher with the same watches as w, but using the new
// compiler. Like when creating a Watcher with New, the provided compiler must not
// be modified after being passed to Migrate, or else behavior is undefined.
//
// After Migrate returns, the old watcher will be closed, and the new will be ready for
// use. All Handles from the old watcher will still be active, via the returned Watcher,
// with the exception of those Handles who's query is no longer valid with the new
// compiler. Such Handles will be shutdown and a final Event sent along their channel
// indicating the cause of the error.
//
// If an error occurs creating the new Watcher, the state of the old Watcher will not be
// changed.
func (w *Watcher) Migrate(c *ast.Compiler, txn storage.Transaction) (*Watcher, error) {
	w.l.Lock()
	defer w.l.Unlock()
	if w.closed {
		return w, errors.New("cannot migrate a closed Watcher")
	}

	m, err := New(w.ctx, w.store, c, txn)
	if err != nil {
		return w, err
	}

	var handles []*Handle
	for h := range w.handles {
		handles = append(handles, h)
	}
	w.close(txn, true)

	for _, h := range handles {
		if err := m.registerHandle(h); err != nil {
			h.shutDown(newInvalidatedWatchError(err))
		}
	}

	return m, nil
}

func create(ctx context.Context, s storage.Store, c *ast.Compiler) *Watcher {
	return &Watcher{
		store:    s,
		compiler: c,
		ctx:      ctx,

		handles:   map[*Handle]struct{}{},
		dataWatch: map[string]map[signal]struct{}{},
	}
}

func (w *Watcher) registerHandle(h *Handle) error {
	h.l.Lock()
	w.l.Lock()
	defer h.l.Unlock()
	defer w.l.Unlock()

	if w.closed {
		return errors.New("cannot start query watch with closed Watcher")
	}

	parsed, err := ast.ParseBody(h.query)
	if err != nil {
		return err
	}

	compiled, err := w.compiler.QueryCompiler().Compile(parsed)
	if err != nil {
		return err
	}

	refs, err := dependencies.Base(w.compiler, compiled)
	if err != nil {
		panic(err)
	}

	h.watcher = w
	w.handles[h] = struct{}{}
	for _, r := range refs {
		s := r.String()
		if _, ok := w.dataWatch[s]; !ok {
			w.dataWatch[s] = map[signal]struct{}{}
		}
		w.dataWatch[s][h.notify] = struct{}{}
	}

	// Send one notification when we start (like first query).
	select {
	case h.notify <- struct{}{}:
	default:
	}

	return nil
}

func (h *Handle) deliver() {
	defer close(h.ack)
	for {
		select {
		case <-h.notify:
			m := metrics.New()
			t := topdown.NewBufferTracer()

			h.l.Lock()
			r := rego.New(
				rego.Compiler(h.watcher.compiler),
				rego.Store(h.watcher.store),
				rego.Query(h.query),
				rego.Metrics(m),
				rego.QueryTracer(t),
				rego.Instrument(h.instrument),
				rego.Runtime(h.runtime),
			)
			ctx := h.watcher.ctx
			h.l.Unlock()

			result, err := r.Eval(ctx)
			h.out <- Event{
				Query: h.query,
				Value: result,
				Error: err,

				Metrics: m,
				Tracer:  *t,
			}
		case <-h.done:
			return
		}
	}
}

func (w *Watcher) endQuery(h *Handle) {
	w.l.Lock()
	defer w.l.Unlock()

	delete(w.handles, h)
	for _, notifiers := range w.dataWatch {
		delete(notifiers, h.notify)
	}
	h.shutDown(nil)
}

func (h *Handle) shutDown(err error) {
	select {
	case _, ok := <-h.done:
		if !ok {
			return // Handle is already ended.
		}
	default:
	}

	close(h.done) // Tell the goroutine relaying query updates to stop sending.
	go cleanupQuery(h, err)
}

// close assumes that the Watcher is locked. It will not unlock it.
func (w *Watcher) close(txn storage.Transaction, migrating bool) {
	w.trigger.Unregister(w.ctx, txn)
	if !migrating {
		for h := range w.handles {
			h.shutDown(nil)
		}
	}
	w.handles = map[*Handle]struct{}{}
	w.dataWatch = map[string]map[signal]struct{}{}
	w.closed = true
}

func cleanupQuery(h *Handle, err error) {
	<-h.ack // The sending goroutine will close ack.

	// The notify channel has been removed from all dataWatches, and the sending
	// goroutine is no longer listening to it, it's safe to close it.
	close(h.notify)

	if err != nil {
		h.out <- Event{Query: h.query, Value: nil, Error: err}
	}

	// The sending goroutine will no longer send to out, it's safe to close it.
	close(h.out)
}

func (w *Watcher) notify(ctx context.Context, txn storage.Transaction, event storage.TriggerEvent) {
	w.l.Lock()
	defer w.l.Unlock()

	if w.closed {
		panic("not reached")
	}

	// For each piece of changed data, see if there is a watch that depends on it in
	// some way. If there is, send notifications to all of the watches that depend on
	// the changed data. If a notification is already enqueued for a watch, another
	// is not sent, as there would be no reason to (the current changes will
	// passively be included next time the watch is evaluated).

	notifySet := map[signal]struct{}{}
	for _, d := range event.Data {
		r := d.Path.Ref(ast.DefaultRootDocument)
		for path, notifiers := range w.dataWatch {
			if r.HasPrefix(ast.MustParseRef(path)) {
				for notify := range notifiers {
					notifySet[notify] = struct{}{}
				}
			}
		}
	}

	for notify := range notifySet {
		select {
		case notify <- struct{}{}:
		default: // Already a notification in the queue.
		}
	}
}

func newInvalidatedWatchError(err error) error {
	return fmt.Errorf("watch invalidated: %s", err.Error())
}
