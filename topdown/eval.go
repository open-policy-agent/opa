package topdown

import (
	"context"
	"fmt"
	"sort"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/topdown/copypropagation"
)

type evalIterator func(*eval) error

type unifyIterator func() error

type queryIDFactory struct {
	curr uint64
}

// Note: The first call to Next() returns 0.
func (f *queryIDFactory) Next() uint64 {
	defer func() { f.curr++ }()
	return f.curr
}

type eval struct {
	ctx           context.Context
	queryID       uint64
	queryIDFact   *queryIDFactory
	parent        *eval
	cancel        Cancel
	query         ast.Body
	index         int
	bindings      *bindings
	store         storage.Store
	baseCache     *baseCache
	withCache     *baseCache
	txn           storage.Transaction
	compiler      *ast.Compiler
	input         *ast.Term
	tracers       []Tracer
	instr         *Instrumentation
	builtinCache  builtins.Cache
	virtualCache  *virtualCache
	saveSet       *saveSet
	saveStack     *saveStack
	saveSupport   *saveSupport
	saveNamespace *ast.Term
	genvarprefix  string
	runtime       *ast.Term
}

func (e *eval) Run(iter evalIterator) error {
	e.traceEnter(e.query)
	return e.eval(func(e *eval) error {
		e.traceExit(e.query)
		err := iter(e)
		e.traceRedo(e.query)
		return err
	})
}

func (e *eval) closure(query ast.Body) *eval {
	cpy := *e
	cpy.index = 0
	cpy.query = query
	cpy.queryID = cpy.queryIDFact.Next()
	cpy.parent = e
	return &cpy
}

func (e *eval) child(query ast.Body) *eval {
	cpy := *e
	cpy.index = 0
	cpy.query = query
	cpy.queryID = cpy.queryIDFact.Next()
	cpy.bindings = newBindings(cpy.queryID, e.instr)
	cpy.parent = e
	return &cpy
}

func (e *eval) next(iter evalIterator) error {
	e.index++
	err := e.evalExpr(iter)
	e.index--
	return err
}

func (e *eval) partial() bool {
	return e.saveSet != nil
}

func (e *eval) traceEnter(x ast.Node) {
	e.traceEvent(EnterOp, x, "")
}

func (e *eval) traceExit(x ast.Node) {
	e.traceEvent(ExitOp, x, "")
}

func (e *eval) traceEval(x ast.Node) {
	e.traceEvent(EvalOp, x, "")
}

func (e *eval) traceFail(x ast.Node) {
	e.traceEvent(FailOp, x, "")
}

func (e *eval) traceRedo(x ast.Node) {
	e.traceEvent(RedoOp, x, "")
}

func (e *eval) traceSave(x ast.Node) {
	e.traceEvent(SaveOp, x, "")
}

func (e *eval) traceIndex(x ast.Node, msg string) {
	e.traceEvent(IndexOp, x, msg)
}

func (e *eval) traceEvent(op Op, x ast.Node, msg string) {

	if !traceIsEnabled(e.tracers) {
		return
	}

	locals := ast.NewValueMap()
	e.bindings.Iter(nil, func(k, v *ast.Term) error {
		locals.Put(k.Value, v.Value)
		return nil
	})

	var parentID uint64
	if e.parent != nil {
		parentID = e.parent.queryID
	}

	evt := &Event{
		QueryID:  e.queryID,
		ParentID: parentID,
		Op:       op,
		Node:     x,
		Locals:   locals,
		Message:  msg,
	}

	for i := range e.tracers {
		if e.tracers[i].Enabled() {
			e.tracers[i].Trace(evt)
		}
	}
}

func (e *eval) eval(iter evalIterator) error {
	return e.evalExpr(iter)
}

func (e *eval) evalExpr(iter evalIterator) error {

	if e.cancel != nil && e.cancel.Cancelled() {
		return &Error{
			Code:    CancelErr,
			Message: "caller cancelled query execution",
		}
	}

	if e.index >= len(e.query) {
		return iter(e)
	}

	expr := e.query[e.index]
	e.traceEval(expr)

	if len(expr.With) > 0 {
		if e.partial() {
			return e.saveExpr(expr, e.bindings, func() error {
				return e.next(iter)
			})
		}
		return e.evalWith(iter)
	}

	return e.evalStep(iter)
}

func (e *eval) evalStep(iter evalIterator) error {

	expr := e.query[e.index]

	if expr.Negated {
		return e.evalNot(iter)
	}

	var defined bool
	var err error

	switch terms := expr.Terms.(type) {
	case []*ast.Term:
		if expr.IsEquality() {
			err = e.unify(terms[1], terms[2], func() error {
				defined = true
				err := e.next(iter)
				e.traceRedo(expr)
				return err
			})
		} else {
			err = e.evalCall(terms, func() error {
				defined = true
				err := e.next(iter)
				e.traceRedo(expr)
				return err
			})
		}
	case *ast.Term:
		rterm := e.generateVar(fmt.Sprintf("term_%d_%d", e.queryID, e.index))
		err = e.unify(terms, rterm, func() error {
			if e.saveSet.Contains(rterm, e.bindings) {
				return e.saveExpr(ast.NewExpr(rterm), e.bindings, func() error {
					return e.next(iter)
				})
			}
			if !e.bindings.Plug(rterm).Equal(ast.BooleanTerm(false)) {
				defined = true
				err := e.next(iter)
				e.traceRedo(expr)
				return err
			}
			return nil
		})
	}

	if err != nil {
		return err
	}

	if !defined {
		e.traceFail(expr)
	}

	return nil
}

func (e *eval) evalNot(iter evalIterator) error {

	if e.partial() {
		return e.evalNotPartial(iter)
	}

	expr := e.query[e.index]
	negation := expr.Complement().NoWith()
	child := e.closure(ast.NewBody(negation))

	var defined bool

	err := child.eval(func(*eval) error {
		defined = true
		return nil
	})

	if err != nil {
		return err
	}

	if !defined {
		return e.next(iter)
	}

	e.traceFail(expr)
	return nil
}

func (e *eval) evalWith(iter evalIterator) error {
	expr := e.query[e.index]
	pairsInput := [][2]*ast.Term{}
	pairsData := [][2]*ast.Term{}

	for i := range expr.With {
		plugged := e.bindings.Plug(expr.With[i].Value)
		if isInputRef(expr.With[i].Target) {
			pairsInput = append(pairsInput, [...]*ast.Term{expr.With[i].Target, plugged})
		} else if isDataRef(expr.With[i].Target) {
			pairsData = append(pairsData, [...]*ast.Term{expr.With[i].Target, plugged})
		}
	}

	input, err := makeInput(pairsInput)
	if err != nil {
		return &Error{
			Code:     ConflictErr,
			Location: expr.Location,
			Message:  err.Error(),
		}
	}

	for _, pair := range pairsData {
		ref := pair[0].Value.(ast.Ref)
		e.withCache.Put(ref, pair[1].Value)
	}

	var old *ast.Term

	if input != nil {
		old = e.input
		e.input = ast.NewTerm(input)
	}

	e.virtualCache.Push()
	err = e.evalStep(iter)
	e.virtualCache.Pop()

	if input != nil {
		e.input = old
	}

	for _, pair := range pairsData {
		ref := pair[0].Value.(ast.Ref)
		e.withCache.Remove(ref)
	}

	return err
}

