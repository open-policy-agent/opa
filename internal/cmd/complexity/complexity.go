// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package complexity

import (
	"fmt"
	"io"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	pr "github.com/open-policy-agent/opa/internal/presentation"
)

// Report holds the result for runtime complexity for the query
type Report struct {
	calculator *Calculator
	Complexity *analyzeQuery `json:"results"`
}

// JSON outputs complexity results to w as JSON
func (r *Report) JSON(w io.Writer) error {
	return pr.JSON(w, r)
}

// String returns the string representation of the runtime complexity
func (r *Report) String() string {
	queryRes := r.Complexity
	buf := fmt.Sprintf("\nComplexity Results for query \"%v\":\n", queryRes.Query)

	if len(queryRes.Missing) != 0 {
		buf = buf + "Missing:\n" + strings.Join(queryRes.Missing, "\n")
	} else {
		analyzeResults := queryRes.String()
		if analyzeResults == "" {
			buf = buf + "O(1)"
		} else {
			buf = buf + fmt.Sprintf("O(%v)", analyzeResults)
		}
	}
	return buf
}

// analyzeQuery holds the result of analyzing the query
// Expressions -> time complexity of each expression in the rule body
// Missing     -> unhandled expression
// Complexity  -> runtime complexity of query
// relation    -> whether the expression is a relation or not
//                An expression is a relation if the reference it contains
//                has first occurrence of a variable. eg. p[x]
// time        -> time complexity of each variable in query
// count       -> count complexity of each variable in query
// size        -> size complexity of each variable in query
// binding     -> map of variable to the value assigned to it
type analyzeQuery struct {
	Query       string   `json:"query"`
	Expressions []*Time  `json:"expressions,omitempty"`
	Missing     []string `json:"missing,omitempty"`
	Complexity  *Time    `json:"complexity,omitempty"`
	relation    []bool
	body        ast.Body
	time        map[ast.Var]*Time
	size        map[ast.Var]*size
	count       map[ast.Var]*count
	binding     *ast.ValueMap
}

func newAnalyzeQuery(query string, body ast.Body) *analyzeQuery {
	return &analyzeQuery{
		Query:       query,
		Expressions: make([]*Time, len(body)),
		Missing:     []string{},
		Complexity:  nil,
		relation:    make([]bool, len(body)),
		body:        body,
		time:        make(map[ast.Var]*Time),
		size:        make(map[ast.Var]*size),
		count:       make(map[ast.Var]*count),
		binding:     ast.NewValueMap(),
	}
}

// String returns the string representation of the runtime complexity
func (q *analyzeQuery) String() string {
	if q.Complexity == nil {
		return ""
	}
	return q.Complexity.String()
}

// runtime calculates the runtime complexity of a query
// Time(Body) = Time(ExprN) * [Time(ExprN+1) * ...] when ExprN is a relation
// Time(Body) = Time(ExprN) + [Time(ExprN+1) + ...] when ExprN is not a relation
func (q *analyzeQuery) runtime() {
	var isProduct bool
	var result Time

	for i := range q.Expressions {
		exprTime := q.Expressions[i]
		if exprTime == nil {
			continue
		}

		if isProduct {
			result = product(*exprTime, result)
		} else {
			result = sum(*exprTime, result)
		}
		isProduct = q.relation[i]
	}
	q.Complexity = &result
}

func product(a, b Time) Time {
	var result Time
	result.Product = append(result.Product, b) // add existing
	result.Product = append(result.Product, a) // add new
	return result
}

func sum(a, b Time) Time {
	var result Time
	result.Sum = append(result.Sum, b) // add existing

	if !result.contains(&a) {
		result.Sum = append(result.Sum, a) // add new
	}
	return result
}

// Time holds results for time complexity
// time complexity can be:
// 1) constant ie. O(1)
// 2) reference ie. represented in terms of base ref
// 3) sum of the time complexity of multiple rules/expressions
// 4) product of the time complexity of multiple expressions
type Time struct {
	R       ast.Ref `json:"ref,omitempty"`
	Sum     []Time  `json:"sum,omitempty"`
	Product []Time  `json:"product,omitempty"`
}

