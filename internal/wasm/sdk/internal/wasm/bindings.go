// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package wasm

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
	"github.com/open-policy-agent/opa/v1/topdown/print"
)

// instantiateOPAModule registers the "opa" host module in r with all OPA host
// functions backed by d. It must be called before the "env" glue module and the
// policy module are instantiated, since both import from "opa".
func instantiateOPAModule(ctx context.Context, r wazero.Runtime, d *builtinDispatcher) error {
	b := r.NewHostModuleBuilder(hostModuleName)
	b.NewFunctionBuilder().WithFunc(d.opaAbort).Export("opa_abort")
	b.NewFunctionBuilder().WithFunc(d.opaPrintln).Export("opa_println")
	b.NewFunctionBuilder().WithFunc(d.opaBuiltin0).Export("opa_builtin0")
	b.NewFunctionBuilder().WithFunc(d.opaBuiltin1).Export("opa_builtin1")
	b.NewFunctionBuilder().WithFunc(d.opaBuiltin2).Export("opa_builtin2")
	b.NewFunctionBuilder().WithFunc(d.opaBuiltin3).Export("opa_builtin3")
	b.NewFunctionBuilder().WithFunc(d.opaBuiltin4).Export("opa_builtin4")
	_, err := b.Instantiate(ctx)
	return err
}

// builtinDispatcher routes opa_builtinN calls from wasm to Go topdown builtins.
type builtinDispatcher struct {
	ctx      *topdown.BuiltinContext
	builtins map[int32]topdown.BuiltinFunc

	// Wired after policy module instantiation.
	mem          api.Memory
	malloc       api.Function
	valueDumpFn  api.Function
	valueParseFn api.Function
}

func newBuiltinDispatcher() *builtinDispatcher {
	return &builtinDispatcher{}
}

func (d *builtinDispatcher) SetMap(m map[int32]topdown.BuiltinFunc) {
	d.builtins = m
}

// Reset is called in Eval before using the builtinDispatcher. It (re)builds the
// BuiltinContext and starts a single goroutine that bridges context
// cancellation into topdown.Cancel, so builtins that cooperate via topdown.Cancel
// (e.g. net.cidr_expand) are aborted when ctx is done. The returned stop func
// must be called when the eval completes to tear that goroutine down.
func (d *builtinDispatcher) Reset(ctx context.Context,
	seed io.Reader,
	ns time.Time,
	iqbCache cache.InterQueryCache,
	ndbCache builtins.NDBCache,
	ph print.Hook,
	capabilities *ast.Capabilities) (stop func()) {
	if ns.IsZero() {
		ns = time.Now()
	}
	if seed == nil {
		seed = rand.Reader
	}
	d.ctx = &topdown.BuiltinContext{
		Context:                ctx,
		Metrics:                metrics.New(),
		Seed:                   seed,
		Time:                   ast.NumberTerm(json.Number(strconv.FormatInt(ns.UnixNano(), 10))),
		Cancel:                 topdown.NewCancel(),
		Runtime:                nil,
		Cache:                  make(builtins.Cache),
		Location:               nil,
		Tracers:                nil,
		QueryTracers:           nil,
		QueryID:                0,
		ParentID:               0,
		InterQueryBuiltinCache: iqbCache,
		NDBuiltinCache:         ndbCache,
		PrintHook:              ph,
		Capabilities:           capabilities,
	}

	// WithCloseOnContextDone interrupts wasm-native execution, but it cannot
	// preempt a running Go builtin; those cooperate via topdown.Cancel instead.
	// A single bridge per eval suffices since d.ctx.Cancel lives for the eval.
	// Capture cancel before spawning to avoid a race with the next Reset() call
	// overwriting d.ctx.
	cancel := d.ctx.Cancel
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-ctx.Done():
			cancel.Cancel()
		}
	}()
	return func() { close(done) }
}

func (d *builtinDispatcher) opaAbort(_ context.Context, addr int32) {
	uaddr := uint32(addr)
	size := d.mem.Size()
	if uaddr >= size {
		panic(abortError{message: "(unreadable abort message)"})
	}
	data, ok := d.mem.Read(uaddr, size-uaddr)
	if !ok {
		panic(abortError{message: "(unreadable abort message)"})
	}
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		panic(abortError{message: "(unterminated abort message)"})
	}
	panic(abortError{message: string(data[:n])})
}

