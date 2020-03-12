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
	buf := fmt.Sprintf("\nComplexity Results for query \"%v\":\n", queryRes.Query.String())

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
// relation    -> whether the expression is a relation or not
//			      An expression is a relation if the reference it contains
// 			      has first occurrence of a variable. eg. p[x]
// time        -> time complexity of each variable in query
// count       -> count complexity of each variable in query
// size        -> size complexity of each variable in query
// binding     -> map of variable to the value it refers to
// complexity  -> runtime complexity of query
type analyzeQuery struct {
	Query       ast.Body `json:"query"`
	Expressions []*time  `json:"expressions,omitempty"`
	Missing     []string `json:"missing,omitempty"`
	relation    []bool
	time        map[ast.Var]*time
	size        map[ast.Var]*size
	count       map[ast.Var]*count
	binding     *ast.ValueMap
	complexity  *time `json:"complexity"`
}

func newAnalyzeQuery(query ast.Body) *analyzeQuery {
	return &analyzeQuery{
		Query:       query,
		Expressions: make([]*time, len(query)),
		Missing:     []string{},
		relation:    make([]bool, len(query)),
		time:        make(map[ast.Var]*time),
		size:        make(map[ast.Var]*size),
		count:       make(map[ast.Var]*count),
		binding:     ast.NewValueMap(),
		complexity:  nil,
	}
}

// String returns the string representation of the runtime complexity
func (q *analyzeQuery) String() string {
	if q.complexity == nil {
		return ""
	}
	return q.complexity.String()
}

// runtime calculates the runtime complexity of a query
// Time(Body) = Time(ExprN) * [Time(ExprN+1) * ...] when ExprN is a relation
// Time(Body) = Time(ExprN) + [Time(ExprN+1) + ...] when ExprN is not a relation
func runtime(q *analyzeQuery) *time {
	var isProduct bool
	var result time

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
	return &result
}

func product(a, b time) time {
	var result time
	result.product = append(result.product, b) // add existing
	result.product = append(result.product, a) // add new
	return result
}

func sum(a, b time) time {
	var result time
	result.sum = append(result.sum, b) // add existing
	result.sum = append(result.sum, a) // add new
	return result
}

// time holds results for time complexity
// time complexity can be:
// 1) constant ie. O(1)
// 2) reference ie. represented in terms of base ref
// 3) sum of the time complexity of multiple rules/expressions
// 4) product of the time complexity of multiple expressions
type time struct {
	r       ast.Ref
	sum     []time
	product []time
}

func (t *time) String() string {
	var result string

	if len(t.r) != 0 {
		result = t.r.String()
	} else if len(t.sum) != 0 {
		result = t.stringGeneratorSum()
	} else if len(t.product) != 0 {
		result = t.stringGeneratorProduct()
	}
	return result
}

func (t *time) stringGeneratorSum() string {

	groupResult := []string{}
	for _, group := range t.sum {
		temp := group.String()

		if temp == "" {
			continue
		}

		groupResult = append(groupResult, temp)
	}
	return strings.Join(groupResult, " + ")
}

