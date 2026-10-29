// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"maps"
	"slices"
)

// The rule index in three stages: translate a rule body into a condition, plan
// what of it the index tests, and execute that plan on a lookup. This file holds
// the first stage, and records conditions into the refindices the trie is built
// from (recordRule).
//
// The one property all of it rests on: for every rule and input, if the rule
// holds, the index returns it. Translation keeps it by only producing conditions
// the rule body implies; it may lose information, never add any.

// cond is one thing the index knows a rule body requires of its input: a
// *oneCond, *someCond, *lookupCond, *andCond or *anyCond. nil is nothing known.
type cond interface{ isCond() }

// oneCond: the value at ref matches value.
type oneCond struct {
	ref   Ref
	value condValue
}

// someCond: the value at ref matches one of values, all of one ref: `input.x in
// {"a", "b"}`, the bases of `strings.any_prefix_match`. shared is what the
// values have in common.
type someCond struct {
	ref    Ref
	shared condValue
	values []Value
}

// lookupCond: the value at the key is a key of one of the collections; see
// membership.
type lookupCond struct{ membership }

// andCond: every operand holds, `lhs and rhs`.
type andCond struct{ operands []condAll }

// anyCond: one operand holds, `lhs or rhs`, or an expression over a literal at
// one of its positions.
type anyCond struct{ operands []condAll }

func (*oneCond) isCond()    {}
func (*someCond) isCond()   {}
func (*lookupCond) isCond() {}
func (*andCond) isCond()    {}
func (*anyCond) isCond()    {}

// condAll is a body: it holds if every one of its conditions does. The empty one
// holds always.
type condAll []cond

// condValue is one value a ref is tested against: equal to it, holding it as an
// element (member), starting or ending with it (affix), matching it as a glob
// (mapper). A var for value means the ref only has to be defined.
type condValue struct {
	value  Value
	member bool
	affix  affix
	mapper *valueMapper
}

// slab hands out pointers to values it keeps in chunks, so that the conditions
// of a rule are not an allocation each. reset makes the last chunk reusable.
type slab[T any] struct{ free []T }

func (s *slab[T]) reset() {
	clear(s.free)
	s.free = s.free[:0]
}

func (s *slab[T]) new(v T) *T {
	if len(s.free) == cap(s.free) {
		s.free = make([]T, 0, min(max(2*cap(s.free), 4), 64))
	}
	s.free = append(s.free, v)
	return &s.free[len(s.free)-1]
}

func (t *translator) one(ref Ref, v condValue) cond {
	return t.ones.new(oneCond{ref: ref, value: v})
}

func (t *translator) some(ref Ref, vs []Value, shared condValue) cond {
	return t.somes.new(someCond{ref: ref, shared: shared, values: vs})
}

func (*translator) lookup(m membership) cond { return &lookupCond{m} }

// translateEnv is what translation knows about the locals of the body so far.
// An `and`/`or` operand is a scope of its own: it sees what the enclosing body
// knows, and what it learns stays inside (see translateOperand).
//
// A rule has few locals, and most bind none of these: the maps are made when
// first written to, and aliases, which most rules do have, are a list.
type translateEnv struct {
	constants map[Var]Value   // x := "a"
	aliases   []varRef        // x := input.foo
	domains   map[Var]binding // some x in {"a", "b"}
	elements  map[Var]Ref     // x := input.xs[_]
}

type varRef struct {
	v   Var
	ref Ref
}

func (e *translateEnv) clone() *translateEnv {
	return &translateEnv{
		constants: maps.Clone(e.constants),
		aliases:   slices.Clone(e.aliases),
		domains:   maps.Clone(e.domains),
		elements:  maps.Clone(e.elements),
	}
}

func (e *translateEnv) constant(v Var, value Value) {
	if e.constants == nil {
		e.constants = map[Var]Value{}
	}
	e.constants[v] = value
}

func (e *translateEnv) aliasOf(v Var) Ref {
	for _, a := range e.aliases {
		if a.v.Equal(v) {
			return a.ref
		}
	}
	return nil
}