func (t *Time) String() string {
	var result string

	if len(t.R) != 0 {
		result = t.R.String()
	} else if len(t.Sum) != 0 {
		result = t.stringGeneratorSum()
	} else if len(t.Product) != 0 {
		result = t.stringGeneratorProduct()
	}
	return result
}

func (t *Time) stringGeneratorSum() string {

	groupResult := []string{}
	for _, group := range t.Sum {
		temp := group.String()

		if temp == "" {
			continue
		}

		groupResult = append(groupResult, temp)
	}
	return strings.Join(groupResult, " + ")
}

func (t *Time) stringGeneratorProduct() string {

	groupResult := []string{}
	for _, group := range t.Product {
		temp := group.String()

		if temp == "" {
			continue
		}

		if len(strings.Fields(temp)) > 1 && encloseString(temp) {
			groupResult = append(groupResult, fmt.Sprintf("[%v]", temp))
		} else {
			groupResult = append(groupResult, temp)
		}
	}

	result := strings.Join(groupResult, " * ")
	if result != "" {
		result = fmt.Sprintf("[%v]", result)
	}
	return result
}

// contains checks if t contains other
//
// If t and other are both refs, check if the refs are equal.
//
// If t is a ref and other is a collection of time objects, check that every
// item in other is contained in t.
//
// If t is a collection and other is a ref, check that other is contained in one
// of the time objects in t.
//
// If t and other are both collections, check that every object in other is
// contained in one of the time objects in t.
func (t *Time) contains(other *Time) bool {

	if other.isEmpty() {
		return true
	}

	if len(t.R) != 0 {
		if len(other.R) != 0 {
			return t.R.Equal(other.R)
		} else if len(other.Sum) != 0 {
			return containsComposite(t, other.Sum)
		} else if len(other.Product) != 0 {
			return containsComposite(t, other.Product)
		}
	} else if len(t.Sum) != 0 {
		if len(other.R) != 0 {
			for _, group := range t.Sum {
				if group.contains(other) {
					return true
				}
			}
		} else if len(other.Sum) != 0 {
			return containsComposite(t, other.Sum)
		} else if len(other.Product) != 0 {
			return containsComposite(t, other.Product)
		}
	} else if len(t.Product) != 0 {
		if len(other.R) != 0 {
			for _, group := range t.Product {
				if group.contains(other) {
					return true
				}
			}
		} else if len(other.Sum) != 0 {
			return containsComposite(t, other.Sum)
		} else if len(other.Product) != 0 {
			return containsComposite(t, other.Product)
		}
	}
	return false
}

func (t *Time) isEmpty() bool {
	return len(t.R) == 0 && len(t.Sum) == 0 && len(t.Product) == 0
}

func containsComposite(a *Time, b []Time) bool {
	for _, t := range b {
		if !a.contains(&t) {
			return false
		}
	}
	return true
}

// count holds results for count complexity of a term, var
// count complexity can be:
// 1) constant ie. O(1)
// 2) reference ie. represented in terms of base ref
// 3) product of the count complexity of the variables in the term
type count struct {
	r       ast.Ref
	product []count
}

func (c *count) countToSize() size {
	var result size

	if len(c.r) != 0 {
		result.r = c.r
	} else if len(c.product) != 0 {
		for _, cp := range c.product {
			result.product = append(result.product, cp.countToSize())
		}
	}
	return result
}

// size holds results for size complexity of a ref, term, var
// size complexity can be:
// 1) constant ie. O(1)
// 2) reference ie. represented in terms of base ref
// 3) sum of the size complexity of multiple rules
// 4) product of the size complexity of multiple expressions
type size struct {
	r       ast.Ref
	sum     []size
	product []size
}

func (s *size) sizeToTime() Time {
	var result Time

	if len(s.r) != 0 {
		result.R = s.r
	} else if len(s.sum) != 0 {
		for _, ss := range s.sum {
			result.Sum = append(result.Sum, ss.sizeToTime())
		}
	} else if len(s.product) != 0 {
		for _, sp := range s.product {
			result.Product = append(result.Product, sp.sizeToTime())
		}
	}
	return result
}