func (e *eval) evalNotPartial(iter evalIterator) error {

	// Prepare query normally.
	expr := e.query[e.index]
	negation := expr.Complement().NoWith()
	child := e.closure(ast.NewBody(negation))

	var caller *bindings

	if e.parent == nil {
		// The top-level query is being evaluated. Do not namespace variables from this
		// query.
		caller = e.bindings
	} else {
		// The top-level query is NOT being evaluated. All variables in the queries
		// that are emitted by partial evaluation of this query MUST be namespaced. A
		// sentinel is used because the plug operations do not namespace if the caller
		// is nil.
		caller = sentinel
	}

	// Unknowns is the set of variables that are marked as unknown. The variables
	// are namespaced with the query ID that they originate in. This ensures that
	// variables across two or more queries are identified uniquely.
	//
	// NOTE(tsandall): this is greedy in the sense that we only need variable
	// dependencies of the negation.
	unknowns := e.saveSet.Vars(caller)

	// Run partial evaluation, plugging the result and applying copy propagation to
	// each result. Since the result may require support, push a new query onto the
	// save stack to avoid mutating the current save query.
	p := copypropagation.New(unknowns).WithEnsureNonEmptyBody(true)
	var savedQueries []ast.Body
	e.saveStack.PushQuery(nil)

	child.eval(func(*eval) error {
		query := e.saveStack.Peek()
		plugged := query.Plug(caller)
		result := p.Apply(plugged)
		savedQueries = append(savedQueries, result)
		return nil
	})

	e.saveStack.PopQuery()

	// If partial evaluation produced no results, the expression is always undefined
	// so it does not have to be saved.
	if len(savedQueries) == 0 {
		return e.next(iter)
	}

	// Check if the partial evaluation result can be inlined in this query. If not,
	// generate support rules for the result. Depending on the size of the partial
	// evaluation result and the contents, it may or may not be inlinable. We treat
	// the unknowns as safe because vars in the save set will either be known to
	// the caller or made safe by an expression on the save stack.
	if !canInlineNegation(unknowns, savedQueries) {
		return e.evalNotPartialSupport(expr, unknowns, savedQueries, iter)
	}

	// If we can inline the result, we have to generate the cross product of the
	// queries. For example:
	//
	//	(A && B) || (C && D)
	//
	// Becomes:
	//
	//	(!A && !C) || (!A && !D) || (!B && !C) || (!B && !D)
	return complementedCartesianProduct(savedQueries, 0, nil, func(q ast.Body) error {
		return e.savePluggedExprs(q, func() error {
			return e.next(iter)
		})
	})
}

func (e *eval) evalNotPartialSupport(expr *ast.Expr, unknowns ast.VarSet, queries []ast.Body, iter evalIterator) error {

	// Prepare support rule head.
	supportName := fmt.Sprintf("__not%d_%d__", e.queryID, e.index)
	term := ast.RefTerm(ast.DefaultRootDocument, e.saveNamespace, ast.StringTerm(supportName))
	path := term.Value.(ast.Ref)
	head := ast.NewHead(ast.Var(supportName), nil, ast.BooleanTerm(true))

	bodyVars := ast.NewVarSet()

	for _, q := range queries {
		bodyVars.Update(q.Vars(ast.VarVisitorParams{}))
	}

	unknowns = unknowns.Intersect(bodyVars)

	// Make rule args. Sort them to ensure order is deterministic.
	args := make([]*ast.Term, 0, len(unknowns))

	for v := range unknowns {
		args = append(args, ast.NewTerm(v))
	}

	sort.Slice(args, func(i, j int) bool {
		return args[i].Value.Compare(args[j].Value) < 0
	})

	if len(args) > 0 {
		head.Args = ast.Args(args)
	}

	// Save support rules.
	for _, query := range queries {
		e.saveSupport.Insert(path, &ast.Rule{
			Head: head,
			Body: query,
		})
	}

	// Save expression that refers to support rule set.
	expr = expr.Copy()
	if len(args) > 0 {
		terms := make([]*ast.Term, len(args)+1)
		terms[0] = term
		for i := 0; i < len(args); i++ {
			terms[i+1] = args[i]
		}
		expr.Terms = terms
	} else {
		expr.Terms = term
	}

	return e.savePluggedExprs([]*ast.Expr{expr}, func() error {
		return e.next(iter)
	})
}

func (e *eval) evalCall(terms []*ast.Term, iter unifyIterator) error {

	ref := terms[0].Value.(ast.Ref)

	if ref[0].Equal(ast.DefaultRootDocument) {
		eval := evalFunc{
			e:     e,
			ref:   ref,
			terms: terms,
		}
		return eval.eval(iter)
	}

	bi := ast.BuiltinMap[ref.String()]
	if bi == nil {
		return unsupportedBuiltinErr(e.query[e.index].Location)
	}

	if e.partial() {
		// Check if call can be evaluated during partial eval. Some
		// calls, such as time.now_ns() should not be evaluated during
		// partial evaluation.
		mustSave := false
		for _, ignore := range ast.IgnoreDuringPartialEval {
			if bi == ignore {
				mustSave = true
				break
			}
		}
		if !mustSave {
			for i := 1; i < len(terms); i++ {
				if e.saveSet.ContainsRecursive(terms[i], e.bindings) {
					mustSave = true
					break
				}
			}
		}
		if mustSave {
			return e.saveCall(len(bi.Decl.Args()), terms, iter)
		}
	}

	f := builtinFunctions[bi.Name]
	if f == nil {
		return unsupportedBuiltinErr(e.query[e.index].Location)
	}

	var parentID uint64
	if e.parent != nil {
		parentID = e.parent.queryID
	}

	bctx := BuiltinContext{
		Runtime:  e.runtime,
		Cache:    e.builtinCache,
		Location: e.query[e.index].Location,
		Tracers:  e.tracers,
		QueryID:  e.queryID,
		ParentID: parentID,
	}

	eval := evalBuiltin{
		e:     e,
		bi:    bi,
		bctx:  bctx,
		f:     f,
		terms: terms[1:],
	}
	return eval.eval(iter)
}

func (e *eval) unify(a, b *ast.Term, iter unifyIterator) error {
	return e.biunify(a, b, e.bindings, e.bindings, iter)
}

