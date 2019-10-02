// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package analyze

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/open-policy-agent/opa/ast"
)

// analyzeResult holds the result of rule analysis that groups the variables
// in a rule into one-value and many-value buckets.
// one-value: Variable can have only one value
// many-value: Variable can be assigned multiple values
type analyzeResult struct {
	Raw          *ast.Rule
	Rule         string
	OneValue     map[string]ast.Value
	ManyValue    map[string]string
	RevManyValue map[string]string
	Error        error
	Missing      []string
	Iterates     bool
}

// analyzeExpression holds the result of expression analysis
type analyzeExpression struct {
	Raw            *ast.Expr
	Count          string
	Time           string
	TimeComplexity string
}

// RuntimeComplexityResult holds the result for runtime complexity of rules
type RuntimeComplexityResult struct {
	Result  map[string][]string
	Missing map[string][]string
}

var (
	varRegexp        = regexp.MustCompile("^[[:alpha:]_][[:alpha:][:digit:]_]*$")
	virtualDocRegexp = regexp.MustCompile(`{[^}]*}`)
	underscoreCount  = 0

	varVisitorParams = ast.VarVisitorParams{
		SkipRefCallHead: true,
		SkipRefHead:     true,
	}
)

// CalculateRuntimeComplexity returns the runtime complexity for the rules
// in the given module
func CalculateRuntimeComplexity(m *ast.Module) *RuntimeComplexityResult {
	results := []*analyzeResult{}
	finalResult := RuntimeComplexityResult{}
	finalResult.Result = make(map[string][]string)
	finalResult.Missing = make(map[string][]string)

	// Walk over the rules in the module and record the variables in them.
	// Categorize those variables into 2 groups:
	// 1. Variables which have only one value. eg. x := 1
	// 2. Variables that can be assigned multiple values. eg. p[x]
	ast.WalkRules(m, func(r *ast.Rule) bool {
		result := analyzeResult{}
		result.Raw = r
		result.Rule = r.Head.Name.String()
		result.OneValue = make(map[string]ast.Value)
		result.ManyValue = make(map[string]string)
		result.RevManyValue = make(map[string]string)
		seen := make(map[string]bool)
		for _, expr := range r.Body {
			analyzeExpr(expr, seen, &result)
		}
		results = append(results, &result)

		return false
	})

	resultsPre := runtimeComplexityRule(m, results, finalResult.Missing)
	finalResult.Result = postProcessComplexityResults(resultsPre)
	return &finalResult
}

// postProcessComplexityResults walks through the complexity results and
// replaces results with rule names with their actual complexity results
//
// Example:
// Rule: allow  Complexity: O(foo)
// Rule: foo    Complexity: O(input.bar)
//
// becomes
//
// Rule: allow  Complexity: O(input.bar)
// Rule: foo    Complexity: O(input.bar)
func postProcessComplexityResults(preRes map[string][]string) map[string][]string {

	postRes := make(map[string][]string)

	for ruleName, results := range preRes {
		for _, result := range results {
			matches := virtualDocRegexp.FindAllString(result, -1)

			if matches == nil || len(matches) == 0 {
				postRes[ruleName] = append(postRes[ruleName], result)
				continue
			}

			for _, match := range matches {
				fullName := strings.Trim(match, "{O()}")
				name := strings.Split(fullName, ".")
				rule := name[len(name)-1]

				replaceWith := postProcessComplexityResultsHelper(rule, preRes)
				result = strings.Replace(result, match, replaceWith, -1)
			}
			postRes[ruleName] = append(postRes[ruleName], result)
		}
	}
	return postRes
}

