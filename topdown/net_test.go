// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build !race
// +build !race

package topdown

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/foxcpp/go-mockdns"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

// TestNetLookupIPAddr replaces the resolver used by builtinLookupIPAddr.
// Due to some intricacies of the net/LookupIP internals, it seems impossible
// to do that in a way that passes the race detector.
func TestNetLookupIPAddr(t *testing.T) {
	srv, err := mockdns.NewServerWithLogger(map[string]mockdns.Zone{
		"v4.org.": {
			A: []string{"1.2.3.4"},
		},
		"v6.org.": {
			AAAA: []string{"1:2:3::4"},
		},
		"v4-v6.org.": {
			A:    []string{"1.2.3.4"},
			AAAA: []string{"1:2:3::4"},
		},
		"error.org.": {
			Err: fmt.Errorf("OH NO"),
		},
	}, sink{}, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })

	srvFail, err := mockdns.NewServerWithLogger(map[string]mockdns.Zone{}, sink{}, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srvFail.Close() })
	t.Cleanup(func() { mockdns.UnpatchNet(resolv) })

	for addr, exp := range map[string]ast.Set{
		"v4.org":    ast.NewSet(ast.StringTerm("1.2.3.4")),
		"v6.org":    ast.NewSet(ast.StringTerm("1:2:3::4")),
		"v4-v6.org": ast.NewSet(ast.StringTerm("1.2.3.4"), ast.StringTerm("1:2:3::4")),
	} {
		t.Run(addr, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			bctx := BuiltinContext{
				Context: ctx,
				Cache:   make(builtins.Cache),
			}
			srv.PatchNet(resolv)
			err := builtinLookupIPAddr(bctx, []*ast.Term{ast.StringTerm(addr)}, func(act *ast.Term) error {
				if exp.Compare(act.Value) != 0 {
					t.Errorf("expected %v, got %v", exp, act)
				}
				return nil
			})
			if err != nil {
				t.Error(err)
			}

			// check cache put
			act, ok := bctx.Cache.Get(lookupIPAddrCacheKey(addr))
			if !ok {
				t.Fatal("result not put into cache")
			}
			if exp.Compare(act.(*ast.Term).Value) != 0 {
				t.Errorf("cache: expected %v, got %v", exp, act)
			}

			// exercise cache hit
			srvFail.PatchNet(resolv)
			err = builtinLookupIPAddr(bctx, []*ast.Term{ast.StringTerm(addr)}, func(act *ast.Term) error {
				if exp.Compare(act.Value) != 0 {
					t.Errorf("expected %v, got %v", exp, act)
				}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		})
	}

	for _, addr := range []string{"error.org", "nosuch.org"} {
		t.Run(addr, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			bctx := BuiltinContext{
				Context: ctx,
				Cache:   make(builtins.Cache),
			}
			srv.PatchNet(resolv)
			err := builtinLookupIPAddr(bctx, []*ast.Term{ast.StringTerm(addr)}, func(*ast.Term) error {
				t.Fatal("expected not to be called")
				return nil
			})
			if err == nil {
				t.Error("expected error")
			}
			if testing.Verbose() {
				t.Log(err)
			}
		})
	}

	cancelled := func() (context.Context, func()) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx, cancel
	}
	timedOut := func() (context.Context, func()) {
		return context.WithTimeout(context.Background(), time.Nanosecond)
	}

	for name, ctx := range map[string]func() (context.Context, func()){
		"cancelled": cancelled,
		"timed out": timedOut,
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := ctx()
			defer cancel()
			bctx := BuiltinContext{
				Context: ctx,
				Cache:   make(builtins.Cache),
			}
			srv.PatchNet(resolv)
			err := builtinLookupIPAddr(bctx, []*ast.Term{ast.StringTerm("example.org")}, func(*ast.Term) error {
				t.Fatal("expected not to be called")
				return nil
			})
			if err == nil {
				t.Fatal("expected error")
			}
			_, ok := err.(Halt)
			if !ok {
				t.Errorf("expected Halt error, got %v (%[1]T)", err)
			}
			if !IsCancel(err) {
				t.Errorf("expected wrapped Cancel error, got %v (%[1]T)", err)
			}
		})
	}
}

type sink struct{}

func (sink) Printf(string, ...interface{}) {}