func (s *size) sizeToRef() ast.Ref {
	ref := make(ast.Ref, 0)

	if s == nil {
		return nil
	}

	if len(s.r) != 0 {
		for _, t := range s.r {
			ref = ref.Append(t)
		}
	} else if len(s.sum) != 0 {
		for _, ss := range s.sum {
			for _, t := range ss.sizeToRef() {
				ref = ref.Append(t)
			}
		}
	} else if len(s.product) != 0 {
		for _, ss := range s.product {
			for _, t := range ss.sizeToRef() {
				ref = ref.Append(t)
			}
		}
	}
	return ref
}

// Calculator provides the interface to initiate the runtime complexity
// calculation for a query
type Calculator struct {
	compiler   *ast.Compiler
	query      string
	parseQuery ast.Body
}

// New returns a new Calculator object
func New() *Calculator {
	return &Calculator{}
}

// WithCompiler sets the compiler to use for the calculator
func (c *Calculator) WithCompiler(compiler *ast.Compiler) *Calculator {
	c.compiler = compiler
	return c
}

// WithQuery sets the query to use for the calculator
func (c *Calculator) WithQuery(query string) *Calculator {
	c.query = query
	return c
}

// Calculate calculates the runtime complexity of the query and generates a report
func (c *Calculator) Calculate() (*Report, error) {
	report := Report{}
	report.calculator = c

	compiledQuery, err := c.compiler.QueryCompiler().Compile(ast.MustParseBody(c.query))
	if err != nil {
		return nil, err
	}

	if len(compiledQuery) == 0 {
		report.Complexity = nil
	} else {
		report.Complexity = newAnalyzeQuery(c.query, compiledQuery)

		err := c.analyzeQuery(report.Complexity)
		if err != nil {
			return nil, err
		}
	}
	return &report, nil
}

func (c *Calculator) analyzeQuery(a *analyzeQuery) error {

	for i, e := range a.body {
		err := c.analyzeExpr(a, e, i)
		if err != nil {
			return err
		}
	}

	// calculate query time complexity
	a.runtime()
	return nil
}