func (e *translateEnv) alias(v Var, ref Ref) {
	if e.aliasOf(v) == nil {
		e.aliases = append(e.aliases, varRef{v: v, ref: ref})
	}
}

func (e *translateEnv) reset() {
	if len(e.constants) > 0 {
		clear(e.constants)
	}
	if len(e.aliases) > 0 {
		clear(e.aliases)
		e.aliases = e.aliases[:0]
	}
	if len(e.domains) > 0 {
		clear(e.domains)
	}
	if len(e.elements) > 0 {
		clear(e.elements)
	}
}

// translator keeps its environment and result across rules, so that most rules
// translate without allocating.
type translator struct {
	isVirtual func(Ref) bool
	rule      *Rule
	env       translateEnv
	buf       condAll
	ones      slab[oneCond]
	somes     slab[someCond]
}

// translate returns what rule requires of the input, valid until the next call.
func (t *translator) translate(rule *Rule) condAll {
	clear(t.buf)
	t.buf = t.buf[:0]
	t.ones.reset()
	t.somes.reset()
	if bodySkipsIndexing(rule.Body) {
		return t.buf
	}
	t.rule = rule
	t.env.reset()
	t.buf = t.body(rule.Body, &t.env, t.buf)
	return t.buf
}

func (t *translator) body(body Body, env *translateEnv, buf condAll) condAll {
	for _, expr := range body {
		if c := t.expr(expr, env); c != nil {
			buf = append(buf, c)
		}
	}
	return buf
}

// expr returns what expr requires, recording in env what it binds.
func (t *translator) expr(expr *Expr, env *translateEnv) cond {
	if len(expr.With) > 0 || expr.Negated {
		return nil
	}

	expr = plugRefs(expr, env.constants)

	if it, ok := t.iterationOf(expr, env); ok {
		return t.iteration(expr, it, env)
	}

	switch terms := expr.Terms.(type) {
	case *LogicalAnd:
		return &andCond{operands: []condAll{t.operand(terms.Lhs, env), t.operand(terms.Rhs, env)}}
	case *LogicalOr:
		return &anyCond{operands: []condAll{t.operand(terms.Lhs, env), t.operand(terms.Rhs, env)}}
	}

	op := expr.Operator()
	if op == nil {
		if ts, ok := expr.Terms.(*Term); ok {
			if ref, ok := ts.Value.(Ref); ok {
				if c := t.collectionKey(ref, env); c != nil {
					return c
				}
				return t.eq(ts.Value, anyValue, env)
			}
		}
		return nil
	}

	switch n := len(expr.Operands()); {
	case op.Equal(Interned.Refs.Equality), op.Equal(Interned.Refs.Equal) && n == 2:
		a, b := expr.Operand(0).Value, expr.Operand(1).Value
		if c := t.wildcardRef(a, b, env); c != nil {
			return c
		}
		if c := t.wildcardRef(b, a, env); c != nil {
			return c
		}
		return t.eq(a, b, env)
	case op.Equal(Interned.Refs.GlobMatch) && n == 3:
		return t.globMatch(expr, env)
	case op.Equal(Interned.Refs.Member) && n == 2:
		return t.member(expr, env)
	case op.Equal(Interned.Refs.StartsWith) && n == 2:
		return t.affix(expr, env, affixPrefix)
	case op.Equal(Interned.Refs.EndsWith) && n == 2:
		return t.affix(expr, env, affixSuffix)
	case op.Equal(Interned.Refs.AnyPrefixMatch) && n == 2:
		return t.anyAffix(expr, env, affixPrefix)
	case op.Equal(Interned.Refs.AnySuffixMatch) && n == 2:
		return t.anyAffix(expr, env, affixSuffix)
	}
	return nil
}

func (t *translator) operand(body Body, env *translateEnv) condAll {
	return t.body(body, env.clone(), nil)
}