func (e *eval) biunify(a, b *ast.Term, b1, b2 *bindings, iter unifyIterator) error {
	a, b1 = b1.apply(a)
	b, b2 = b2.apply(b)
	switch vA := a.Value.(type) {
	case ast.Var, ast.Ref, *ast.ArrayComprehension, *ast.SetComprehension, *ast.ObjectComprehension:
		return e.biunifyValues(a, b, b1, b2, iter)
	case ast.Null:
		switch b.Value.(type) {
		case ast.Var, ast.Null, ast.Ref:
			return e.biunifyValues(a, b, b1, b2, iter)
		}
	case ast.Boolean:
		switch b.Value.(type) {
		case ast.Var, ast.Boolean, ast.Ref:
			return e.biunifyValues(a, b, b1, b2, iter)
		}
	case ast.Number:
		switch b.Value.(type) {
		case ast.Var, ast.Number, ast.Ref:
			return e.biunifyValues(a, b, b1, b2, iter)
		}
	case ast.String:
		switch b.Value.(type) {
		case ast.Var, ast.String, ast.Ref:
			return e.biunifyValues(a, b, b1, b2, iter)
		}
	case ast.Array:
		switch vB := b.Value.(type) {
		case ast.Var, ast.Ref, *ast.ArrayComprehension:
			return e.biunifyValues(a, b, b1, b2, iter)
		case ast.Array:
			return e.biunifyArrays(vA, vB, b1, b2, iter)
		}
	case ast.Object:
		switch vB := b.Value.(type) {
		case ast.Var, ast.Ref, *ast.ObjectComprehension:
			return e.biunifyValues(a, b, b1, b2, iter)
		case ast.Object:
			return e.biunifyObjects(vA, vB, b1, b2, iter)
		}
	case ast.Set:
		return e.biunifyValues(a, b, b1, b2, iter)
	}
	return nil
}

func (e *eval) biunifyArrays(a, b ast.Array, b1, b2 *bindings, iter unifyIterator) error {
	if len(a) != len(b) {
		return nil
	}
	return e.biunifyArraysRec(a, b, b1, b2, iter, 0)
}

func (e *eval) biunifyArraysRec(a, b ast.Array, b1, b2 *bindings, iter unifyIterator, idx int) error {
	if idx == len(a) {
		return iter()
	}
	return e.biunify(a[idx], b[idx], b1, b2, func() error {
		return e.biunifyArraysRec(a, b, b1, b2, iter, idx+1)
	})
}

func (e *eval) biunifyObjects(a, b ast.Object, b1, b2 *bindings, iter unifyIterator) error {
	if a.Len() != b.Len() {
		return nil
	}

	// Objects must not contain unbound variables as keys at this point as we
	// cannot unify them. Similar to sets, plug both sides before comparing the
	// keys and unifying the values.
	if nonGroundKeys(a) {
		a = plugKeys(a, b1)
	}

	if nonGroundKeys(b) {
		b = plugKeys(b, b2)
	}

	return e.biunifyObjectsRec(a, b, b1, b2, iter, a.Keys(), 0)
}

func (e *eval) biunifyObjectsRec(a, b ast.Object, b1, b2 *bindings, iter unifyIterator, keys []*ast.Term, idx int) error {
	if idx == len(keys) {
		return iter()
	}
	v2 := b.Get(keys[idx])
	if v2 == nil {
		return nil
	}
	return e.biunify(a.Get(keys[idx]), v2, b1, b2, func() error {
		return e.biunifyObjectsRec(a, b, b1, b2, iter, keys, idx+1)
	})
}

func (e *eval) biunifyValues(a, b *ast.Term, b1, b2 *bindings, iter unifyIterator) error {

	saveA := e.saveSet.Contains(a, b1)
	saveB := e.saveSet.Contains(b, b2)
	_, refA := a.Value.(ast.Ref)
	_, refB := b.Value.(ast.Ref)

	// Try to evaluate refs and comprehensions. If partial evaluation is
	// enabled, then skip evaluation (and save the expression) if the term is
	// in the save set. Currently, comprehensions are not evaluated during
	// partial eval. This could be improved in the future.
	if refA && !saveA {
		return e.biunifyRef(a, b, b1, b2, iter)
	} else if refB && !saveB {
		return e.biunifyRef(b, a, b2, b1, iter)
	}

	compA := ast.IsComprehension(a.Value)
	compB := ast.IsComprehension(b.Value)

	if saveA || saveB {
		return e.saveUnify(a, b, b1, b2, iter)
	}

	if compA {
		return e.biunifyComprehension(a, b, b1, b2, false, iter)
	} else if compB {
		return e.biunifyComprehension(b, a, b2, b1, true, iter)
	}

	// Perform standard unification.
	_, varA := a.Value.(ast.Var)
	_, varB := b.Value.(ast.Var)

	if varA && varB {
		if b1 == b2 && a.Equal(b) {
			return iter()
		}
		undo := b1.bind(a, b, b2)
		err := iter()
		undo.Undo()
		return err
	} else if varA && !varB {
		undo := b1.bind(a, b, b2)
		err := iter()
		undo.Undo()
		return err
	} else if varB && !varA {
		undo := b2.bind(b, a, b1)
		err := iter()
		undo.Undo()
		return err
	}

	// Sets must not contain unbound variables at this point as we cannot unify
	// them. So simply plug both sides (to substitute any bound variables with
	// values) and then check for equality.
	switch a.Value.(type) {
	case ast.Set:
		a = b1.Plug(a)
		b = b2.Plug(b)
	}

	if a.Equal(b) {
		return iter()
	}

	return nil
}

func (e *eval) biunifyRef(a, b *ast.Term, b1, b2 *bindings, iter unifyIterator) error {

	ref := a.Value.(ast.Ref)
	plugged := make(ast.Ref, len(ref))
	plugged[0] = ref[0]

	if ref[0].Equal(ast.DefaultRootDocument) {
		node := e.compiler.RuleTree.Child(ref[0].Value)

		eval := evalTree{
			e:         e,
			ref:       ref,
			pos:       1,
			plugged:   plugged,
			bindings:  b1,
			rterm:     b,
			rbindings: b2,
			node:      node,
		}
		return eval.eval(iter)
	}

	var term *ast.Term
	var termbindings *bindings

	if ref[0].Equal(ast.InputRootDocument) {
		term = e.input
		termbindings = b1
	} else {
		term, termbindings = b1.apply(ref[0])
		if term == ref[0] {
			term = nil
		}
	}

	if term == nil {
		return nil
	}

	eval := evalTerm{
		e:            e,
		ref:          ref,
		pos:          1,
		bindings:     b1,
		term:         term,
		termbindings: termbindings,
		rterm:        b,
		rbindings:    b2,
	}

	return eval.eval(iter)
}

func (e *eval) biunifyComprehension(a, b *ast.Term, b1, b2 *bindings, swap bool, iter unifyIterator) error {

	if e.partial() {
		return e.biunifyComprehensionPartial(a, b, b1, b2, swap, iter)
	}

	switch a := a.Value.(type) {
	case *ast.ArrayComprehension:
		return e.biunifyComprehensionArray(a, b, b1, b2, iter)
	case *ast.SetComprehension:
		return e.biunifyComprehensionSet(a, b, b1, b2, iter)
	case *ast.ObjectComprehension:
		return e.biunifyComprehensionObject(a, b, b1, b2, iter)
	}

	return fmt.Errorf("illegal comprehension %T", a)
}