func postProcessComplexityResultsHelper(ruleName string, preRes map[string][]string) string {

	var ok bool
	var values []string
	if values, ok = preRes[ruleName]; !ok {
		panic(fmt.Sprintf("Rule %v not found in pre time complexity result map", ruleName))
	}

	result := []string{}
	for _, val := range values {
		matches := virtualDocRegexp.FindAllString(val, -1)

		if matches == nil || len(matches) == 0 {
			result = append(result, val)
		} else {
			for _, match := range matches {
				fullName := strings.Trim(match, "{O()}")
				name := strings.Split(fullName, ".")
				rule := name[len(name)-1]

				replaceWith := postProcessComplexityResultsHelper(rule, preRes)
				val = strings.Replace(val, match, replaceWith, -1)
				result = append(result, val)
			}
		}
	}
	if len(result) > 1 {
		return fmt.Sprintf("[%v]", strings.Join(result, " + "))
	}
	return strings.Join(result, "")
}

// runtimeComplexityRule walks over the rules in a module and returns their
// runtime complexity
func runtimeComplexityRule(m *ast.Module, analyzeResults []*analyzeResult, missing map[string][]string) map[string][]string {
	analyzeResultsCopy := make([]*analyzeResult, len(analyzeResults))
	copy(analyzeResultsCopy, analyzeResults)

	complexityResults := make(map[string][]string)
	var ruleAnalyzeResult analyzeResult

	// walk over the rules, to calculate the time complexity
	ast.WalkRules(m, func(r *ast.Rule) bool {
		var complexity string

		// check if rule has body
		if len(r.Body) == 0 {
			complexity = "O(1)"
		} else {
			// TimeComplexity(rule) = TimeComplexity(body)
			// TODO: Make this a map to optimize rule lookup
			i := 0
			var item *analyzeResult
			for i, item = range analyzeResultsCopy {
				if item.Rule == r.Head.Name.String() {
					ruleAnalyzeResult = *item
					break
				}
			}

			analyzeResultsCopy = append(analyzeResultsCopy[:i], analyzeResultsCopy[i+1:]...)
			complexity = runtimeComplexityBody(r.Body, &ruleAnalyzeResult, analyzeResults)
		}

		if len(ruleAnalyzeResult.Missing) == 0 {
			complexityResults[r.Head.Name.String()] = append(complexityResults[r.Head.Name.String()], strings.TrimSpace(complexity))
		} else {
			missing[r.Head.Name.String()] = ruleAnalyzeResult.Missing
		}

		return false
	})

	return complexityResults
}

// runtimeComplexityBody analyzes the expressions in the rule body and returns
// the runtime complexity for the rule
func runtimeComplexityBody(body ast.Body, analyzeRuleResult *analyzeResult, all []*analyzeResult) string {

	results := []analyzeExpression{}
	for _, expr := range body {
		result := analyzeExpression{}
		result.Raw = expr
		runtimeComplexityExpr(expr, &result, analyzeRuleResult, all)
		results = append(results, result)
	}

	// stitch the time complexity results for the rule by walking over the
	// expressions in the rule body
	var resultComplexity string

	for i := len(results) - 1; i >= 0; i-- {

		if results[i].TimeComplexity == "" {
			continue
		}
		if len(resultComplexity) == 0 && results[i].TimeComplexity != "O(1)" {
			resultComplexity += fmt.Sprintf("%v", results[i].TimeComplexity)
		} else {
			if ast.ContainsRefs(results[i].Raw) {
				if results[i].TimeComplexity != "O(1)" {
					resultComplexity = fmt.Sprintf("[ %v * %v ]", results[i].TimeComplexity, resultComplexity)
				}

			} else {
				if results[i].TimeComplexity != "O(1)" {
					resultComplexity = fmt.Sprintf("[ %v + %v ]", results[i].TimeComplexity, resultComplexity)
				}
			}
		}
	}

	if resultComplexity == "" {
		resultComplexity = "O(1)"
	}

	// remove end brackets
	resultComplexity = strings.TrimPrefix(resultComplexity, "[")
	return strings.TrimSuffix(resultComplexity, "]")
}