func (c *Calculator) analyzeExpr(a *analyzeQuery, expr *ast.Expr, idx int) error {

	switch t := expr.Terms.(type) {
	case []*ast.Term:
		if expr.IsEquality() {
			left, right := t[1], t[2]
			var err error

			if l, ok := left.Value.(ast.Var); ok {
				err = c.analyzeExprEquality(l, right, a, expr, idx)
			} else if r, ok := right.Value.(ast.Var); ok {
				err = c.analyzeExprEquality(r, left, a, expr, idx)
			}

			if err != nil {
				return err
			}
		} else {
			if _, ok := ast.BuiltinMap[expr.Operator().String()]; ok {
				//TODO builtins
			} else {
				err := c.analyzeExprUserDefinedFunctions(expr, a, idx)
				if err != nil {
					return err
				}
			}
		}
	case *ast.Term:
		err := c.analyzeExprSingleTerm(t, a, idx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Calculator) analyzeExprEquality(v ast.Var, t *ast.Term, a *analyzeQuery, expr *ast.Expr, idx int) error {

	switch x := t.Value.(type) {
	case ast.Null, ast.Boolean, ast.Number, ast.String, ast.Array, ast.Set, ast.Object, ast.Var:
		if a.binding.Get(v) == nil {
			addVarBinding(v, x, a)

			// set time, count, size complexity of var
			a.time[v] = nil
			a.count[v] = nil
			a.size[v] = nil

			y, ok := x.(ast.Var)
			if ok {
				a.size[v] = a.size[y]
			}

			arr, ok := x.(ast.Array)
			if ok {
				unifyArrayVar(arr, v, a, a)
			}

			set, ok := x.(ast.Set)
			if ok {
				unifySetVar(set, v, a, a)
			}

			obj, ok := x.(ast.Object)
			if ok {
				unifyObjVar(obj, v, a, a)
			}
		}
	case ast.Ref:
		if len(c.compiler.GetRulesDynamic(x)) != 0 {
			err := c.analyzeExprEqVarVirtualRef(v, x, a, idx)
			if err != nil {
				return err
			}
		} else {
			c.analyzeExprEqVarBaseRef(v, x, a, idx)
		}
	case *ast.ArrayComprehension, *ast.SetComprehension, *ast.ObjectComprehension:
		err := c.analyzeExprEqVarComprehension(v, x, a, idx)
		if err != nil {
			return err
		}
	default:
		a.Missing = append(a.Missing, expr.String())
	}
	return nil
}

func (c *Calculator) analyzeExprSingleTerm(t *ast.Term, a *analyzeQuery, idx int) error {

	switch x := t.Value.(type) {
	case ast.Var:
		a.time[x] = nil
		a.count[x] = nil
		a.size[x] = nil
	case ast.Ref:
		if len(c.compiler.GetRulesDynamic(x)) != 0 {
			_, err := c.analyzeExprVirtualRef(x, a, idx)
			if err != nil {
				return err
			}
		} else {
			c.analyzeExprBaseRef(x, a, idx)
		}
	case *ast.ArrayComprehension, *ast.SetComprehension, *ast.ObjectComprehension:
		_, err := c.analyzeExprComprehension(x, a, idx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Calculator) analyzeExprEqVarVirtualRef(v ast.Var, r ast.Ref, a *analyzeQuery, idx int) error {
	s, err := c.analyzeExprVirtualRef(r, a, idx)
	if err != nil {
		return err
	}

	// bind var on the lhs of the equality expression
	bindVarVirtualRef(v, r, s, a)
	return nil
}

func (c *Calculator) analyzeExprVirtualRef(r ast.Ref, a *analyzeQuery, idx int) ([]size, error) {
	t, s, m, err := c.getTimeAndSizeVirtualRef(r)
	if err != nil {
		return s, err
	}

	if len(m) != 0 {
		a.Missing = append(a.Missing, m...)
		return s, nil
	}

	setTimeAndSizeVirtualRef(r, a, idx, t, s)
	return s, nil
}

func (c *Calculator) getTimeAndSizeVirtualRef(r ast.Ref) ([]Time, []size, []string, error) {
	rules := c.compiler.GetRulesDynamic(r)
	timeComplexitySum := []Time{}
	sizeComplexitySum := []size{}
	missing := []string{}

	for _, rule := range rules {
		queryResult, err := c.getTimeComplexityRuleBody(rule.Body)
		if err != nil {
			return timeComplexitySum, sizeComplexitySum, missing, err
		}

		if queryResult == nil {
			continue
		}

		if len(queryResult.Missing) != 0 {
			missing = append(missing, queryResult.Missing...)
			continue
		}

		// time complexity for rule
		if queryResult.Complexity != nil {

			// check if the new time result is redundant to the overall time
			// complexity of the expression
			// For example: current = O(input.foo)
			//				new = O(input.foo)
			//				overall time result  = O(input.foo) + O(input.foo)
			//									 = O(input.foo)
			contains := false
			for _, t := range timeComplexitySum {
				if ctn := t.contains(queryResult.Complexity); ctn {
					contains = true
					break
				}
			}

			if !contains {
				timeComplexitySum = append(timeComplexitySum, *queryResult.Complexity)
			}
		}

		// size complexity for rule
		size := getSizeComplexityRule(rule, queryResult)
		if size != nil {
			sizeComplexitySum = append(sizeComplexitySum, *size)
		}
	}
	return timeComplexitySum, sizeComplexitySum, missing, nil
}

func (c *Calculator) analyzeExprEqVarBaseRef(v ast.Var, r ast.Ref, a *analyzeQuery, idx int) {
	c.analyzeExprBaseRef(r, a, idx)

	// bind var on the lhs of the equality expression
	bindVarBaseRef(v, r, a, a.relation[idx])
}

func (c *Calculator) analyzeExprBaseRef(r ast.Ref, a *analyzeQuery, idx int) {

	relation := false

	ast.WalkVars(r, func(x ast.Var) bool {
		if !isRootDocument(x) && a.binding.Get(x) == nil {
			relation = true
			bindVarBaseRef(x, r, a, relation)
		}
		return false
	})

	if relation {
		a.Expressions[idx] = getTimeInTermsOfBaseDoc(r, a)
		a.relation[idx] = true
	}
}

func (c *Calculator) analyzeExprEqVarComprehension(v ast.Var, val ast.Value, a *analyzeQuery, idx int) error {

	s, err := c.analyzeExprComprehension(val, a, idx)
	if err != nil {
		return err
	}

	// bind var on the lhs of the equality expression
	bindVarComprehension(v, *s, a)
	return nil
}

func (c *Calculator) analyzeExprComprehension(val ast.Value, a *analyzeQuery, idx int) (*size, error) {

	var head *ast.Term
	var body ast.Body
	var sizeHead size

	switch x := val.(type) {
	case *ast.ArrayComprehension:
		head = x.Term
		body = x.Body

	case *ast.SetComprehension:
		head = x.Term
		body = x.Body

	case *ast.ObjectComprehension:
		head = x.Key
		body = x.Body
	}

	// calculate time and size complexity
	queryResult, err := c.getTimeComplexityRuleBody(body)
	if err != nil {
		return nil, err
	}

	if queryResult != nil {
		a.Expressions[idx] = queryResult.Complexity
	}

	if head != nil {
		var countHead count
		seen := make(map[ast.Var]struct{})
		for av := range head.Vars() {
			countVar := getCountComplexityPartialRule(av, queryResult, seen)
			if countVar != nil {
				countHead.product = append(countHead.product, *countVar)
			}
		}

		// convert count complexity to size
		sizeHead = countHead.countToSize()
	}
	return &sizeHead, nil
}

func (c *Calculator) analyzeExprUserDefinedFunctions(e *ast.Expr, a *analyzeQuery, idx int) error {

	rules := c.compiler.GetRulesDynamic(e.Operator())
	timeComplexitySum := []Time{}
	sizeComplexitySum := []size{}

	for _, rule := range rules {

		queryResult := c.unify(e, rule, a)

		err := c.analyzeQuery(queryResult)
		if err != nil {
			return err
		}

		// time complexity for rule
		if queryResult.Complexity != nil {

			// check if the new time result is redundant to the overall time
			// complexity of the expression
			// For example: current = O(input.foo)
			//				new = O(input.foo)
			//				overall time result  = O(input.foo) + O(input.foo)
			//									 = O(input.foo)
			contains := false
			for _, t := range timeComplexitySum {
				if ctn := t.contains(queryResult.Complexity); ctn {
					contains = true
					break
				}
			}

			if !contains {
				timeComplexitySum = append(timeComplexitySum, *queryResult.Complexity)
			}
		}

		// size complexity for rule
		size := getSizeComplexityRule(rule, queryResult)
		if size != nil {
			sizeComplexitySum = append(sizeComplexitySum, *size)
		}
	}

	// expression time complexity
	a.Expressions[idx] = &Time{Sum: timeComplexitySum}

	for _, s := range sizeComplexitySum {
		sTot := s.sizeToTime()
		contains := a.Expressions[idx].contains(&sTot)

		// include size complexity in the overall time complexity result of
		// the expression only if it adds to the overall result. This check
		// prevents addition of redundant values to the overall time result.
		// For example: time = O(input.foo * input.bar)
		//				size = O(input.foo)
		//				overall time result  = O(input.foo * input.bar) + O(input.foo)
		//									 = O(input.foo * input.bar)
		if !contains {
			a.Expressions[idx].Sum = append(a.Expressions[idx].Sum, sTot)
		}
	}

	// set the size complexity of the output variable of the function
	last := e.Operand(len(e.Operands()) - 1)
	if last != nil {
		switch x := last.Value.(type) {
		case ast.Var:

			// time complexity
			a.time[x] = nil

			// count complexity
			a.count[x] = nil

			// size complexity
			a.size[x] = &size{sum: sizeComplexitySum}
		}
	}
	return nil
}

// unify unifies the arguments at the call-site and the function definition
func (c *Calculator) unify(e *ast.Expr, r *ast.Rule, a *analyzeQuery) *analyzeQuery {

	operands := e.Operands()
	result := newAnalyzeQuery(c.query, r.Body)

	for i := 0; i < len(r.Head.Args); i++ {
		unifyTerms(e, operands[i], r.Head.Args[i], a, result)
	}
	return result
}

func (c *Calculator) getTimeComplexityRuleBody(b ast.Body) (*analyzeQuery, error) {

	if len(b) == 0 {
		return nil, nil
	}

	a := newAnalyzeQuery(c.query, b)

	err := c.analyzeQuery(a)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// helper functions

func unifyTerms(e *ast.Expr, a, b *ast.Term, aCurr, aNew *analyzeQuery) {

	switch x := a.Value.(type) {
	case ast.Null, ast.Boolean, ast.Number, ast.String:
		switch y := b.Value.(type) {
		case ast.Var:
			aNew.time[y] = nil
			aNew.size[y] = nil
			aNew.count[y] = nil
		case ast.Array, ast.Set, ast.Object:
			aCurr.Missing = append(aCurr.Missing, e.String())
		}
	case ast.Var:
		switch y := b.Value.(type) {
		case ast.Var:
			aNew.time[y] = aCurr.time[x]
			aNew.size[y] = aCurr.size[x]
			aNew.count[y] = aCurr.count[x]
		case ast.Array:
			bindVal := aCurr.binding.Get(x)
			if bindVal != nil {
				if arr, ok := bindVal.(ast.Array); ok {
					if len(arr) != len(y) {
						aCurr.Missing = append(aCurr.Missing, e.String())
					}
					unifySlice(e, arr, y, aCurr, aNew)
				}
			}
		case ast.Set:
			bindVal := aCurr.binding.Get(x)
			if bindVal != nil {
				if set, ok := bindVal.(ast.Set); ok {
					if set.Len() != y.Len() {
						aCurr.Missing = append(aCurr.Missing, e.String())
					}
					unifySlice(e, set.Slice(), y.Slice(), aCurr, aNew)
				}
			}
		case ast.Object:
			bindVal := aCurr.binding.Get(x)
			if bindVal != nil {
				if obj, ok := bindVal.(ast.Object); ok {
					if obj.Len() != y.Len() {
						aCurr.Missing = append(aCurr.Missing, e.String())
					}
					unifySlice(e, obj.Keys(), y.Keys(), aCurr, aNew)
				}
			}
		}
	case ast.Array:
		switch y := b.Value.(type) {
		case ast.Null, ast.Boolean, ast.Number, ast.String:
			aCurr.Missing = append(aCurr.Missing, e.String())
		case ast.Var:
			unifyArrayVar(x, y, aCurr, aNew)
		case ast.Array:
			if len(x) != len(y) {
				aCurr.Missing = append(aCurr.Missing, e.String())
			}
			unifySlice(e, x, y, aCurr, aNew)
		}
	case ast.Set:
		switch y := b.Value.(type) {
		case ast.Null, ast.Boolean, ast.Number, ast.String:
			aCurr.Missing = append(aCurr.Missing, e.String())
		case ast.Var:
			unifySetVar(x, y, aCurr, aNew)
		case ast.Set:
			if x.Len() != y.Len() {
				aCurr.Missing = append(aCurr.Missing, e.String())
			}
			unifySlice(e, x.Slice(), y.Slice(), aCurr, aNew)
		}
	case ast.Object:
		switch y := b.Value.(type) {
		case ast.Null, ast.Boolean, ast.Number, ast.String:
			aCurr.Missing = append(aCurr.Missing, e.String())
		case ast.Var:
			unifyObjVar(x, y, aCurr, aNew)
		case ast.Object:
			if x.Len() != y.Len() {
				aCurr.Missing = append(aCurr.Missing, e.String())
			}
			unifySlice(e, x.Keys(), y.Keys(), aCurr, aNew)
		}
	}
	return
}

// unifyArrayVar unifies an array argument at the call-site with a variable
// at the function definition. The size complexity of the variable
// is the sum of the size complexity of the array's terms
func unifyArrayVar(a ast.Array, v ast.Var, aCurr, aNew *analyzeQuery) {
	aNew.time[v] = nil
	aNew.size[v] = &size{}
	aNew.count[v] = nil
	addVarBinding(v, a, aNew)

	for _, e := range a {
		if av, ok := e.Value.(ast.Var); ok {
			aNew.size[v].sum = append(aNew.size[v].sum, *aCurr.size[av])
		}
	}
	return
}

// unifySetVar unifies a set argument at the call-site with a variable
// at the function definition. The size complexity of the variable
// is the sum of the size complexity of the set's terms
func unifySetVar(s ast.Set, v ast.Var, aCurr, aNew *analyzeQuery) {
	aNew.time[v] = nil
	aNew.size[v] = &size{}
	aNew.count[v] = nil
	addVarBinding(v, s, aNew)

	s.Foreach(func(x *ast.Term) {
		if sv, ok := x.Value.(ast.Var); ok {
			aNew.size[v].sum = append(aNew.size[v].sum, *aCurr.size[sv])
		}
	})
	return
}

// unifyObjVar unifies an object argument at the call-site with a variable
// at the function definition. The size complexity of the variable
// is the sum of the size complexity of the object's keys
func unifyObjVar(o ast.Object, v ast.Var, aCurr, aNew *analyzeQuery) {
	aNew.time[v] = nil
	aNew.size[v] = &size{}
	aNew.count[v] = nil
	addVarBinding(v, o, aNew)

	for _, e := range o.Keys() {
		if av, ok := e.Value.(ast.Var); ok {
			aNew.size[v].sum = append(aNew.size[v].sum, *aCurr.size[av])
		}
	}
	return
}

func unifySlice(expr *ast.Expr, a, b []*ast.Term, aCurr, aNew *analyzeQuery) {
	for i := range a {
		unifyTerms(expr, a[i], b[i], aCurr, aNew)
	}
	return
}

func bindVarVirtualRef(v ast.Var, bindVal ast.Ref, complexityVal []size, a *analyzeQuery) {
	if a.binding.Get(v) == nil {
		addVarBinding(v, bindVal.GroundPrefix(), a)
		setComplexityVarVirtualRef(v, a, complexityVal)
	}
}

func bindVarBaseRef(v ast.Var, bindVal ast.Ref, a *analyzeQuery, isRelation bool) {
	if a.binding.Get(v) == nil {
		addVarBinding(v, bindVal.GroundPrefix(), a)
		setComplexityVarBaseRef(v, bindVal.GroundPrefix(), a, isRelation)
	}
}

func bindVarComprehension(v ast.Var, complexityVal size, a *analyzeQuery) {
	if a.binding.Get(v) == nil {
		addVarBinding(v, ast.StringTerm("Comprehension").Value, a)
		setComplexityVarComprehension(v, &complexityVal, a)
	}
}

func addVarBinding(k, v ast.Value, a *analyzeQuery) {
	a.binding.Put(k, v)
}

func setTimeAndSizeVirtualRef(r ast.Ref, a *analyzeQuery, idx int, t []Time, s []size) {

	// expression time complexity
	a.Expressions[idx] = &Time{Sum: t}

	relation := false
	ast.WalkVars(r, func(x ast.Var) bool {
		if !isRootDocument(x) && a.binding.Get(x) == nil {
			relation = true
			bindVarVirtualRef(x, r, s, a)
		}
		return false
	})

	if relation {
		for _, e := range s {
			sTot := e.sizeToTime()
			contains := a.Expressions[idx].contains(&sTot)

			// include size complexity in the overall time complexity result of
			// the expression only if it adds to the overall result. This check
			// prevents addition of redundant values to the overall time result.
			// For example: time = O(input.foo * input.bar)
			//				size = O(input.foo)
			//				overall time result  = O(input.foo * input.bar) + O(input.foo)
			//									 = O(input.foo * input.bar)
			if !contains {
				a.Expressions[idx].Sum = append(a.Expressions[idx].Sum, sTot)
			}
		}

		a.relation[idx] = true
	}
}

func setComplexityVarVirtualRef(v ast.Var, a *analyzeQuery, sizeComplexitySum []size) {

	// time complexity
	a.time[v] = nil

	// count complexity
	a.count[v] = nil

	// size complexity
	a.size[v] = &size{sum: sizeComplexitySum}
}

func setComplexityVarBaseRef(v ast.Var, r ast.Ref, a *analyzeQuery, isRelation bool) {

	// time complexity
	a.time[v] = nil

	// count complexity
	a.count[v] = nil

	if isRelation {
		a.count[v] = getCountInTermsOfBaseDoc(r, a)
	}

	// size complexity
	a.size[v] = getSizeInTermsOfBaseDoc(r, a)
}

func setComplexityVarComprehension(v ast.Var, s *size, a *analyzeQuery) {

	// time complexity
	a.time[v] = nil

	// count complexity
	a.count[v] = nil

	// size complexity
	a.size[v] = s
}

func getSizeComplexityRule(r *ast.Rule, a *analyzeQuery) *size {

	switch r.Head.DocKind() {
	case ast.CompleteDoc:
		switch x := r.Head.Value.Value.(type) {
		case ast.Var:
			seen := make(map[ast.Var]struct{})
			return getSizeComplexityCompleteRule(x, a, seen)
		}
	case ast.PartialSetDoc, ast.PartialObjectDoc:
		key := r.Head.Key
		if key != nil {
			var countHead count
			seen := make(map[ast.Var]struct{})
			for v := range key.Vars() {
				countVar := getCountComplexityPartialRule(v, a, seen)
				if countVar != nil {
					countHead.product = append(countHead.product, *countVar)
				}
			}
			// convert count complexity to size
			sizeHead := countHead.countToSize()
			return &sizeHead
		}
	default:
		panic("illegal rule kind")
	}
	return nil
}

func getSizeComplexityCompleteRule(v ast.Var, a *analyzeQuery, seen map[ast.Var]struct{}) *size {

	if _, ok := seen[v]; ok {
		return nil
	}

	seen[v] = struct{}{}

	var result *size
	var ok bool
	if result, ok = a.size[v]; !ok {
		return nil
	}

	if result != nil {
		return result
	}

	// Check if the variable is bound to another variable.
	// If it is, get the size complexity of the assignment
	boundVal := a.binding.Get(v)
	if boundVal != nil {
		switch y := boundVal.(type) {
		case ast.Var:
			return getSizeComplexityCompleteRule(y, a, seen)
		}
	}
	return nil
}

func getCountComplexityPartialRule(v ast.Var, a *analyzeQuery, seen map[ast.Var]struct{}) *count {

	if _, ok := seen[v]; ok {
		return nil
	}

	seen[v] = struct{}{}

	var result *count
	var ok bool
	if result, ok = a.count[v]; !ok {
		return nil
	}

	if result != nil {
		return result
	}

	// Check if the variable is bound to another variable.
	// If it is, get the count complexity of the assignment
	boundVal := a.binding.Get(v)
	if boundVal != nil {
		switch y := boundVal.(type) {
		case ast.Var:
			return getCountComplexityPartialRule(y, a, seen)
		}
	}
	return nil
}

func getTimeInTermsOfBaseDoc(r ast.Ref, a *analyzeQuery) *Time {
	if len(r) == 0 {
		return nil
	}

	s := getSizeInTermsOfBaseDoc(r, a)
	t := s.sizeToTime()
	return &t
}

func getCountInTermsOfBaseDoc(r ast.Ref, a *analyzeQuery) *count {
	if len(r) == 0 {
		return nil
	}

	s := getSizeInTermsOfBaseDoc(r, a)
	return &count{
		r: s.sizeToRef(),
	}
}

func getSizeInTermsOfBaseDoc(r ast.Ref, a *analyzeQuery) *size {
	if len(r) == 0 {
		return nil
	}

	switch x := r[0].Value.(type) {
	case ast.Var:
		if isRootDocument(x) || a.size[x] == nil {
			return &size{r: r.GroundPrefix()}
		}

		if len(a.size[x].r) != 0 {
			return getSizeInTermsOfBaseDoc(a.size[x].r, a)
		}
		return a.size[x]
	}
	return &size{r: r.GroundPrefix()}
}

func encloseString(s string) bool {
	return !(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

func isRootDocument(v ast.Var) bool {
	return v.Equal(ast.InputRootDocument.Value) || v.Equal(ast.DefaultRootDocument.Value)
}