func (e *eval) biunifyComprehensionPartial(a, b *ast.Term, b1, b2 *bindings, swap bool, iter unifyIterator) error {

	// Capture bindings available to the comprehension. We will add expressions
	// to the comprehension body that ensure the comprehension body is safe.
	// Currently this process adds _all_ bindings (even if they are not
	// needed.) Eventually we may want to make the logic a bit smarter.
	var extras []*ast.Expr

	err := b1.Iter(sentinel, func(k, v *ast.Term) error {
		extras = append(extras, ast.Equality.Expr(k, v))
		return nil
	})

	if err != nil {
		return err
	}

	// Namespace the variables in the body to avoid collision when the final
	// queries returned by partial evaluation.
	var body *ast.Body

	switch a := a.Value.(type) {
	case *ast.ArrayComprehension:
		body = &a.Body
	case *ast.SetComprehension:
		body = &a.Body
	case *ast.ObjectComprehension:
		body = &a.Body
	default:
		return fmt.Errorf("illegal comprehension %T", a)
	}

	for _, e := range extras {
		body.Append(e)
	}

	b1.Namespace(a, sentinel)

	// The other term might need to be plugged so include the bindings. The
	// bindings for the comprehension term are saved (for compatibility) but
	// the eventual plug operation on the comprehension will be a no-op.
	if !swap {
		return e.saveUnify(a, b, b1, b2, iter)
	}

	return e.saveUnify(b, a, b2, b1, iter)
}

func (e *eval) biunifyComprehensionArray(x *ast.ArrayComprehension, b *ast.Term, b1, b2 *bindings, iter unifyIterator) error {
	result := ast.Array{}
	child := e.closure(x.Body)
	err := child.Run(func(child *eval) error {
		result = append(result, child.bindings.Plug(x.Term))
		return nil
	})
	if err != nil {
		return err
	}
	return e.biunify(ast.NewTerm(result), b, b1, b2, iter)
}

func (e *eval) biunifyComprehensionSet(x *ast.SetComprehension, b *ast.Term, b1, b2 *bindings, iter unifyIterator) error {
	result := ast.NewSet()
	child := e.closure(x.Body)
	err := child.Run(func(child *eval) error {
		result.Add(child.bindings.Plug(x.Term))
		return nil
	})
	if err != nil {
		return err
	}
	return e.biunify(ast.NewTerm(result), b, b1, b2, iter)
}

func (e *eval) biunifyComprehensionObject(x *ast.ObjectComprehension, b *ast.Term, b1, b2 *bindings, iter unifyIterator) error {
	result := ast.NewObject()
	child := e.closure(x.Body)
	err := child.Run(func(child *eval) error {
		key := child.bindings.Plug(x.Key)
		value := child.bindings.Plug(x.Value)
		exist := result.Get(key)
		if exist != nil && !exist.Equal(value) {
			return objectDocKeyConflictErr(x.Key.Location)
		}
		result.Insert(key, value)
		return nil
	})
	if err != nil {
		return err
	}
	return e.biunify(ast.NewTerm(result), b, b1, b2, iter)
}

type savePair struct {
	term *ast.Term
	b    *bindings
}

func getSavePairs(x *ast.Term, b *bindings, result []savePair) []savePair {
	if _, ok := x.Value.(ast.Var); ok {
		result = append(result, savePair{x, b})
		return result
	}
	vis := ast.NewVarVisitor().WithParams(ast.VarVisitorParams{
		SkipClosures: true,
		SkipRefHead:  true,
	})
	ast.Walk(vis, x)
	for v := range vis.Vars() {
		y, next := b.apply(ast.NewTerm(v))
		result = getSavePairs(y, next, result)
	}
	return result
}

func (e *eval) saveExpr(expr *ast.Expr, b *bindings, iter unifyIterator) error {
	e.saveStack.Push(expr, b, b)
	defer e.saveStack.Pop()
	e.traceSave(expr)
	return iter()
}

func (e *eval) savePluggedExprs(exprs []*ast.Expr, iter unifyIterator) error {
	for _, expr := range exprs {
		e.saveStack.Push(expr, nil, nil)
		defer e.saveStack.Pop()
		e.traceSave(expr)
	}
	return iter()
}

func (e *eval) saveUnify(a, b *ast.Term, b1, b2 *bindings, iter unifyIterator) error {
	expr := ast.Equality.Expr(a, b)
	if pairs := getSavePairs(a, b1, nil); len(pairs) > 0 {
		for _, p := range pairs {
			e.saveSet.Push([]*ast.Term{p.term}, p.b)
			defer e.saveSet.Pop()
		}
	}
	if pairs := getSavePairs(b, b2, nil); len(pairs) > 0 {
		for _, p := range pairs {
			e.saveSet.Push([]*ast.Term{p.term}, p.b)
			defer e.saveSet.Pop()
		}
	}
	e.saveStack.Push(expr, b1, b2)
	defer e.saveStack.Pop()
	e.traceSave(expr)
	return iter()
}

func (e *eval) saveCall(declArgsLen int, terms []*ast.Term, iter unifyIterator) error {
	expr := ast.NewExpr(terms)
	// If call-site includes output value then partial eval must add vars in output
	// position to the save set.
	if declArgsLen == len(terms)-2 {
		if pairs := getSavePairs(terms[len(terms)-1], e.bindings, nil); len(pairs) > 0 {
			for _, p := range pairs {
				e.saveSet.Push([]*ast.Term{p.term}, p.b)
				defer e.saveSet.Pop()
			}
		}
	}
	e.saveStack.Push(expr, e.bindings, nil)
	defer e.saveStack.Pop()
	e.traceSave(expr)
	return iter()
}

func (e *eval) getRules(ref ast.Ref) (*ast.IndexResult, error) {
	e.instr.startTimer(evalOpRuleIndex)
	defer e.instr.stopTimer(evalOpRuleIndex)

	index := e.compiler.RuleIndex(ref)
	if index == nil {
		return nil, nil
	}

	result, err := index.Lookup(e)
	if err != nil {
		return nil, err
	}

	var msg string
	if len(result.Rules) == 1 {
		msg = "(matched 1 rule)"
	} else {
		msg = fmt.Sprintf("(matched %v rules)", len(result.Rules))
	}
	e.traceIndex(e.query[e.index], msg)
	return result, err
}

func (e *eval) Resolve(ref ast.Ref) (ast.Value, error) {
	e.instr.startTimer(evalOpResolve)
	defer e.instr.stopTimer(evalOpResolve)

	if e.saveSet.Contains(ast.NewTerm(ref), nil) {
		return nil, ast.UnknownValueErr{}
	}

	if ref[0].Equal(ast.InputRootDocument) {
		if e.input != nil {
			v, err := e.input.Value.Find(ref[1:])
			if err != nil {
				return nil, nil
			}
			return v, nil
		}
		return nil, nil
	}

	if ref[0].Equal(ast.DefaultRootDocument) {

		// Check cache with replacement data
		repValue, ok := e.withCache.Get(ref)
		if ok {
			return repValue, nil
		}

		expr := e.query[e.index]
		if len(expr.With) > 0 && repValue == nil {
			return nil, nil
		}

		var merged ast.Value
		var err error

		// Check cache with real data

		// Converting large JSON values into AST values can be fairly expensive. For
		// example, a 2MB JSON value can take upwards of 30 millisceonds to convert.
		// We cache the result of conversion here in case the same base document is
		// being read multiple times during evaluation.
		realValue, ok := e.baseCache.Get(ref)
		if ok {
			e.instr.counterIncr(evalOpBaseCacheHit)
			if repValue == nil {
				return realValue, nil
			}
			merged, ok = merge(repValue, realValue)
			if !ok {
				return nil, mergeConflictErr(ref[0].Location)
			}
		} else {
			e.instr.counterIncr(evalOpBaseCacheMiss)
			merged, err = e.resolveReadFromStorage(ref, repValue)
			if err != nil {
				return nil, err
			}
		}
		return merged, nil
	}
	return nil, fmt.Errorf("illegal ref")
}