func (t *time) stringGeneratorProduct() string {

	groupResult := []string{}
	for _, group := range t.product {
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
func (t *time) contains(other *time) bool {

	if other.isEmpty() {
		return true
	}

	if len(t.r) != 0 {
		if len(other.r) != 0 {
			return t.r.Equal(other.r)
		} else if len(other.sum) != 0 {
			return containsComposite(t, other.sum)
		} else if len(other.product) != 0 {
			return containsComposite(t, other.product)
		}
	} else if len(t.sum) != 0 {
		if len(other.r) != 0 {
			for _, group := range t.sum {
				if group.contains(other) {
					return true
				}
			}
		} else if len(other.sum) != 0 {
			return containsComposite(t, other.sum)
		} else if len(other.product) != 0 {
			return containsComposite(t, other.product)
		}
	} else if len(t.product) != 0 {
		if len(other.r) != 0 {
			for _, group := range t.product {
				if group.contains(other) {
					return true
				}
			}
		} else if len(other.sum) != 0 {
			return containsComposite(t, other.sum)
		} else if len(other.product) != 0 {
			return containsComposite(t, other.product)
		}
	}
	return false
}

func (t *time) isEmpty() bool {
	return len(t.r) == 0 && len(t.sum) == 0 && len(t.product) == 0
}

func containsComposite(a *time, b []time) bool {
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

func (s *size) sizeToTime() time {
	var result time

	if len(s.r) != 0 {
		result.r = s.r
	} else if len(s.sum) != 0 {
		for _, ss := range s.sum {
			result.sum = append(result.sum, ss.sizeToTime())
		}
	} else if len(s.product) != 0 {
		for _, sp := range s.product {
			result.product = append(result.product, sp.sizeToTime())
		}
	}
	return result
}

// Calculator provides the interface to initiate the runtime complexity
// calculation for a query
type Calculator struct {
	compiler *ast.Compiler
	query    ast.Body
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
	c.query = ast.MustParseBody(query)
	return c
}

// WithParsedQuery sets the parsed query to use for the calculator
func (c *Calculator) WithParsedQuery(query ast.Body) *Calculator {
	c.query = query
	return c
}

// Calculate calculates the runtime complexity of the query and generates a report
func (c *Calculator) Calculate() (*Report, error) {
	report := Report{}
	report.calculator = c

	compiledQuery, err := c.compiler.QueryCompiler().Compile(c.query)
	if err != nil {
		return nil, err
	}

	report.Complexity, err = c.analyzeQuery(compiledQuery)
	if err != nil {
		return nil, err
	}

	return &report, nil
}

func (c *Calculator) analyzeQuery(body ast.Body) (*analyzeQuery, error) {

	if len(body) == 0 {
		return nil, nil
	}

	analyzeQueryResult := newAnalyzeQuery(c.query)

	for i, e := range body {
		err := c.analyzeExpr(analyzeQueryResult, e, i)
		if err != nil {
			return nil, err
		}
	}

	// calculate query time complexity
	analyzeQueryResult.complexity = runtime(analyzeQueryResult)

	return analyzeQueryResult, nil
}

func (c *Calculator) analyzeExpr(a *analyzeQuery, expr *ast.Expr, idx int) error {

	switch expr.Terms.(type) {
	case []*ast.Term:
		if expr.IsEquality() {
			switch x := expr.Operands()[0].Value.(type) {
			case ast.Var:
				switch y := expr.Operands()[1].Value.(type) {
				case ast.Null, ast.Boolean, ast.Number, ast.String, ast.Array, ast.Set, ast.Object, ast.Var:
					if a.binding.Get(x) == nil {
						addVarBinding(x, y, a)

						// set time, count, size complexity of var
						a.time[x] = nil
						a.count[x] = nil
						a.size[x] = nil
					}
				case ast.Ref:
					if len(c.compiler.GetRulesDynamic(y)) != 0 {
						err := c.analyzeExprEqVarVirtualRef(x, y, a, idx)
						if err != nil {
							return err
						}
					} else {
						c.analyzeExprEqVarBaseRef(x, y, a, idx)
					}
					return nil
				case *ast.ArrayComprehension, *ast.ObjectComprehension, *ast.SetComprehension:
					//TODO
				default:
					a.Missing = append(a.Missing, expr.String())
					return nil
				}
			}
		} else {
			// TODO: functions and builtins
		}
	case *ast.Term:
		// TODO
	}
	return nil
}

func (c *Calculator) analyzeExprEqVarVirtualRef(v ast.Var, r ast.Ref, a *analyzeQuery, idx int) error {

	rules := c.compiler.GetRulesDynamic(r)
	timeComplexitySum := []time{}
	sizeComplexitySum := []size{}

	for _, rule := range rules {
		queryResult, err := getTimeComplexityRuleBody(rule.Body, c.compiler)
		if err != nil {
			return err
		}

		// time complexity for rule
		if queryResult.complexity != nil {

			// check if the new time result is redundant to the overall time
			// complexity of the expression
			// For example: current = O(input.foo)
			//				new = O(input.foo)
			//				overall time result  = O(input.foo) + O(input.foo)
			//									 = O(input.foo)
			contains := false
			for _, t := range timeComplexitySum {
				if ctn := t.contains(queryResult.complexity); ctn {
					contains = true
					break
				}
			}

			if !contains {
				timeComplexitySum = append(timeComplexitySum, *queryResult.complexity)
			}
		}

		// size complexity for rule
		size := getSizeComplexityRule(rule, queryResult)
		if size != nil {
			sizeComplexitySum = append(sizeComplexitySum, *size)
		}
	}

	// expression time complexity
	a.Expressions[idx] = &time{sum: timeComplexitySum}

	relation := false
	ast.WalkVars(r, func(x ast.Var) bool {
		if !isRootDocument(x) && a.binding.Get(x) == nil {
			relation = true
			bindVarVirtualRef(x, r, sizeComplexitySum, a)
		}
		return false
	})

	if relation {
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
				a.Expressions[idx].sum = append(a.Expressions[idx].sum, sTot)
			}
		}

		a.relation[idx] = true
	}

	// bind var on the lhs of the equality expression
	bindVarVirtualRef(v, r, sizeComplexitySum, a)

	return nil
}

func (c *Calculator) analyzeExprEqVarBaseRef(v ast.Var, r ast.Ref, a *analyzeQuery, idx int) {

	relation := false
	ast.WalkVars(r, func(x ast.Var) bool {
		if !isRootDocument(x) && a.binding.Get(x) == nil {
			relation = true
			bindVarBaseRef(x, r, a, relation)
		}
		return false
	})

	if relation {
		a.Expressions[idx] = &time{r: r.GroundPrefix()}
		a.relation[idx] = true
	}

	// bind var on the lhs of the equality expression
	bindVarBaseRef(v, r, a, relation)
}

// helper functions

func bindVarVirtualRef(v ast.Var, bindVal ast.Ref, complexityVal []size, a *analyzeQuery) {
	if a.binding.Get(v) == nil {
		addVarBinding(v, bindVal.GroundPrefix(), a)
		setComplexityVarVirtualRef(v, a, complexityVal)
	}
}

func bindVarBaseRef(v ast.Var, bindVal ast.Ref, a *analyzeQuery, isRelation bool) {
	addVarBinding(v, bindVal.GroundPrefix(), a)
	setComplexityVarBaseRef(v, bindVal.GroundPrefix(), a, isRelation)
}

func addVarBinding(k, v ast.Value, a *analyzeQuery) {
	a.binding.Put(k, v)
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
		a.count[v] = &count{r: r}
	}

	// size complexity
	a.size[v] = &size{r: r}
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

func getTimeComplexityRuleBody(b ast.Body, c *ast.Compiler) (*analyzeQuery, error) {
	complexityCalculator := New().WithCompiler(c).WithParsedQuery(b)
	report, err := complexityCalculator.Calculate()
	return report.Complexity, err
}

func encloseString(s string) bool {
	return !(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

func isRootDocument(v ast.Var) bool {
	return v.Equal(ast.InputRootDocument.Value) || v.Equal(ast.DefaultRootDocument.Value)
}
