package topdown

import (
	"cmp"
	"container/list"
	"fmt"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util"
)

// saveSet contains a stack of terms that are considered 'unknown' during
// partial evaluation. Only var and ref terms (rooted at one of the root
// documents) can be added to the save set. Vars added to the save set are
// namespaced by the binding list they are added with. This means the save set
// can be shared across queries.
type saveSet struct {
	instr *Instrumentation
	l     *list.List
}

func newSaveSet(ts []*ast.Term, b *bindings, instr *Instrumentation) *saveSet {
	ss := &saveSet{
		l:     list.New(),
		instr: instr,
	}
	ss.Push(ts, b)
	return ss
}

func (ss *saveSet) Push(ts []*ast.Term, b *bindings) {
	ss.l.PushBack(newSaveSetElem(ts, b))
}

func (ss *saveSet) Pop() {
	ss.l.Remove(ss.l.Back())
}

// Contains returns true if the term t is contained in the save set. Non-var and
// non-ref terms are never contained. Ref terms are contained if they share a
// prefix with a ref that was added (in either direction).
func (ss *saveSet) Contains(t *ast.Term, b *bindings) bool {
	if ss != nil {
		ss.instr.startTimer(partialOpSaveSetContains)
		ret := ss.contains(t, b)
		ss.instr.stopTimer(partialOpSaveSetContains)
		return ret
	}
	return false
}

func (ss *saveSet) contains(t *ast.Term, b *bindings) bool {
	for el := ss.l.Back(); el != nil; el = el.Prev() {
		if el.Value.(*saveSetElem).Contains(t, b) {
			return true
		}
	}
	return false
}

// ContainsRecursive returns true if the term t is or contains a term that is
// contained in the save set. This function will close over the binding list
// when it encounters vars.
func (ss *saveSet) ContainsRecursive(t *ast.Term, b *bindings) bool {
	if ss != nil {
		ss.instr.startTimer(partialOpSaveSetContainsRec)
		ret := ss.containsrec(t, b)
		ss.instr.stopTimer(partialOpSaveSetContainsRec)
		return ret
	}
	return false
}

func (ss *saveSet) containsrec(t *ast.Term, b *bindings) bool {
	var found bool
	ast.WalkTerms(t, func(x *ast.Term) bool {
		if _, ok := x.Value.(ast.Var); ok {
			x1, b1 := b.apply(x)
			if x1 != x || b1 != b {
				if ss.containsrec(x1, b1) {
					found = true
				}
			} else if ss.contains(x1, b1) {
				found = true
			}
		}
		return found
	})
	return found
}

func (ss *saveSet) Vars(caller *bindings) ast.VarSet {
	result := ast.NewVarSet()
	for x := ss.l.Front(); x != nil; x = x.Next() {
		elem := x.Value.(*saveSetElem)
		for _, v := range elem.vars {
			if v, ok := elem.b.PlugNamespaced(v, caller).Value.(ast.Var); ok {
				result.Add(v)
			}
		}
	}
	return result
}

func (ss *saveSet) String() string {
	var buf []string

	for x := ss.l.Front(); x != nil; x = x.Next() {
		buf = append(buf, x.Value.(*saveSetElem).String())
	}

	return "(" + strings.Join(buf, " ") + ")"
}

type saveSetElem struct {
	refs []ast.Ref
	vars []*ast.Term
	b    *bindings
}

func newSaveSetElem(ts []*ast.Term, b *bindings) *saveSetElem {

	var refs []ast.Ref
	var vars []*ast.Term

	for _, t := range ts {
		switch v := t.Value.(type) {
		case ast.Var:
			vars = append(vars, t)
		case ast.Ref:
			refs = append(refs, v)
		default:
			panic("illegal value")
		}
	}

	return &saveSetElem{
		b:    b,
		vars: vars,
		refs: refs,
	}
}

func (sse *saveSetElem) Contains(t *ast.Term, b *bindings) bool {
	switch other := t.Value.(type) {
	case ast.Var:
		return sse.containsVar(t, b)
	case ast.Ref:
		for _, ref := range sse.refs {
			if ref.HasPrefix(other) || other.HasPrefix(ref) {
				return true
			}
		}
		return sse.containsVar(other[0], b)
	}
	return false
}

func (sse *saveSetElem) String() string {
	return fmt.Sprintf("(refs: %v, vars: %v, b: %v)", sse.refs, sse.vars, sse.b)
}