func (e *eval) resolveReadFromStorage(ref ast.Ref, a ast.Value) (ast.Value, error) {
	path, err := storage.NewPathForRef(ref)
	if err != nil {
		if !storage.IsNotFound(err) {
			return nil, err
		}
		return a, nil
	}

	blob, err := e.store.Read(e.ctx, e.txn, path)
	if err != nil {
		if !storage.IsNotFound(err) {
			return nil, err
		}
		return a, nil
	}

	if len(path) == 0 {
		obj := blob.(map[string]interface{})
		if len(obj) > 0 {
			cpy := make(map[string]interface{}, len(obj)-1)
			for k, v := range obj {
				if string(ast.SystemDocumentKey) == k {
					continue
				}
				cpy[k] = v
			}
			blob = cpy
		}
	}

	v, err := ast.InterfaceToValue(blob)
	if err != nil {
		return nil, err
	}

	e.baseCache.Put(ref, v)

	if a == nil {
		return v, nil
	}

	merged, ok := merge(a, v)
	if !ok {
		return nil, mergeConflictErr(ref[0].Location)
	}
	return merged, nil
}

func merge(a, b ast.Value) (ast.Value, bool) {
	aObj, ok1 := a.(ast.Object)
	bObj, ok2 := b.(ast.Object)

	if ok1 && ok2 {
		return mergeObjects(aObj, bObj)
	}
	return nil, false
}

// mergeObjects returns a new Object containing the non-overlapping keys of
// the objA and objB. If there are overlapping keys between objA and objB,
// the values of associated with the keys are merged. Only
// objects can be merged with other objects. If the values cannot be merged,
// objB value will be overwritten by objA value.
func mergeObjects(objA, objB ast.Object) (result ast.Object, ok bool) {
	result = ast.NewObject()
	stop := objA.Until(func(k, v *ast.Term) bool {
		if v2 := objB.Get(k); v2 == nil {
			result.Insert(k, v)
		} else {
			obj1, ok1 := v.Value.(ast.Object)
			obj2, ok2 := v2.Value.(ast.Object)

			if !ok1 || !ok2 {
				result.Insert(k, v)
				return false
			}
			obj3, ok := mergeObjects(obj1, obj2)
			if !ok {
				return true
			}
			result.Insert(k, ast.NewTerm(obj3))
		}
		return false
	})
	if stop {
		return nil, false
	}
	objB.Foreach(func(k, v *ast.Term) {
		if v2 := objA.Get(k); v2 == nil {
			result.Insert(k, v)
		}
	})
	return result, true
}

func (e *eval) generateVar(suffix string) *ast.Term {
	return ast.VarTerm(fmt.Sprintf("%v_%v", e.genvarprefix, suffix))
}

type evalBuiltin struct {
	e     *eval
	bi    *ast.Builtin
	bctx  BuiltinContext
	f     BuiltinFunc
	terms []*ast.Term
}

func (e evalBuiltin) eval(iter unifyIterator) error {

	operands := make([]*ast.Term, len(e.terms))

	for i := 0; i < len(e.terms); i++ {
		operands[i] = e.e.bindings.Plug(e.terms[i])
	}

	numDeclArgs := len(e.bi.Decl.Args())

	e.e.instr.startTimer(evalOpBuiltinCall)
	defer e.e.instr.stopTimer(evalOpBuiltinCall)

	return e.f(e.bctx, operands, func(output *ast.Term) error {

		e.e.instr.stopTimer(evalOpBuiltinCall)
		defer e.e.instr.startTimer(evalOpBuiltinCall)

		if len(operands) == numDeclArgs {
			if output.Value.Compare(ast.Boolean(false)) != 0 {
				return iter()
			}
			return nil
		}
		return e.e.unify(e.terms[len(e.terms)-1], output, iter)
	})
}

type evalFunc struct {
	e     *eval
	ref   ast.Ref
	terms []*ast.Term
}

func (e evalFunc) eval(iter unifyIterator) error {

	ir, err := e.e.getRules(e.ref)
	if err != nil {
		return err
	}

	if ir.Empty() {
		return nil
	}

	// Partial evaluation of ordered rules is not supported currently. Save the
	// expression and continue. This could be revisited in the future.
	if e.e.partial() && len(ir.Else) > 0 {
		return e.e.saveCall(len(ir.Rules[0].Head.Args), e.terms, iter)
	}

	var prev *ast.Term

	for i := range ir.Rules {
		next, err := e.evalOneRule(iter, ir.Rules[i], prev)
		if err != nil {
			return err
		}
		if next == nil {
			for _, rule := range ir.Else[ir.Rules[i]] {
				next, err = e.evalOneRule(iter, rule, prev)
				if err != nil {
					return err
				}
				if next != nil {
					break
				}
			}
		}
		if next != nil {
			prev = next
		}
	}

	return nil
}

func (e evalFunc) evalOneRule(iter unifyIterator, rule *ast.Rule, prev *ast.Term) (*ast.Term, error) {

	child := e.e.child(rule.Body)

	args := make(ast.Array, len(e.terms)-1)

	for i := range rule.Head.Args {
		args[i] = rule.Head.Args[i]
	}

	if len(args) == len(rule.Head.Args)+1 {
		args[len(args)-1] = rule.Head.Value
	}

	var result *ast.Term

	child.traceEnter(rule)

	err := child.biunifyArrays(e.terms[1:], args, e.e.bindings, child.bindings, func() error {
		return child.eval(func(child *eval) error {
			child.traceExit(rule)
			result = child.bindings.Plug(rule.Head.Value)

			if len(rule.Head.Args) == len(e.terms)-1 {
				if result.Value.Compare(ast.Boolean(false)) == 0 {
					return nil
				}
			}

			// Partial evaluation should explore all rules and may not produce
			// a ground result so we do not perform conflict detection or
			// deduplication. See "ignore conflicts: functions" test case for
			// an example.
			if !e.e.partial() {
				if prev != nil {
					if ast.Compare(prev, result) != 0 {
						return functionConflictErr(rule.Location)
					}
					child.traceRedo(rule)
					return nil
				}
			}

			prev = result

			if err := iter(); err != nil {
				return err
			}

			child.traceRedo(rule)
			return nil
		})
	})

	return result, err
}

type evalTree struct {
	e         *eval
	ref       ast.Ref
	plugged   ast.Ref
	pos       int
	bindings  *bindings
	rterm     *ast.Term
	rbindings *bindings
	node      *ast.TreeNode
}

func (e evalTree) eval(iter unifyIterator) error {

	if len(e.ref) == e.pos {
		return e.finish(iter)
	}

	plugged := e.bindings.Plug(e.ref[e.pos])

	if plugged.IsGround() {
		return e.next(iter, plugged)
	}

	return e.enumerate(iter)
}