// resolve returns the ref a local stands for, if it stands for one: one it was
// bound to, or a function argument.
func (t *translator) resolve(v Var, env *translateEnv) Ref {
	if ref := env.aliasOf(v); ref != nil {
		return ref
	}
	for j, arg := range t.rule.Head.Args {
		if v.Equal(arg.Value) {
			return Ref{FunctionArgRootDocument, InternedTerm(j)}
		}
	}
	return nil
}

// resolveHead returns ref with a local at its head replaced by the ref the local
// stands for, or nil if it stands for none.
func (t *translator) resolveHead(ref Ref, env *translateEnv) Ref {
	head, ok := ref[0].Value.(Var)
	if !ok || isRootDocument(head) {
		return ref
	}
	resolved := t.resolve(head, env)
	if resolved == nil {
		return nil
	}
	return resolved.Concat(ref[1:])
}

func (t *translator) validRef(ref Ref) bool {
	head, ok := ref[0].Value.(Var)
	return ok && isRootDocument(head) && !ref.IsNested() && ref.IsGround() && !t.isVirtual(ref)
}

// isRootDocument is RootDocumentNames.Contains without hashing, which shows on
// index build.
func isRootDocument(v Var) bool {
	return v == "data" || v == "input"
}

// refOf returns the valid ref term is, or stands for.
func (t *translator) refOf(term *Term, env *translateEnv) Ref {
	var ref Ref
	switch v := term.Value.(type) {
	case Ref:
		ref = v
	case Var:
		ref = t.resolve(v, env)
	}
	if ref == nil || !t.validRef(ref) {
		return nil
	}
	return ref
}

// eq is `a = b` with neither side an iteration over a ref: a ref equal to a
// value, or a local bound to something. Tried both ways round.
func (t *translator) eq(a, b Value, env *translateEnv) cond {
	if c, ok := t.eqOneWay(a, b, env); ok {
		return c
	}
	c, _ := t.eqOneWay(b, a, env)
	return c
}

func (t *translator) eqOneWay(a, b Value, env *translateEnv) (cond, bool) {
	switch v := a.(type) {
	case Var:
		if IsConstant(b) {
			env.constant(v, b)
		} else {
			if bv, ok := b.(Var); ok {
				// `y := x` binds y to whatever x is bound to.
				if c, ok := env.constants[bv]; ok {
					env.constant(v, c)
				}
				if d, ok := env.domains[bv]; ok {
					t.bind(env, v, d)
				}
				if coll, ok := env.elements[bv]; ok {
					t.element(env, v, coll)
				}
			}
			t.recordIteration(v, b, env)
			t.recordElement(v, b, env)
		}

		bval, ok := indexValue(b)
		if !ok {
			return nil, false
		}
		if ref := t.resolve(v, env); ref != nil {
			if bv, ok := bval.(Var); ok {
				env.alias(bv, ref)
			}
			return t.one(ref, condValue{value: bval}), true
		}
		if coll, ok := env.elements[v]; ok && IsScalar(bval) {
			return t.one(coll, condValue{value: bval, member: true}), true
		}

	case Ref:
		ref := t.resolveHead(v, env)
		if ref == nil || !t.validRef(ref) {
			return nil, false
		}
		if bv, ok := b.(Var); ok {
			if c, ok := env.constants[bv]; ok {
				b = c
			}
		} else if bval, ok := indexValue(b); ok {
			b = bval
		} else {
			return nil, false
		}
		if bv, ok := b.(Var); ok {
			env.alias(bv, ref)
		}
		return t.one(ref, condValue{value: b}), true
	}
	return nil, false
}

// wildcardRef is `input.x[_] == "a"`: the collection at input.x holds "a".
func (t *translator) wildcardRef(a, b Value, env *translateEnv) cond {
	ref, ok := a.(Ref)
	if !ok {
		return nil
	}
	ref = t.resolveHead(ref, env)
	if ref == nil {
		return nil
	}
	prefix := ref.GroundPrefix()
	if len(prefix) != len(ref)-1 || !t.validRef(prefix) {
		return nil
	}

	value := b
	if bv, ok := b.(Var); ok {
		if c, ok := env.constants[bv]; ok {
			value = c
		}
	} else if v, ok := indexValue(b); ok {
		value = v
	} else {
		return nil
	}
	if !IsScalar(value) {
		return nil
	}
	return t.one(prefix, condValue{value: value, member: true})
}