func (sse *saveSetElem) containsVar(t *ast.Term, b *bindings) bool {
	if b == sse.b {
		for _, v := range sse.vars {
			if v.Equal(t) {
				return true
			}
		}
	}
	return false
}

// saveStack contains a stack of queries that represent the result of partial
// evaluation. When partial evaluation completes, the top of the stack
// represents a complete, partially evaluated query that can be saved and
// evaluated later.
//
// The result is stored in a stack so that partial evaluation of a query can be
// paused and then resumed in cases where different queries make up the result
// of partial evaluation, such as when a rule with a default clause is
// partially evaluated. In this case, the partially evaluated rule will be
// output in the support module.
type saveStack struct {
	Stack util.GroupStack[saveStackElem]
}

func newSaveStack() *saveStack {
	s := &saveStack{}
	s.Stack.PushGroup(nil)
	return s
}

func (s *saveStack) PushQuery(query saveStackQuery) {
	s.Stack.PushGroup(query)
}

func (s *saveStack) PopQuery() saveStackQuery {
	return s.Stack.PopGroup()
}

func (s *saveStack) Peek() saveStackQuery {
	return s.Stack.PeekGroup()
}

func (s *saveStack) Push(expr *ast.Expr, b1 *bindings, b2 *bindings) {
	s.Stack.Push(saveStackElem{expr, b1, b2})
}

func (s *saveStack) Pop() {
	s.Stack.Pop()
}

type saveStackQuery []saveStackElem

func (s saveStackQuery) Plug(b *bindings) ast.Body {
	if len(s) == 0 {
		return ast.NewBody(ast.NewExpr(ast.BooleanTerm(true)))
	}
	result := make(ast.Body, len(s))
	for i := range s {
		expr := s[i].Plug(b)
		result.Set(expr, i)
	}
	return result
}

type saveStackElem struct {
	Expr *ast.Expr
	B1   *bindings
	B2   *bindings
}

func (e saveStackElem) Plug(caller *bindings) *ast.Expr {
	if e.B1 == nil && e.B2 == nil {
		return e.Expr
	}
	expr := e.Expr.Copy()
	switch terms := expr.Terms.(type) {
	case []*ast.Term:
		if expr.IsEquality() {
			terms[1] = e.B1.PlugNamespaced(terms[1], caller)
			terms[2] = e.B2.PlugNamespaced(terms[2], caller)
		} else {
			for i := 1; i < len(terms); i++ {
				terms[i] = e.B1.PlugNamespaced(terms[i], caller)
			}
		}
	case *ast.Term:
		expr.Terms = e.B1.PlugNamespaced(terms, caller)
	}
	for i := range expr.With {
		expr.With[i].Value = e.B1.PlugNamespaced(expr.With[i].Value, caller)
	}
	return expr
}

// saveSupport contains additional partially evaluated policies that are part
// of the output of partial evaluation.
//
// The support structure is accumulated as partial evaluation runs and then
// considered complete once partial evaluation finishes (but not before). This
// differs from partially evaluated queries which are considered complete as
// soon as each one finishes.
type saveSupport struct {
	modules map[string]*ast.Module
}

func newSaveSupport() *saveSupport {
	return &saveSupport{
		modules: map[string]*ast.Module{},
	}
}

func (s *saveSupport) List() []*ast.Module {
	result := make([]*ast.Module, 0, len(s.modules))
	for _, module := range s.modules {
		result = append(result, module)
	}
	return result
}

func (s *saveSupport) Exists(path ast.Ref) bool {
	pkg, ruleRef := splitPackageAndRule(path)
	module, ok := s.modules[pkg.String()]
	if !ok {
		return false
	}

	if len(ruleRef) == 1 {
		name := ruleRef[0].Value.(ast.Var)
		for _, rule := range module.Rules {
			if rule.Head.Name == name {
				return true
			}
		}
		return false
	}

	for _, rule := range module.Rules {
		if rule.Head.Ref().HasPrefix(ruleRef) {
			return true
		}
	}

	return false
}

func (s *saveSupport) Insert(path ast.Ref, rule *ast.Rule) {
	pkg, _ := splitPackageAndRule(path)
	s.InsertByPkg(pkg, rule)
}

func (s *saveSupport) InsertByPkg(pkg ast.Ref, rule *ast.Rule) {
	k := pkg.String()
	module, ok := s.modules[k]
	if !ok {
		module = &ast.Module{
			Package: &ast.Package{
				Path: pkg,
			},
		}
		s.modules[k] = module
	}
	rule.Module = module
	module.Rules = append(module.Rules, rule)
}

