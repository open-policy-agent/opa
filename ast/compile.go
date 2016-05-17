// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/util"
)

// Compiler contains the state of a compilation process.
type Compiler struct {

	// Errors contains errors that occurred during the compilation process.
	// If there are one or more errors, the compilation process is considered
	// "failed".
	Errors []error

	// Modules contains the compiled modules. The compiled modules are the
	// output of the compilation process. If the compilation process failed,
	// there is no guarantee about the state of the modules.
	Modules map[string]*Module

	// Exports contains a mapping of package paths to variables. The variables
	// represent externally accessible symbols. For now the only type of
	// externally visible symbol is a rule. For example:
	//
	// package a.b.c
	//
	// import data.e.f
	//
	// p = true :- q[x] = 1         # "p" is an export
	// q[x] :- f.r[x], not f.m[x]   # "q" is an export
	//
	// In this case, the mapping would be:
	//
	// {
	//   a.b.c: [p, q]
	// }
	Exports *util.HashMap

	// Globals contains a mapping of modules to globally accessible variables
	// within each module. Each variable is mapped to the value which represents
	// the fully qualified name of the variable. For example:
	//
	// package a.b.c
	//
	// import data.e.f
	//
	// p = true :- q[x] = 1
	// q[x] :- f.r[x], not f.m[x]
	//
	// In this case, the mapping would be
	//
	// {
	//  <modptr>: {q: data.a.b.c.q, f: data.e.f, p: data.a.b.q}
	// }
	Globals map[*Module]map[Var]Value
}

// NewCompiler returns a new empty compiler.
func NewCompiler() *Compiler {
	return &Compiler{
		Globals: map[*Module]map[Var]Value{},
	}
}

// Compile runs the compilation process on the input modules.
// The output of the compilation process can be obtained from
// the Errors or Modules attributes of the Compiler.
func (c *Compiler) Compile(mods map[string]*Module) {

	// TODO(tsandall): should the modules be deep copied?
	c.Modules = mods

	if c.setExports(); c.Failed() {
		return
	}

	if c.setGlobals(); c.Failed() {
		return
	}

	if c.resolveAllRefs(); c.Failed() {
		return
	}
}

// Failed returns true if a compilation error has been encountered.
func (c *Compiler) Failed() bool {
	return len(c.Errors) > 0
}

// FlattenErrors returns a single message that contains a flattened version of the compiler error messages.
// This must only be called when the compilation process has failed.
func (c *Compiler) FlattenErrors() string {

	if len(c.Errors) == 0 {
		panic(fmt.Sprintf("illegal call: %v", c))
	}

	if len(c.Errors) == 1 {
		return fmt.Sprintf("1 error occurred: %v", c.Errors[0].Error())
	}

	b := []string{}
	for _, err := range c.Errors {
		b = append(b, err.Error())
	}

	return fmt.Sprintf("%d errors occurred:\n%s", len(c.Errors), strings.Join(b, "\n"))
}

func (c *Compiler) err(f string, a ...interface{}) {
	err := fmt.Errorf(f, a...)
	c.Errors = append(c.Errors, err)
}

// resolveAllRefs resolves references in expressions to their fully qualified values.
//
// For instance, given the following module:
//
// package a.b
// import data.foo.bar
// p[x] :- bar[_] = x
//
// The reference "bar[_]" would be resolved to "data.foo.bar[_]".
func (c *Compiler) resolveAllRefs() {
	for _, m := range c.Modules {
		for _, rule := range m.Rules {
			for _, expr := range rule.Body {
				switch ts := expr.Terms.(type) {
				case *Term:
					expr.Terms = c.resolveRefs(c.Globals[m], ts)
				case []*Term:
					for i, t := range ts {
						ts[i] = c.resolveRefs(c.Globals[m], t)
					}
				}
			}
		}
	}
}

// setExports populates the Exports on the compiler.
// See Compiler for a description of Exports.
func (c *Compiler) setExports() {

	c.Exports = util.NewHashMap(func(a, b util.T) bool {
		r1 := a.(Ref)
		r2 := a.(Ref)
		return r1.Equal(r2)
	}, func(v util.T) int {
		return v.(Ref).Hash()
	})

	for _, mod := range c.Modules {
		for _, rule := range mod.Rules {
			v, ok := c.Exports.Get(mod.Package.Path)
			if !ok {
				v = []Var{}
			}
			vars := v.([]Var)
			vars = append(vars, rule.Name)
			c.Exports.Put(mod.Package.Path, vars)
		}
	}

}

// setGlobals populates the Globals on the compiler.
// See Compiler for a description of Globals.
func (c *Compiler) setGlobals() {

	for _, m := range c.Modules {

		p := m.Package.Path
		v, ok := c.Exports.Get(p)
		if !ok {
			continue
		}

		exports := v.([]Var)
		globals := map[Var]Value{}

		// Populate globals with exports within the package.
		for _, v := range exports {
			global := append(Ref{}, p...)
			global = append(global, &Term{Value: String(v)})
			globals[v] = global
		}

		// Populate globals with imports within this module.
		for _, i := range m.Imports {
			if len(i.Alias) > 0 {
				switch p := i.Path.Value.(type) {
				case Ref:
					globals[i.Alias] = p
				case Var:
					globals[i.Alias] = p
				default:
					c.err("unexpected %T: %v", p, i)
				}
			} else {
				switch p := i.Path.Value.(type) {
				case Ref:
					switch v := p[len(p)-1].Value.(type) {
					case String:
						globals[Var(v)] = p
					default:
						c.err("unexpected %T: %v", v, i)
					}
				case Var:
					globals[p] = p
				default:
					c.err("unexpected %T: %v", i.Path, i.Path)
				}
			}
		}

		c.Globals[m] = globals
	}
}

func (c *Compiler) resolveRefs(globals map[Var]Value, term *Term) *Term {
	switch v := term.Value.(type) {
	case Var:
		if r, ok := globals[v]; ok {
			cpy := *term
			cpy.Value = r
			return &cpy
		}
		return term
	case Ref:
		fqn := c.resolveRef(globals, v)
		cpy := *term
		cpy.Value = fqn
		return &cpy
	case Object:
		o := Object{}
		for _, i := range v {
			k := c.resolveRefs(globals, i[0])
			v := c.resolveRefs(globals, i[1])
			o = append(o, Item(k, v))
		}
		cpy := *term
		cpy.Value = o
		return &cpy
	case Array:
		a := Array{}
		for _, e := range v {
			x := c.resolveRefs(globals, e)
			a = append(a, x)
		}
		cpy := *term
		cpy.Value = a
		return &cpy
	default:
		return term
	}
}

func (c *Compiler) resolveRef(globals map[Var]Value, ref Ref) Ref {

	global := globals[ref[0].Value.(Var)]
	if global == nil {
		return ref
	}
	fqn := Ref{}
	switch global := global.(type) {
	case Ref:
		fqn = append(fqn, global...)
		for _, p := range ref[1:] {
			switch v := p.Value.(type) {
			case Var:
				global := globals[v]
				if global != nil {
					_, isRef := global.(Ref)
					if isRef {
						c.err("nested references in %v: %v => %v", ref, v, global)
						return ref
					}
					fqn = append(fqn, &Term{Location: p.Location, Value: global})
				} else {
					fqn = append(fqn, p)
				}
			default:
				fqn = append(fqn, p)
			}
		}
	case Var:
		fqn = append(fqn, &Term{Value: global})
		fqn = append(fqn, ref[1:]...)
	default:
		c.err("unexpected %T: %v", global, global)
		return ref
	}

	return fqn
}