func (d *builtinDispatcher) opaPrintln(_ context.Context, addr int32) {
	uaddr := uint32(addr)
	size := d.mem.Size()
	if uaddr >= size {
		return
	}
	data, ok := d.mem.Read(uaddr, size-uaddr)
	if !ok {
		return
	}
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		return
	}
	fmt.Fprintln(os.Stderr, string(data[:n]))
}

func (d *builtinDispatcher) opaBuiltin0(ctx context.Context, id, _ int32) int32 {
	return d.dispatch(ctx, id)
}

func (d *builtinDispatcher) opaBuiltin1(ctx context.Context, id, _, a1 int32) int32 {
	return d.dispatch(ctx, id, a1)
}

func (d *builtinDispatcher) opaBuiltin2(ctx context.Context, id, _, a1, a2 int32) int32 {
	return d.dispatch(ctx, id, a1, a2)
}

func (d *builtinDispatcher) opaBuiltin3(ctx context.Context, id, _, a1, a2, a3 int32) int32 {
	return d.dispatch(ctx, id, a1, a2, a3)
}

func (d *builtinDispatcher) opaBuiltin4(ctx context.Context, id, _, a1, a2, a3, a4 int32) int32 {
	return d.dispatch(ctx, id, a1, a2, a3, a4)
}

// dispatch looks up the builtin by id, decodes its arguments, calls the Go
// implementation, and writes the result back into Wasm memory.
func (d *builtinDispatcher) dispatch(ctx context.Context, id int32, argAddrs ...int32) int32 {
	if d.ctx == nil {
		panic(abortError{message: "unreachable: uninitialized builtin dispatcher context"})
	}
	if d.builtins == nil {
		panic(abortError{message: "unreachable: uninitialized builtin dispatcher index"})
	}

	convertedArgs := make([]*ast.Term, 0, len(argAddrs))
	for _, addr := range argAddrs {
		x, err := d.fromWasmValue(ctx, addr)
		if err != nil {
			panic(builtinError{err: err})
		}
		convertedArgs = append(convertedArgs, x)
	}

	var output *ast.Term
	err := d.builtins[id](*d.ctx, convertedArgs, func(t *ast.Term) error {
		output = t
		return nil
	})
	if err != nil {
		if errors.As(err, &topdown.Halt{}) {
			var e *topdown.Error
			if errors.As(err, &e) && e.Code == topdown.CancelErr {
				panic(cancelledError{message: e.Message})
			}
			panic(builtinError{err: err})
		}
		// non-halt errors are undefined in wasm's non-strict eval mode
	}

	if output == nil {
		return 0
	}

	addr, err := d.toWasmValue(ctx, output)
	if err != nil {
		panic(builtinError{err: err})
	}
	return addr
}

// fromWasmValue dumps the OPA value at addr to a string and parses it as an
// ast.Term.
func (d *builtinDispatcher) fromWasmValue(ctx context.Context, addr int32) (*ast.Term, error) {
	res, err := d.valueDumpFn.Call(ctx, uint64(uint32(addr)))
	if err != nil {
		return nil, err
	}
	serialized := uint32(res[0])
	data, ok := d.mem.Read(serialized, d.mem.Size()-serialized)
	if !ok {
		return nil, errors.New("invalid serialized value address")
	}
	n := bytes.IndexByte(data, 0)
	if n < 0 {
		return nil, errors.New("unterminated serialized value")
	}
	return ast.ParseTerm(string(data[:n]))
}

// toWasmValue serialises term, writes the bytes into Wasm memory via
// opa_malloc, and parses them back as an OPA value, returning the value addr.
func (d *builtinDispatcher) toWasmValue(ctx context.Context, term *ast.Term) (int32, error) {
	raw := []byte(term.String())
	n := uint64(len(raw))
	res, err := d.malloc.Call(ctx, n)
	if err != nil {
		return 0, fmt.Errorf("opa_malloc: %w", err)
	}
	p := uint32(res[0])
	if !d.mem.Write(p, raw) {
		return 0, fmt.Errorf("write at %d", p)
	}
	res, err = d.valueParseFn.Call(ctx, uint64(p), n)
	if err != nil {
		return 0, fmt.Errorf("opa_value_parse: %w", err)
	}
	return int32(res[0]), nil
}