func splitPackageAndRule(path ast.Ref) (ast.Ref, ast.Ref) {
	p := path.Copy()

	ruleRefStart := 2 // path always contains at least 3 terms (data. + one term in package + rule name)
	for i := ruleRefStart; i < len(p.StringPrefix()); i++ {
		t := p[i]
		if str, ok := t.Value.(ast.String); ok && ast.IsVarCompatibleString(string(str)) {
			ruleRefStart = i
		} else {
			break
		}
	}

	pkg := p[:ruleRefStart]
	rule := p[ruleRefStart:]
	rule[0].Value = ast.Var(rule[0].Value.(ast.String))
	return pkg, rule
}

// saveRequired returns true if the statement x will result in some expressions
// being saved. This check allows the evaluator to evaluate statements
// completely during partial evaluation as long as they do not depend on any
// kind of unknown value or statements that would generate saves.
func saveRequired(compilerTree *ast.TreeNode, extStack *externalTreeStack, ic *inliningControl, icIgnoreInternal bool, ss *saveSet, b *bindings, x any, rec bool) bool {

	var found bool

	vis := ast.NewGenericVisitor(func(node any) bool {
		if found {
			return found
		}
		switch node := node.(type) {
		case *ast.Expr:
			found = len(node.With) > 0
			if found {
				return found
			}
			if !ic.nondeterministicBuiltins { // skip evaluating non-det builtins for PE
				found = ignoreExprDuringPartial(node)
			}
		case *ast.Term:
			switch v := node.Value.(type) {
			case ast.Var:
				// Variables only need to be tested in the node from call site
				// because once traversal recurses into a rule existing unknown
				// variables are out-of-scope.
				if !rec && ss.ContainsRecursive(node, b) {
					found = true
				}
			case ast.Ref:
				if ss.Contains(node, b) {
					found = true
				} else if ic.Disabled(v.ConstantPrefix(), icIgnoreInternal) {
					found = true
				} else {
					// Only terms from the call site can be plugged: once traversal
					// recurses into a rule, that rule's variables belong to another
					// binding list and could resolve to unrelated values in b.
					lookup := v
					if !rec {
						lookup = plugRefForRuleLookup(v, b)
					}
					found = anyRuleDynamic(compilerTree, extStack, lookup, ast.RulesOptions{IncludeHiddenModules: false},
						func(rule *ast.Rule) bool {
							return saveRequired(compilerTree, extStack, ic, icIgnoreInternal, ss, b, rule, true)
						})
				}
			}
		}
		return found
	})

	vis.Walk(x)

	return found
}

// plugRefForRuleLookup replaces variables in ref that are bound to a scalar with
// that value, narrowing rule lookup to the sub-tree that will actually be
// evaluated. Positions left as-is, because they are unbound or bound to a
// composite, fan out over all children as before.
func plugRefForRuleLookup(ref ast.Ref, b *bindings) ast.Ref {
	if b == nil {
		return ref
	}

	cpy := ref

	for i := 1; i < len(ref); i++ {
		if _, ok := ref[i].Value.(ast.Var); !ok {
			continue
		}
		plugged := b.Plug(ref[i])
		if !ast.IsScalar(plugged.Value) {
			continue
		}
		if len(cpy) == len(ref) && &cpy[0] == &ref[0] {
			cpy = make(ast.Ref, len(ref))
			copy(cpy, ref)
		}
		cpy[i] = plugged
	}

	return cpy
}

// anyRuleDynamic invokes f for the rules matching ref in the external trees and
// the compiler tree, stopping as soon as f returns true. Rules are streamed to f
// rather than collected so that callers only interested in whether *some* rule
// satisfies a predicate don't pay for walking the whole matching sub-tree, which
// for refs with non-constant elements can mean every rule loaded.
func anyRuleDynamic(compilerTree *ast.TreeNode, extStack *externalTreeStack, ref ast.Ref, opts ast.RulesOptions, f func(*ast.Rule) bool) bool {
	// Check external trees
	if extStack != nil {
		for i := range extStack.entries {
			entry := &extStack.entries[i]
			if entry.tree != nil && ref.HasPrefix(entry.ref) {
				// Navigate into the external tree using the remaining path
				remaining := ref[len(entry.ref):]
				if anyRuleFromTree(entry.tree, remaining, opts, f) {
					return true
				}
			}
		}
	}

	// Then check compiler tree
	return anyRuleFromTree(compilerTree, ref, opts, f)
}