func (e evalTree) finish(iter unifyIterator) error {

	// During partial evaluation it may not be possible to compute the value
	// for this reference if it refers to a virtual document so save the entire
	// expression. See "save: full extent" test case for an example.
	if e.e.partial() && e.node != nil {
		return e.e.saveUnify(ast.NewTerm(e.plugged), e.rterm, e.bindings, e.rbindings, iter)
	}

	v, err := e.extent()
	if err != nil || v == nil {
		return err
	}

	return e.e.biunify(e.rterm, v, e.rbindings, e.bindings, func() error {
		return iter()
	})
}

func (e evalTree) next(iter unifyIterator, plugged *ast.Term) error {

	var node *ast.TreeNode

	_, found := e.e.withCache.Get(e.ref)

	if !found {
		if e.node != nil {
			node = e.node.Child(plugged.Value)
			if node != nil && len(node.Values) > 0 {
				r := evalVirtual{
					e:         e.e,
					ref:       e.ref,
					plugged:   e.plugged,
					pos:       e.pos,
					bindings:  e.bindings,
					rterm:     e.rterm,
					rbindings: e.rbindings,
				}
				r.plugged[e.pos] = plugged
				return r.eval(iter)
			}
		}
	}

	cpy := e
	cpy.plugged[e.pos] = plugged
	cpy.node = node
	cpy.pos++
	return cpy.eval(iter)
}

func (e evalTree) enumerate(iter unifyIterator) error {
	doc, err := e.e.Resolve(e.plugged[:e.pos])
	if err != nil {
		return err
	}

	if doc != nil {
		switch doc := doc.(type) {
		case ast.Array:
			for i := range doc {
				k := ast.IntNumberTerm(i)
				err := e.e.biunify(k, e.ref[e.pos], e.bindings, e.bindings, func() error {
					return e.next(iter, k)
				})
				if err != nil {
					return err
				}
			}
		case ast.Object:
			err := doc.Iter(func(k, _ *ast.Term) error {
				return e.e.biunify(k, e.ref[e.pos], e.bindings, e.bindings, func() error {
					return e.next(iter, k)
				})
			})
			if err != nil {
				return err
			}
		}
	}

	if e.node == nil {
		return nil
	}

	for k := range e.node.Children {
		key := ast.NewTerm(k)
		e.e.biunify(key, e.ref[e.pos], e.bindings, e.bindings, func() error {
			return e.next(iter, key)
		})
	}

	return nil
}

func (e evalTree) extent() (*ast.Term, error) {
	base, err := e.e.Resolve(e.plugged)
	if err != nil {
		return nil, err
	}

	virtual, err := e.leaves(e.plugged, e.node)
	if err != nil {
		return nil, err
	}

	if virtual == nil {
		if base == nil {
			return nil, nil
		}
		return ast.NewTerm(base), nil
	}

	if base != nil {
		merged, ok := merge(base, virtual)
		if !ok {
			return nil, mergeConflictErr(e.plugged[0].Location)
		}
		return ast.NewTerm(merged), nil
	}

	return ast.NewTerm(virtual), nil
}

func (e evalTree) leaves(plugged ast.Ref, node *ast.TreeNode) (ast.Object, error) {

	if e.node == nil {
		return nil, nil
	}

	result := ast.NewObject()

	for _, child := range node.Children {
		if child.Hide {
			continue
		}

		plugged = append(plugged, ast.NewTerm(child.Key))

		var save ast.Value
		var err error

		if len(child.Values) > 0 {
			rterm := e.e.generateVar("leaf")
			err = e.e.unify(ast.NewTerm(plugged), rterm, func() error {
				save = e.e.bindings.Plug(rterm).Value
				return nil
			})
		} else {
			save, err = e.leaves(plugged, child)
		}

		if err != nil {
			return nil, err
		}

		if save != nil {
			v := ast.NewObject([2]*ast.Term{plugged[len(plugged)-1], ast.NewTerm(save)})
			result, _ = result.Merge(v)
		}

		plugged = plugged[:len(plugged)-1]
	}

	return result, nil
}

type evalVirtual struct {
	e         *eval
	ref       ast.Ref
	plugged   ast.Ref
	pos       int
	bindings  *bindings
	rterm     *ast.Term
	rbindings *bindings
}

func (e evalVirtual) eval(iter unifyIterator) error {

	ir, err := e.e.getRules(e.plugged[:e.pos+1])
	if err != nil {
		return err
	}

	// Partial evaluation of ordered rules is not supported currently. Save the
	// expression and continue. This could be revisited in the future.
	if e.e.partial() && len(ir.Else) > 0 {
		return e.e.saveUnify(ast.NewTerm(e.ref), e.rterm, e.bindings, e.rbindings, iter)
	}

	switch ir.Kind {
	case ast.PartialSetDoc:
		eval := evalVirtualPartial{
			e:         e.e,
			ref:       e.ref,
			plugged:   e.plugged,
			pos:       e.pos,
			ir:        ir,
			bindings:  e.bindings,
			rterm:     e.rterm,
			rbindings: e.rbindings,
			empty:     ast.SetTerm(),
		}
		return eval.eval(iter)
	case ast.PartialObjectDoc:
		eval := evalVirtualPartial{
			e:         e.e,
			ref:       e.ref,
			plugged:   e.plugged,
			pos:       e.pos,
			ir:        ir,
			bindings:  e.bindings,
			rterm:     e.rterm,
			rbindings: e.rbindings,
			empty:     ast.ObjectTerm(),
		}
		return eval.eval(iter)
	default:
		eval := evalVirtualComplete{
			e:         e.e,
			ref:       e.ref,
			plugged:   e.plugged,
			pos:       e.pos,
			ir:        ir,
			bindings:  e.bindings,
			rterm:     e.rterm,
			rbindings: e.rbindings,
		}
		return eval.eval(iter)
	}
}

type evalVirtualPartial struct {
	e         *eval
	ref       ast.Ref
	plugged   ast.Ref
	pos       int
	ir        *ast.IndexResult
	bindings  *bindings
	rterm     *ast.Term
	rbindings *bindings
	empty     *ast.Term
}

func (e evalVirtualPartial) eval(iter unifyIterator) error {

	if len(e.ref) == e.pos+1 {
		// During partial evaluation, it may not be possible to produce a value
		// for this reference so save the entire expression. See "save: full
		// extent: partial object" test case for an example.
		if e.e.partial() {
			return e.e.saveUnify(ast.NewTerm(e.ref), e.rterm, e.bindings, e.rbindings, iter)
		}
		return e.evalAllRules(iter, e.ir.Rules)
	}

	var cacheKey ast.Ref

	if e.ir.Kind == ast.PartialObjectDoc {
		plugged := e.bindings.Plug(e.ref[e.pos+1])

		if plugged.IsGround() {
			path := e.plugged[:e.pos+2]
			path[len(path)-1] = plugged
			cached := e.e.virtualCache.Get(path)

			if cached != nil {
				e.e.instr.counterIncr(evalOpVirtualCacheHit)
				return e.evalTerm(iter, cached, e.bindings)
			}

			e.e.instr.counterIncr(evalOpVirtualCacheMiss)
			cacheKey = path
		}
	}

	for _, rule := range e.ir.Rules {
		if err := e.evalOneRule(iter, rule, cacheKey); err != nil {
			return err
		}
	}

	return nil
}