func (t *translator) globMatch(expr *Expr, env *translateEnv) cond {
	delim, ok := globDelimiterToString(expr.Operand(1))
	if !ok {
		return nil
	}
	arr := globPatternToArray(expr.Operand(0), delim)
	if arr == nil {
		return nil
	}
	v, ok := expr.Operand(2).Value.(Var)
	if !ok {
		return nil
	}
	ref := t.resolve(v, env)
	if ref == nil {
		return nil
	}
	return t.one(ref, condValue{value: arr.Value, mapper: &valueMapper{
		Key: delim,
		MapValue: func(v Value) Value {
			if s, ok := v.(String); ok {
				return stringSliceToArray(splitStringEscaped(string(s), delim))
			}
			return v
		},
	}})
}

// member is `lhs in rhs`: a ref in a literal collection, or a value in the
// collection at a ref.
func (t *translator) member(expr *Expr, env *translateEnv) cond {
	lhs, rhs := expr.Operand(0), expr.Operand(1)
	if lv, ok := lhs.Value.(Var); ok {
		if lref := t.resolve(lv, env); lref != nil {
			return t.refInValue(lref, rhs, env)
		}
	}

	lval := lhs.Value
	if lv, ok := lval.(Var); ok {
		if c, ok := env.constants[lv]; ok {
			lval = c
		}
	}
	if _, ok := lval.(Var); !ok && !IsScalar(lval) {
		return nil
	}
	rref := t.refOf(rhs, env)
	if rref == nil {
		return nil
	}
	return t.one(rref, condValue{value: lval, member: !valueIsVar(lval)})
}

func (t *translator) refInValue(ref Ref, rhs *Term, env *translateEnv) cond {
	rval := rhs.Value
	if rv, ok := rval.(Var); ok {
		if c, ok := env.constants[rv]; ok {
			rval = c
		} else if cref := t.resolve(rv, env); cref != nil {
			rval = cref
		}
	}

	var members []Value
	switch col := rval.(type) {
	case Ref:
		// The collection's values, which base data only answers by being walked.
		return nil
	case *Array:
		members = make([]Value, 0, col.Len())
		col.Foreach(func(t *Term) { members = append(members, t.Value) })
	case Set:
		members = make([]Value, 0, col.Len())
		col.Foreach(func(t *Term) { members = append(members, t.Value) })
	case Object:
		members = make([]Value, 0, col.Len())
		if o, ok := col.(*object); ok {
			for _, node := range o.sortedKeys() {
				members = append(members, node.value.Value)
			}
		} else {
			col.Foreach(func(_, v *Term) { members = append(members, v.Value) })
		}
	default:
		return nil
	}
	if len(members) == 0 {
		return nil
	}
	// A var member lets the ref be anything, but the index would drop it in
	// favour of the constant members, taking a path's values to all hold.
	if slices.ContainsFunc(members, valueIsVar) {
		return nil
	}

	return t.some(ref, members, condValue{})
}

func (t *translator) affix(expr *Expr, env *translateEnv, a affix) cond {
	ref := t.refOf(expr.Operand(0), env)
	if ref == nil {
		return nil
	}
	base, ok := constantString(expr.Operand(1), env.constants)
	if !ok {
		return nil
	}
	return t.one(ref, condValue{value: base, affix: a})
}

func (t *translator) anyAffix(expr *Expr, env *translateEnv, a affix) cond {
	ref := t.refOf(expr.Operand(0), env)
	if ref == nil {
		return nil
	}
	base := expr.Operand(1).Value
	if v, ok := base.(Var); ok {
		c, ok := env.constants[v]
		if !ok {
			return nil
		}
		base = c
	}
	if s, ok := base.(String); ok {
		return t.one(ref, condValue{value: s, affix: a})
	}
	bases, ok := groundStrings(base)
	if !ok || len(bases) == 0 {
		return nil
	}
	return t.some(ref, bases, condValue{affix: a})
}