// runtimeComplexityExpr analyzes an expression and returns its runtime complexity
func runtimeComplexityExpr(expr *ast.Expr, analyzeExprResult *analyzeExpression, analyzeRuleResult *analyzeResult, all []*analyzeResult) {

	if expr.IsGround() && !ast.ContainsRefs(expr) {
		analyzeExprResult.Count = "O(1)"
		analyzeExprResult.Time = "O(1)"

		analyzeExprResult.TimeComplexity = "O(1)"
	} else if expr.IsEquality() {
		analyzeExprResult.Count = "O(1)"

		temp := []string{}
		for _, term := range expr.Operands() {
			result := runtimeComplexityTerm(term, analyzeRuleResult, all)
			if result != "" && result != "O(1)" {
				temp = append(temp, result)
			}
		}

		if len(temp) == 0 {
			temp = append(temp, "O(1)")
		}
		analyzeExprResult.Time = strings.Join(temp, " + ")

		analyzeExprResult.TimeComplexity = analyzeExprResult.Time

	} else if expr.IsAssignment() {
		analyzeExprResult.Count = "O(1)"
		analyzeExprResult.Time = runtimeComplexityTerm(expr.Operands()[1], analyzeRuleResult, all)

		analyzeExprResult.TimeComplexity = analyzeExprResult.Time

	} else if expr.IsCall() {
		operator := expr.Operator().String()

		_, ok := ast.BuiltinMap[operator]
		if ok && operator != ast.WalkBuiltin.Name {
			analyzeExprResult.Count = "O(1)"
			analyzeExprResult.Time = "O(1)"

			if operator == ast.Assign.Name {
				analyzeExprResult.Time = runtimeComplexityTerm(expr.Operands()[1], analyzeRuleResult, all)
			}

			analyzeExprResult.TimeComplexity = analyzeExprResult.Time
		} else {
			analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, expr.String())
		}

	} else if ast.ContainsRefs(expr) && !ast.ContainsComprehensions(expr) {
		analyzeExprResult.Time = "O(1)"

		switch terms := expr.Terms.(type) {
		case []*ast.Term:
			temp := []string{}
			for _, term := range terms {
				result := runtimeComplexityTerm(term, analyzeRuleResult, all)
				if result != "" && result != "O(1)" {
					temp = append(temp, result)
				}
			}

			if len(temp) == 0 {
				temp = append(temp, "O(1)")
			}

			analyzeExprResult.Count = strings.Join(temp, " * ")
		case *ast.Term:
			analyzeExprResult.Count = runtimeComplexityTerm(terms, analyzeRuleResult, all)
		}

		analyzeExprResult.TimeComplexity = analyzeExprResult.Count

	} else if ast.ContainsComprehensions(expr) {
		analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, expr.String())
	} else {
		analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, expr.String())
	}
}