func (e evalVirtualPartial) evalAllRules(iter unifyIterator, rules []*ast.Rule) error {

	result := e.empty

	for _, rule := range rules {
		child := e.e.child(rule.Body)
		child.traceEnter(rule)

		err := child.eval(func(*eval) error {
			child.traceExit(rule)
			var err error
			result, err = e.reduce(rule.Head, child.bindings, result)
			if err != nil {
				return err
			}

			child.traceRedo(rule)
			return nil
		})

		if err != nil {
			return err
		}
	}

	return e.e.biunify(result, e.rterm, e.bindings, e.bindings, iter)
}

func (e evalVirtualPartial) evalOneRule(iter unifyIterator, rule *ast.Rule, cacheKey ast.Ref) error {

	key := e.ref[e.pos+1]
	child := e.e.child(rule.Body)

	child.traceEnter(rule)
	var defined bool

	err := child.biunify(rule.Head.Key, key, child.bindings, e.bindings, func() error {
		defined = true
		return child.eval(func(child *eval) error {
			child.traceExit(rule)

			term := rule.Head.Value
			if term == nil {
				term = rule.Head.Key
			}

			if cacheKey != nil {
				result := child.bindings.Plug(term)
				e.e.virtualCache.Put(cacheKey, result)
			}

			term, termbindings := child.bindings.apply(term)
			err := e.evalTerm(iter, term, termbindings)
			if err != nil {
				return err
			}

			child.traceRedo(rule)
			return nil
		})
	})

	if err != nil {
		return err
	}

	if !defined {
		child.traceFail(rule)
	}

	return nil
}

func (e evalVirtualPartial) evalTerm(iter unifyIterator, term *ast.Term, termbindings *bindings) error {
	eval := evalTerm{
		e:            e.e,
		ref:          e.ref,
		pos:          e.pos + 2,
		bindings:     e.bindings,
		term:         term,
		termbindings: termbindings,
		rterm:        e.rterm,
		rbindings:    e.rbindings,
	}
	return eval.eval(iter)
}

func (e evalVirtualPartial) reduce(head *ast.Head, b *bindings, result *ast.Term) (*ast.Term, error) {

	switch v := result.Value.(type) {
	case ast.Set:
		v.Add(b.Plug(head.Key))
	case ast.Object:
		key := b.Plug(head.Key)
		value := b.Plug(head.Value)
		exist := v.Get(key)
		if exist != nil && !exist.Equal(value) {
			return nil, objectDocKeyConflictErr(head.Location)
		}
		v.Insert(key, value)
		result.Value = v
	}

	return result, nil
}

type evalVirtualComplete struct {
	e         *eval
	ref       ast.Ref
	plugged   ast.Ref
	pos       int
	ir        *ast.IndexResult
	bindings  *bindings
	rterm     *ast.Term
	rbindings *bindings
}

func (e evalVirtualComplete) eval(iter unifyIterator) error {

	if e.ir.Empty() {
		return nil
	}

	if len(e.ir.Rules) > 0 && len(e.ir.Rules[0].Head.Args) > 0 {
		return nil
	}

	if !e.e.partial() {
		return e.evalValue(iter)
	}

	if e.ir.Default != nil {
		rterm := e.rbindings.Plug(e.rterm)

		// If the other term is not constant OR it's equal to the default value, then
		// a support rule must be produced as the default value _may_ be required. On
		// the other hand, if the other term is constant (i.e., it does not require
		// evaluation) and it differs from the default value then the default value is
		// _not_ required, so partially evaluate the rule normally.
		if !ast.IsConstant(rterm.Value) || e.ir.Default.Head.Value.Equal(rterm) {
			return e.partialEvalDefault(iter)
		}
	}

	return e.partialEval(iter)
}

func (e evalVirtualComplete) evalValue(iter unifyIterator) error {
	cached := e.e.virtualCache.Get(e.plugged[:e.pos+1])
	if cached != nil {
		e.e.instr.counterIncr(evalOpVirtualCacheHit)
		return e.evalTerm(iter, cached, e.bindings)
	}

	e.e.instr.counterIncr(evalOpVirtualCacheMiss)

	var prev *ast.Term

	for i := range e.ir.Rules {
		next, err := e.evalValueRule(iter, e.ir.Rules[i], prev)
		if err != nil {
			return err
		}
		if next == nil {
			for _, rule := range e.ir.Else[e.ir.Rules[i]] {
				next, err = e.evalValueRule(iter, rule, prev)
				if err != nil {
					return err
				}
				if next != nil {
					break
				}
			}
		}
		if next != nil {
			prev = next
		}
	}

	if e.ir.Default != nil && prev == nil {
		_, err := e.evalValueRule(iter, e.ir.Default, prev)
		return err
	}

	return nil
}

func (e evalVirtualComplete) evalValueRule(iter unifyIterator, rule *ast.Rule, prev *ast.Term) (*ast.Term, error) {

	child := e.e.child(rule.Body)
	child.traceEnter(rule)
	var result *ast.Term

	err := child.eval(func(child *eval) error {
		child.traceExit(rule)
		result = child.bindings.Plug(rule.Head.Value)

		if prev != nil {
			if ast.Compare(result, prev) != 0 {
				return completeDocConflictErr(rule.Location)
			}
			child.traceRedo(rule)
			return nil
		}

		prev = result
		e.e.virtualCache.Put(e.plugged[:e.pos+1], result)
		term, termbindings := child.bindings.apply(rule.Head.Value)

		err := e.evalTerm(iter, term, termbindings)
		if err != nil {
			return err
		}

		child.traceRedo(rule)
		return nil
	})

	return result, err
}