// collectionKey is `data.groups.g1.members[input.subject]`: the key has to be a
// key of the collection.
func (t *translator) collectionKey(ref Ref, env *translateEnv) cond {
	if len(ref) < 2 || !ref[0].Equal(DefaultRootDocument) {
		return nil
	}
	prefix := ref[:len(ref)-1]
	if !prefix.IsGround() || t.isVirtual(prefix) {
		return nil
	}

	var key Ref
	var each bool
	switch v := ref[len(ref)-1].Value.(type) {
	case Ref:
		if v.IsGround() && !t.isVirtual(v) {
			key = v
		}
	case Var:
		if coll, ok := env.elements[v]; ok {
			key, each = coll, true
		} else {
			key = t.resolve(v, env)
		}
	}
	if key == nil || t.isVirtual(key) {
		return nil
	}
	return t.lookup(membership{key: key, collections: []Ref{prefix}, each: each})
}

// recordIteration notes that v and the key next to it iterate a literal, if src
// is an iteration over one: `v = c[k]`, c being bound to the literal.
func (t *translator) recordIteration(v Var, src Value, env *translateEnv) {
	ref, ok := src.(Ref)
	if !ok || len(ref) != 2 {
		return
	}
	k, ok := ref[1].Value.(Var)
	if !ok {
		return
	}
	coll := ref[0].Value
	if head, ok := coll.(Var); ok {
		coll = env.constants[head]
	}

	it := &iteration{}
	switch c := coll.(type) {
	case Set:
		it.keys, it.values = make([]Value, 0, c.Len()), make([]Value, 0, c.Len())
		c.Foreach(func(t *Term) { it.keys, it.values = append(it.keys, t.Value), append(it.values, t.Value) })
	case *Array:
		it.keys, it.values = make([]Value, 0, c.Len()), make([]Value, 0, c.Len())
		c.Foreach(func(t *Term) {
			it.keys, it.values = append(it.keys, IntNumberTerm(len(it.keys)).Value), append(it.values, t.Value)
		})
	case Object:
		it.keys, it.values = make([]Value, 0, c.Len()), make([]Value, 0, c.Len())
		c.Foreach(func(key, val *Term) { it.keys, it.values = append(it.keys, key.Value), append(it.values, val.Value) })
	default:
		return
	}

	scalars := func(vs []Value) bool { return !slices.ContainsFunc(vs, func(e Value) bool { return !IsScalar(e) }) }
	if len(it.keys) == 0 || !scalars(it.keys) {
		return
	}
	t.bind(env, k, binding{it: it, key: true})
	if scalars(it.values) {
		t.bind(env, v, binding{it: it})
	}
}

func (*translator) bind(env *translateEnv, v Var, b binding) {
	if env.domains == nil {
		env.domains = map[Var]binding{}
	}
	env.domains[v] = b
}

// recordElement notes that v is bound to an element of the collection at a
// reference, if src is an iteration over one: `v = input.subjects[_]`.
func (t *translator) recordElement(v Var, src Value, env *translateEnv) {
	ref, ok := src.(Ref)
	if !ok || len(ref) < 2 || !valueIsVar(ref[len(ref)-1].Value) {
		return
	}
	coll := t.resolveHead(ref[:len(ref)-1], env)
	if coll == nil || !t.validRef(coll) {
		return
	}
	t.element(env, v, coll)
}

func (*translator) element(env *translateEnv, v Var, coll Ref) {
	if env.elements == nil {
		env.elements = map[Var]Ref{}
	}
	env.elements[v] = coll
}