// runtimeComplexityTerm returns the runtime complexity of a term
func runtimeComplexityTerm(term *ast.Term, analyzeRuleResult *analyzeResult, all []*analyzeResult) string {

	if term.IsGround() && !ast.ContainsRefs(term) {
		return "O(1)"
	}

	vars := getTermVars(term, varVisitorParams)

	switch x := term.Value.(type) {
	case ast.Ref:
		// virtual-doc reference
		if len(isVirtualDoc(x.GroundPrefix().String(), all)) != 0 {
			// Time(virtual-object) + Count(x.GroundPrefix()) => O(1) + Count(x.GroundPrefix())
			return fmt.Sprintf("{O(%v)}", x.GroundPrefix())
		}

		// strict data reference
		temp := []string{}
		for v := range vars {
			var value string

			// resolve underscores
			if v.String() == "_" {
				for vr, iter := range analyzeRuleResult.ManyValue {
					if isGenerated(vr) {
						value = iter
						delete(analyzeRuleResult.ManyValue, vr)
						break
					}
				}
			} else {
				value = analyzeRuleResult.ManyValue[v.String()]
			}

			// check if a reference in the many value bucket is a prefix to this
			// reference
			// case: input.foo[y].bar[z]
			// Complexity is O(input.foo) and not O(input.foo.bar)
			// Reference: input.foo[y].bar
			// RevManyValue: map[input.foo:y input.foo[y].bar:z]

			// check if a variable in the one value bucket is a prefix to this
			// reference
			// case: x := input.foo[y]
			//		 x.bar[z]
			// Complexity is O(input.foo) and not O(input.foo) * O(x.bar)
			// Reference: x.bar
			// OnveValue: map[x:<nil>]

			// The existence of a prefix to the given reference implies that we
			// have an original input/data rooted reference that will contribute
			// to the complexity of the term

			if isPrefixInManyVal(value, analyzeRuleResult.RevManyValue) ||
				isPrefixInOneVal(value, analyzeRuleResult.OneValue) {
				continue
			}

			if value != "" {
				temp = append(temp, fmt.Sprintf("O(%v)", value))
			}
		}
		return strings.Join(temp, " * ")
	case *ast.ArrayComprehension:
		analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, term.String())
		return fmt.Sprintf("O(%v)", term.String())
	case *ast.ObjectComprehension:
		analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, term.String())
		return fmt.Sprintf("O(%v)", term.String())
	case *ast.SetComprehension:
		analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, term.String())
		return fmt.Sprintf("O(%v)", term.String())
	case ast.Null, ast.Boolean, ast.Number, ast.String, ast.Var:
		return "O(1)"
	case ast.Array, ast.Set, ast.Object:
		return "O(1)"
	case ast.Call:
		if !x.IsGround() || len(isVirtualDoc(strings.Split(term.String(), "(")[0], all)) != 0 {
			analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, term.String())
			return fmt.Sprintf("O(%v)", term.String())
		}
		return "O(1)"
	default:
		analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, term.String())
		return "ERROR"
	}
}

// analyzeExpr analyzes an expression by categorizing the variables in the
// expression into one-value and many-value. The variable's first occurrence
// in the body of the rule determines its category
func analyzeExpr(expr *ast.Expr, seen map[string]bool, result *analyzeResult) {

	if expr.IsEquality() {
		for _, term := range expr.Operands() {
			analyzeTerm(term, seen, result)
		}

		// For expressions that do not contain any variables on the RHS,
		// assign the variable on the LHS to the RHS in the one-value category
		// eg. x := input.request.object.metadata.name
		// eg. x := ["foo", "bar"]
		switch x := expr.Operands()[1].Value.(type) {
		case ast.Ref:
			if _, ok := result.OneValue[x.String()]; !ok {
				result.OneValue[expr.Operands()[0].String()] = x
			}
		default:
			result.OneValue[expr.Operands()[0].String()] = x
		}
	} else if expr.IsCall() {
		operator := expr.Operator().String()

		_, ok := ast.BuiltinMap[operator]

		if ok && operator != ast.WalkBuiltin.Name {
			for _, term := range expr.Operands() {
				analyzeTerm(term, seen, result)
			}
		}
	} else if ast.ContainsRefs(expr) && !ast.ContainsComprehensions(expr) {
		switch terms := expr.Terms.(type) {
		case []*ast.Term:
			for _, term := range terms {
				analyzeTerm(term, seen, result)
			}
		case *ast.Term:
			analyzeTerm(terms, seen, result)
		}

	} else if ast.ContainsComprehensions(expr) {
		ast.WalkClosures(expr, func(x interface{}) bool {
			switch x := x.(type) {
			case *ast.ArrayComprehension:
				for _, expr := range x.Body {
					analyzeExpr(expr, seen, result)
				}
			case *ast.SetComprehension:
				for _, expr := range x.Body {
					analyzeExpr(expr, seen, result)
				}
			case *ast.ObjectComprehension:
				for _, expr := range x.Body {
					analyzeExpr(expr, seen, result)
				}
			}
			return false
		})
	} else {
		vars := expr.Vars(varVisitorParams)
		for v := range vars {
			updateAnalyzeResult(seen, v, "", "Missing", result)
		}
	}
}