func (e evalVirtualComplete) partialEval(iter unifyIterator) error {

	for _, rule := range e.ir.Rules {
		child := e.e.child(rule.Body)
		child.traceEnter(rule)

		err := child.eval(func(child *eval) error {
			child.traceExit(rule)
			term, termbindings := child.bindings.apply(rule.Head.Value)

			err := e.evalTerm(iter, term, termbindings)
			if err != nil {
				return err
			}

			child.traceRedo(rule)
			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}

func (e evalVirtualComplete) partialEvalDefault(iter unifyIterator) error {

	path := e.plugged[:e.pos+1].Insert(e.e.saveNamespace, 1)

	if !e.e.saveSupport.Exists(path) {

		for i := range e.ir.Rules {
			err := e.partialEvalDefaultRule(iter, e.ir.Rules[i], path)
			if err != nil {
				return err
			}
		}

		err := e.partialEvalDefaultRule(iter, e.ir.Default, path)
		if err != nil {
			return err
		}
	}

	rewritten := ast.NewTerm(e.ref.Insert(e.e.saveNamespace, 1))
	return e.e.saveUnify(rewritten, e.rterm, e.bindings, e.rbindings, iter)
}

func (e evalVirtualComplete) partialEvalDefaultRule(iter unifyIterator, rule *ast.Rule, path ast.Ref) error {

	child := e.e.child(rule.Body)
	child.traceEnter(rule)

	e.e.saveStack.PushQuery(nil)
	defer e.e.saveStack.PopQuery()

	return child.eval(func(child *eval) error {
		child.traceExit(rule)

		current := e.e.saveStack.PopQuery()
		defer e.e.saveStack.PushQuery(current)
		plugged := current.Plug(child.bindings)

		head := ast.NewHead(rule.Head.Name, nil, child.bindings.PlugNamespaced(rule.Head.Value, child.bindings))
		p := copypropagation.New(head.Vars()).WithEnsureNonEmptyBody(true)

		e.e.saveSupport.Insert(path, &ast.Rule{
			Head:    head,
			Body:    p.Apply(plugged),
			Default: rule.Default,
		})

		child.traceRedo(rule)
		return nil
	})
}

func (e evalVirtualComplete) evalTerm(iter unifyIterator, term *ast.Term, termbindings *bindings) error {
	eval := evalTerm{
		e:            e.e,
		ref:          e.ref,
		pos:          e.pos + 1,
		bindings:     e.bindings,
		term:         term,
		termbindings: termbindings,
		rterm:        e.rterm,
		rbindings:    e.rbindings,
	}
	return eval.eval(iter)
}

type evalTerm struct {
	e            *eval
	ref          ast.Ref
	pos          int
	bindings     *bindings
	term         *ast.Term
	termbindings *bindings
	rterm        *ast.Term
	rbindings    *bindings
}

func (e evalTerm) eval(iter unifyIterator) error {

	if len(e.ref) == e.pos {
		return e.e.biunify(e.term, e.rterm, e.termbindings, e.rbindings, iter)
	}

	if e.e.saveSet.Contains(e.term, e.termbindings) {
		return e.save(iter)
	}

	plugged := e.bindings.Plug(e.ref[e.pos])

	if plugged.IsGround() {
		return e.next(iter, plugged)
	}

	return e.enumerate(iter)
}

func (e evalTerm) next(iter unifyIterator, plugged *ast.Term) error {

	term, bindings := e.get(plugged)
	if term == nil {
		return nil
	}

	cpy := e
	cpy.term = term
	cpy.termbindings = bindings
	cpy.pos++
	return cpy.eval(iter)
}

func (e evalTerm) enumerate(iter unifyIterator) error {

	switch v := e.term.Value.(type) {
	case ast.Array:
		for i := range v {
			k := ast.IntNumberTerm(i)
			err := e.e.biunify(k, e.ref[e.pos], e.bindings, e.bindings, func() error {
				return e.next(iter, k)
			})
			if err != nil {
				return err
			}
		}
	case ast.Object:
		return v.Iter(func(k, _ *ast.Term) error {
			return e.e.biunify(k, e.ref[e.pos], e.termbindings, e.bindings, func() error {
				return e.next(iter, e.termbindings.Plug(k))
			})
		})
	case ast.Set:
		return v.Iter(func(elem *ast.Term) error {
			return e.e.biunify(elem, e.ref[e.pos], e.termbindings, e.bindings, func() error {
				return e.next(iter, e.termbindings.Plug(elem))
			})
		})
	}

	return nil
}

func (e evalTerm) get(plugged *ast.Term) (*ast.Term, *bindings) {
	switch v := e.term.Value.(type) {
	case ast.Set:
		if v.IsGround() {
			if v.Contains(plugged) {
				return e.termbindings.apply(plugged)
			}
		} else {
			var t *ast.Term
			var b *bindings
			stop := v.Until(func(elem *ast.Term) bool {
				if e.termbindings.Plug(elem).Equal(plugged) {
					t, b = e.termbindings.apply(plugged)
					return true
				}
				return false
			})
			if stop {
				return t, b
			}
		}
	case ast.Object:
		if v.IsGround() {
			term := v.Get(plugged)
			if term != nil {
				return e.termbindings.apply(term)
			}
		} else {
			var t *ast.Term
			var b *bindings
			stop := v.Until(func(k, v *ast.Term) bool {
				if e.termbindings.Plug(k).Equal(plugged) {
					t, b = e.termbindings.apply(v)
					return true
				}
				return false
			})
			if stop {
				return t, b
			}
		}
	case ast.Array:
		term := v.Get(plugged)
		if term != nil {
			return e.termbindings.apply(term)
		}
	}
	return nil, nil
}

func (e evalTerm) save(iter unifyIterator) error {

	suffix := e.ref[e.pos:]
	ref := make(ast.Ref, len(suffix)+1)
	ref[0] = e.term

	for i := 0; i < len(suffix); i++ {
		ref[i+1] = suffix[i]
	}

	return e.e.biunify(ast.NewTerm(ref), e.rterm, e.termbindings, e.rbindings, iter)
}

func nonGroundKeys(a ast.Object) bool {
	return a.Until(func(k, _ *ast.Term) bool {
		return !k.IsGround()
	})
}

func plugKeys(a ast.Object, b *bindings) ast.Object {
	plugged, _ := a.Map(func(k, v *ast.Term) (*ast.Term, *ast.Term, error) {
		return b.Plug(k), v, nil
	})
	return plugged
}

func plugSlice(xs []*ast.Term, b *bindings) []*ast.Term {
	cpy := make([]*ast.Term, len(xs))
	for i := range cpy {
		cpy[i] = b.Plug(xs[i])
	}
	return cpy
}

func canInlineNegation(safe ast.VarSet, queries []ast.Body) bool {

	size := 1

	for _, query := range queries {
		size *= len(query)
		for _, expr := range query {
			if !expr.Negated {
				// Positive expressions containing variables cannot be trivially negated
				// because they become unsafe (e.g., "x = 1" negated is "not x = 1" making x
				// unsafe.) We check if the vars in the expr are already safe.
				vis := ast.NewVarVisitor().WithParams(ast.VarVisitorParams{
					SkipRefCallHead: true,
					SkipClosures:    true,
				})
				ast.Walk(vis, expr)
				unsafe := vis.Vars().Diff(safe).Diff(ast.ReservedVars)
				if len(unsafe) > 0 {
					return false
				}
			}
		}
	}

	// NOTE(tsandall): this limit is arbitrary–it's only in place to prevent the
	// partial evaluation result from blowing up. In the future, we could make this
	// configurable or do something more clever.
	if size > 16 {
		return false
	}

	return true
}

func complementedCartesianProduct(queries []ast.Body, idx int, curr ast.Body, iter func(ast.Body) error) error {
	if idx == len(queries) {
		return iter(curr)
	}
	for _, expr := range queries[idx] {
		curr = append(curr, expr.Complement())
		if err := complementedCartesianProduct(queries, idx+1, curr, iter); err != nil {
			return err
		}
		curr = curr[:len(curr)-1]
	}
	return nil
}

func isInputRef(term *ast.Term) bool {
	if ref, ok := term.Value.(ast.Ref); ok {
		if ref.HasPrefix(ast.InputRootRef) {
			return true
		}
	}
	return false
}

func isDataRef(term *ast.Term) bool {
	if ref, ok := term.Value.(ast.Ref); ok {
		if ref.HasPrefix(ast.DefaultRootRef) {
			return true
		}
	}
	return false
}