// iterationOf returns an iteration a local of expr takes part in, provided the
// combinations of all iterations expr uses are few enough to translate one by
// one. `y = x` is recorded rather than expanded.
func (*translator) iterationOf(expr *Expr, env *translateEnv) (*iteration, bool) {
	if len(env.domains) == 0 || isLocalAlias(expr) {
		return nil, false
	}
	var its []*iteration
	product := 1
	WalkVars(expr, func(v Var) bool {
		if b, ok := env.domains[v]; ok && !slices.Contains(its, b.it) {
			its = append(its, b.it)
			product = min(product*len(b.it.keys), maxDomainExpansion+1)
		}
		return false
	})
	if len(its) == 0 || product > maxDomainExpansion {
		return nil, false
	}
	return its[0], true
}

// iteration is expr at each position of it, every local bound to it replaced
// by its key or value there: `some x in {"a", "b"}; input.y == x` as
// `{input.y == "a"} or {input.y == "b"}`.
func (t *translator) iteration(expr *Expr, it *iteration, env *translateEnv) cond {
	positions := make([]condAll, 0, len(it.keys))
	for pos := range it.keys {
		scoped := env.clone()
		maps.DeleteFunc(scoped.domains, func(_ Var, b binding) bool { return b.it == it })
		for v, b := range env.domains {
			if b.it == it {
				scoped.constant(v, b.at(pos))
			}
		}
		var body condAll
		if c := t.expr(expr, scoped); c != nil {
			body = condAll{c}
		}
		positions = append(positions, body)
	}
	return &anyCond{operands: positions}
}

// projection is what an `and`/`or` operand, or a position of a literal
// iteration, comes to: the ways to reach the rule (each a list of values per
// ref, several values for one ref being alternatives), and the collection
// lookups checked on it. recordCond hands it on to the rule's own records.
type projection struct {
	unconditional []projected
	disjunctions  [][][]projected
	memberships   []membership
}

type projected struct {
	ref Ref
	condValue
}

func (p projected) isVar() bool { return valueIsVar(p.value) }

func (p *projection) body(body condAll) {
	for _, c := range body {
		p.expr(c)
	}
}

func (p *projection) expr(c cond) {
	switch c := c.(type) {
	case *oneCond:
		p.insert(projected{ref: c.ref, condValue: c.value})
	case *someCond:
		for _, v := range c.values {
			cv := c.shared
			cv.value = v
			p.insert(projected{ref: c.ref, condValue: cv})
		}
	case *lookupCond:
		p.recordMembership(c.membership)
	case *andCond:
		for _, operand := range c.operands {
			op := projectOperand(operand)
			p.require(op.alts)
			for _, m := range op.memberships {
				p.recordMembership(m)
			}
		}
	case *anyCond:
		ops := make([]projectedOperand, len(c.operands))
		for j, operand := range c.operands {
			ops[j] = projectOperand(operand)
		}
		p.disjoin(ops)
	}
}

type projectedOperand struct {
	alts        [][]projected
	memberships []membership
}

func projectOperand(body condAll) projectedOperand {
	var sub projection
	sub.body(body)
	op := projectedOperand{memberships: sub.memberships}
	alts := sub.paths()
	if len(alts) == 1 && len(alts[0]) == 0 {
		return op
	}
	for _, alt := range alts {
		for j, v := range alt {
			// A local of the operand does not reach outside it; that the ref
			// has to be defined does.
			if v.isVar() {
				alt[j] = projected{ref: v.ref, condValue: condValue{value: anyValue, mapper: v.mapper}}
			}
		}
	}
	op.alts = alts
	return op
}

// insert adds v to the unconditional values, as refindices.insert does: once,
// and in place of a "defined" entry for its ref if it is a value to equal.
func (p *projection) insert(v projected) {
	for j, other := range p.unconditional {
		if !RefEqual(other.ref, v.ref) {
			continue
		}
		if other.affix == v.affix && other.member == v.member && ValueEqual(other.value, v.value) {
			return
		}
		if !v.isVar() && v.affix == affixNone && other.isVar() {
			p.unconditional[j] = v
			return
		}
	}
	p.unconditional = append(p.unconditional, v)
}

func (p *projection) require(alts [][]projected) {
	switch len(alts) {
	case 0:
	case 1:
		for _, v := range alts[0] {
			p.insert(v)
		}
	default:
		p.disjunctions = append(p.disjunctions, alts)
	}
}