func analyzeTerm(term *ast.Term, seen map[string]bool, result *analyzeResult) {
	vars := getTermVars(term, varVisitorParams)

	switch x := term.Value.(type) {
	case ast.Ref:
		varToRefPrefix := generateVarToRefPrefix(x)
		for v := range vars {
			updateAnalyzeResult(seen, v, varToRefPrefix[v], "ManyValue", result)
		}
	case *ast.ArrayComprehension:
		for _, expr := range x.Body {
			analyzeExpr(expr, seen, result)
		}
	case *ast.ObjectComprehension:
		for _, expr := range x.Body {
			analyzeExpr(expr, seen, result)
		}
	case *ast.SetComprehension:
		for _, expr := range x.Body {
			analyzeExpr(expr, seen, result)
		}
	case ast.Null, ast.Boolean, ast.Number, ast.String, ast.Var:
		for v := range vars {
			updateAnalyzeResult(seen, v, "", "OneValue", result)
		}
	case ast.Array, ast.Set, ast.Object:
		for v := range vars {
			updateAnalyzeResult(seen, v, "", "OneValue", result)
		}
	case ast.Call:
		for v := range vars {
			updateAnalyzeResult(seen, v, "", "OneValue", result)
		}
	default:
		result.Error = fmt.Errorf("Unknown term type %T", x)
	}
}

// updateAnalyzeResult updates the map of seen variables and places given variable
// in the right category
func updateAnalyzeResult(seen map[string]bool, item ast.Var, refPrefix string, bucket string, result *analyzeResult) {
	if _, ok := seen[item.String()]; !ok {

		if item.String() == "_" {
			item = rewriteUnderscoreInExpr()
		}

		seen[item.String()] = true

		if bucket == "OneValue" {
			result.OneValue[item.String()] = nil
		} else if bucket == "ManyValue" {
			result.ManyValue[item.String()] = refPrefix
			result.RevManyValue[refPrefix] = item.String()
			result.Iterates = true
		} else if bucket == "Missing" {
			result.Missing = append(result.Missing, item.String())
		}
	}
}

// helper functions

func isVirtualDoc(item string, all []*analyzeResult) []*analyzeResult {

	name := strings.Split(item, ".")
	rule := name[len(name)-1]
	results := []*analyzeResult{}

	for _, result := range all {
		if result.Rule == rule {
			results = append(results, result)
		}
	}
	return results
}

func isPrefixInManyVal(item string, revMany map[string]string) bool {
	for iter := range revMany {
		if item != iter && strings.HasPrefix(item, iter) {
			return true
		}
	}
	return false
}

func isPrefixInOneVal(item string, oneValue map[string]ast.Value) bool {
	for iter := range oneValue {
		if item != iter && strings.HasPrefix(item, iter) {
			return true
		}
	}
	return false
}

func getTermVars(term *ast.Term, params ast.VarVisitorParams) ast.VarSet {
	vis := ast.NewVarVisitor().WithParams(params)
	ast.Walk(vis, term)
	return vis.Vars()
}

func rewriteUnderscoreInExpr() ast.Var {
	result := fmt.Sprintf("_$%v", underscoreCount)
	underscoreCount++
	return ast.Var(result)
}

func isGenerated(v string) bool {
	return strings.HasPrefix(v, "_$")
}

func generateVarToRefPrefix(ref ast.Ref) map[ast.Var]string {
	result := make(map[ast.Var]string)
	var buf []string
	path := ref
	switch v := ref[0].Value.(type) {
	case ast.Var:
		buf = append(buf, string(v))
		path = path[1:]
	}
	for _, p := range path {
		switch p := p.Value.(type) {
		case ast.String:
			str := string(p)
			if varRegexp.MatchString(str) && len(buf) > 0 && !ast.IsKeyword(str) {
				buf = append(buf, "."+str)
			} else {
				buf = append(buf, "["+p.String()+"]")
			}
		case ast.Var:
			result[p] = strings.Join(buf, "")
			buf = append(buf, "["+p.String()+"]")
		default:
			buf = append(buf, "["+p.String()+"]")
		}
	}
	return result
}