// anyRuleFromTree walks a tree to find rules matching the given ref, invoking f
// for each and stopping early if f returns true.
func anyRuleFromTree(node *ast.TreeNode, ref ast.Ref, opts ast.RulesOptions, f func(*ast.Rule) bool) bool {
	var walk func(*ast.TreeNode, int) bool
	walk = func(nav *ast.TreeNode, i int) bool {
		switch {
		case i >= len(ref):
			// The rules on nav itself have already been passed to f by the caller,
			// unless nav is where the walk started.
			return anyRuleDescendant(nav, opts, f, i == 0)

		case i == 0 || ast.IsConstant(ref[i].Value):
			child := nav.Child(ref[i].Value)
			if child == nil {
				return false
			}
			return anyRule(child.Values, f) || walk(child, i+1)

		default:
			for _, child := range nav.Children {
				if child.Hide && !opts.IncludeHiddenModules {
					continue
				}
				if anyRule(child.Values, f) || walk(child, i+1) {
					return true
				}
			}
			return false
		}
	}

	return walk(node, 0)
}

// anyRuleDescendant invokes f for every rule in node's sub-tree, stopping early
// if f returns true. The rules on node itself are only visited if visitSelf is
// set. Hidden nodes are not descended into unless opts.IncludeHiddenModules is
// set.
func anyRuleDescendant(node *ast.TreeNode, opts ast.RulesOptions, f func(*ast.Rule) bool, visitSelf bool) bool {
	if visitSelf && anyRule(node.Values, f) {
		return true
	}

	if node.Hide && !opts.IncludeHiddenModules {
		return false
	}

	for _, child := range node.Children {
		if anyRuleDescendant(child, opts, f, true) {
			return true
		}
	}

	return false
}

func anyRule(rules []*ast.Rule, f func(*ast.Rule) bool) bool {
	return slices.ContainsFunc(rules, f)
}

func ignoreExprDuringPartial(expr *ast.Expr) bool {
	if !expr.IsCall() {
		return false
	}

	bi, ok := ast.BuiltinMap[expr.Operator().String()]

	return ok && ignoreDuringPartial(bi)
}

func ignoreDuringPartial(bi *ast.Builtin) bool {
	// Note(philipc): We keep this legacy check around to avoid breaking
	// existing library users.
	//nolint:staticcheck // We specifically ignore our own linter warning here.
	return cmp.Or(slices.Contains(ast.IgnoreDuringPartialEval, bi), bi.Nondeterministic)
}

type inliningControl struct {
	shallow                  bool
	disable                  []disableInliningFrame
	nondeterministicBuiltins bool // evaluate non-det builtins during PE (if args are known)
}

type disableInliningFrame struct {
	internal bool
	refs     []ast.Ref
	v        ast.Var
}

func (i *inliningControl) PushDisable(x any, internal bool) {
	if i == nil {
		return
	}

	switch x := x.(type) {
	case []ast.Ref:
		i.PushDisableRefs(x, internal)
	case ast.Var:
		i.PushDisableVar(x, internal)
	}
}

func (i *inliningControl) PushDisableRefs(refs []ast.Ref, internal bool) {
	if i == nil {
		return
	}

	i.disable = append(i.disable, disableInliningFrame{
		internal: internal,
		refs:     refs,
	})
}

func (i *inliningControl) PushDisableVar(v ast.Var, internal bool) {
	if i == nil {
		return
	}

	i.disable = append(i.disable, disableInliningFrame{
		internal: internal,
		v:        v,
	})
}

func (i *inliningControl) PopDisable() {
	if i == nil {
		return
	}
	i.disable = i.disable[:len(i.disable)-1]
}

func (i *inliningControl) Disabled(x any, ignoreInternal bool) bool {
	if i == nil {
		return false
	}

	switch x := x.(type) {
	case ast.Ref:
		return i.DisabledRef(x, ignoreInternal)
	case ast.Var:
		return i.DisabledVar(x, ignoreInternal)
	}

	return false
}

func (i *inliningControl) DisabledRef(ref ast.Ref, ignoreInternal bool) bool {
	if i == nil {
		return false
	}

	for _, frame := range i.disable {
		if !frame.internal || !ignoreInternal {
			for _, other := range frame.refs {
				if other.HasPrefix(ref) || ref.HasPrefix(other) {
					return true
				}
			}
		}
	}
	return false
}

func (i *inliningControl) DisabledVar(v ast.Var, ignoreInternal bool) bool {
	if i == nil {
		return false
	}

	for _, frame := range i.disable {
		if (!frame.internal || !ignoreInternal) && frame.v == v {
			return true
		}
	}
	return false
}