func (p *projection) disjoin(ops []projectedOperand) {
	if !slices.ContainsFunc(ops, func(op projectedOperand) bool { return len(op.alts) == 0 }) {
		var alts [][]projected
		for _, op := range ops {
			alts = append(alts, op.alts...)
		}
		p.require(alts)
	}

	for _, m := range ops[0].memberships {
		merged := membership{key: m.key, collections: slices.Clone(m.collections), each: m.each}
		for _, op := range ops[1:] {
			j := slices.IndexFunc(op.memberships, func(o membership) bool { return RefEqual(o.key, m.key) && o.each == m.each })
			if j < 0 {
				merged.collections = nil
				break
			}
			merged.collections = append(merged.collections, op.memberships[j].collections...)
		}
		if merged.collections != nil {
			p.recordMembership(merged)
		}
	}
}

func (p *projection) recordMembership(m membership) {
	for _, other := range p.memberships {
		if RefEqual(other.key, m.key) && other.each == m.each && slices.EqualFunc(other.collections, m.collections, RefEqual) {
			return
		}
	}
	p.memberships = append(p.memberships, m)
}

func (p *projection) paths() [][]projected {
	paths := [][]projected{p.unconditional}
	for _, alts := range p.disjunctions {
		if len(paths)*len(alts) > maxIndexPaths {
			return [][]projected{p.unconditional}
		}
		combined := make([][]projected, 0, len(paths)*len(alts))
		for _, path := range paths {
			for _, alt := range alts {
				combined = append(combined, append(slices.Clone(path), alt...))
			}
		}
		paths = combined
	}
	return paths
}

// recordRule records what rule requires of the input into the indices the trie
// is built from.
func (i *refindices) recordRule(rule *Rule) {
	i.recordBody(rule, i.translator.translate(rule))
}

func (i *refindices) recordBody(rule *Rule, body condAll) {
	for _, c := range body {
		i.recordCond(rule, c)
	}
}

func (i *refindices) recordCond(rule *Rule, c cond) {
	switch c := c.(type) {
	case *oneCond:
		i.insert(rule, i.refindexOf(c.ref, c.value))
	case *someCond:
		i.recordValues(rule, c)
	case *lookupCond:
		i.recordMembership(rule, c.membership)
	case *andCond:
		for _, operand := range c.operands {
			op := projectOperand(operand)
			i.require(rule, i.refindicesOf(op.alts))
			for _, m := range op.memberships {
				i.recordMembership(rule, m)
			}
		}
	case *anyCond:
		ops := make([]operand, len(c.operands))
		for j, body := range c.operands {
			op := projectOperand(body)
			ops[j] = operand{alts: i.refindicesOf(op.alts), memberships: op.memberships}
		}
		i.disjoin(rule, ops)
	}
}

// recordValues records the values one ref is tested against. Several of them,
// as an `in` or a base collection has, go in at once, which is also what weighs
// them as alternatives in the statistics the trie is ordered by.
func (i *refindices) recordValues(rule *Rule, c *someCond) {
	if len(c.values) == 1 {
		v := c.shared
		v.value = c.values[0]
		i.insert(rule, i.refindexOf(c.ref, v))
		return
	}
	if a := c.shared.affix; a != affixNone {
		i.insertAffixes(rule, c.ref, c.values, a)
		return
	}
	i.insertMembers(rule, c.ref, c.values)
}

func (i *refindices) refindexOf(ref Ref, v condValue) *refindex {
	return &refindex{ref: i.table.intern(ref), Value: v.value, Member: v.member, Affix: v.affix, Mapper: v.mapper}
}

func (i *refindices) refindicesOf(alts [][]projected) alternatives {
	if alts == nil {
		return nil
	}
	out := make(alternatives, len(alts))
	for j, alt := range alts {
		out[j] = make([]*refindex, len(alt))
		for k, v := range alt {
			out[j][k] = i.refindexOf(v.ref, v.condValue)
		}
	}
	return out
}
