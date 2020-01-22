// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package complexity

import (
	"fmt"
	"regexp"
	"strconv"
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
	varRegexp                    = regexp.MustCompile("^[[:alpha:]_][[:alpha:][:digit:]_]*$")
	virtualDocRegexp             = regexp.MustCompile(`{[^}]*}`)
	functionCallRegexp           = regexp.MustCompile(`\#(.*?)\#`)
	functionResultGroupingRegexp = regexp.MustCompile(`]`)
	argRegexp                    = regexp.MustCompile(`\((.*?)\)`)
	indexSeparator               = "$"
	underscoreCount              = 0
	localVarCount                = 0

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

		// mark function arguments as seen
		if len(r.Head.Args) > 0 {
			for _, term := range r.Head.Args {
				seen[term.String()] = true
			}
		}

		for _, expr := range r.Body {
			analyzeExpr(expr, seen, &result)
		}
		results = append(results, &result)

		return false
	})

	resultsPre := runtimeComplexityRule(m, results, finalResult.Missing)
	finalResult.Result = postProcessComplexityResults(resultsPre, finalResult.Missing)
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
func postProcessComplexityResults(preRes map[string][]string, missing map[string][]string) map[string][]string {

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

				replaceWith := postProcessComplexityResultsHelper(rule, preRes, missing)
				result = strings.Replace(result, match, replaceWith, -1)
			}
			postRes[ruleName] = append(postRes[ruleName], result)
		}
	}
	return postProcessFunctionCall(postRes, missing)
}

func postProcessComplexityResultsHelper(ruleName string, preRes map[string][]string, missing map[string][]string) string {

	var ok bool
	var values []string
	if values, ok = preRes[ruleName]; !ok {
		// try the missing results
		if _, ok = missing[ruleName]; ok {
			return fmt.Sprintf("{O(%v)}", ruleName)
		}
		panic(fmt.Sprintf("Rule %v not found in pre time complexity result map or missing map", ruleName))
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

				replaceWith := postProcessComplexityResultsHelper(rule, preRes, missing)
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

// postProcessFunctionCall walks through the complexity results and
// replaces results with function names with their actual complexity results
func postProcessFunctionCall(postRes map[string][]string, missing map[string][]string) map[string][]string {
	finalRes := make(map[string][]string)

	for ruleName, results := range postRes {
		for _, result := range results {
			matches := functionCallRegexp.FindAllString(result, -1)

			if matches == nil || len(matches) == 0 {
				finalRes[ruleName] = append(finalRes[ruleName], result)
				continue
			}

			for _, match := range matches {
				fullName := strings.Trim(match, "#")
				name := strings.Split(fullName, "(")
				rule := name[0]

				replaceWith := postProcessFunctionCallHelper(rule, postRes, missing)
				result = strings.Replace(result, match, substituteFunctionArgs(fullName, replaceWith, true), -1)
			}

			finalRes[ruleName] = append(finalRes[ruleName], generateGroupedResult(result))
		}
	}
	return finalRes
}

func postProcessFunctionCallHelper(ruleName string, postRes map[string][]string, missing map[string][]string) string {

	var ok bool
	var values []string
	if values, ok = postRes[ruleName]; !ok {
		// try the missing results
		if _, ok = missing[ruleName]; ok {
			return fmt.Sprintf("{O(%v)}", ruleName)
		}
		panic(fmt.Sprintf("Rule %v not found in pre time complexity result map or missing map", ruleName))
	}

	result := []string{}
	for _, val := range values {
		matches := functionCallRegexp.FindAllString(val, -1)

		if matches == nil || len(matches) == 0 {
			if val != "O(1)" {
				result = append(result, val)
			}

		} else {
			for _, match := range matches {
				fullName := strings.Trim(match, "#")
				name := strings.Split(fullName, "(")
				rule := name[0]

				replaceWith := postProcessFunctionCallHelper(rule, postRes, missing)
				val = strings.Replace(val, match, substituteFunctionArgs(fullName, replaceWith, false), -1)
				result = append(result, val)
			}
		}
	}

	if len(result) == 0 {
		result = append(result, "O(1)")
	} else if len(result) > 1 {
		return fmt.Sprintf("[%v]", strings.Join(result, " + "))
	}
	return strings.Join(result, "")
}

// substituteFunctionArgs rewrites the runtime of a function in terms of the
// arguments provided to the function by the caller
//
// Example:
// foo {
//    bar(input.request)
// }
//
// bar(request) {
//    request.spec[_] == "hello"
// }
//
// Rule: foo    Complexity: O(bar(input.request))
// Rule: bar    Complexity: O(request.spec$0)  # where $0 indicates the position
//                                             # of the argument. In this case
//                                             # "request" is the first (ie 0th)
//                                             # argument to the function bar
//
// becomes
//
// Rule: foo    Complexity: O(input.request.spec)
func substituteFunctionArgs(original, subWith string, topLevelRule bool) string {

	if subWith == "O(1)" {
		return subWith
	}

	matches := argRegexp.FindAllString(original, -1)
	argsOriginal := strings.Split(strings.Trim(matches[0], "()"), " ")

	matches = argRegexp.FindAllString(subWith, -1)
	inputsSub := strings.Split(strings.Trim(matches[0], "()"), " ")

	if len(inputsSub) > len(argsOriginal) {
		panic(fmt.Sprintf("Error replacing inputs in complexity result %v with arguments in function %v", subWith, original))
	}

	for _, item := range inputsSub {
		items := strings.Split(item, indexSeparator)
		index := items[len(items)-1]
		i, err := strconv.Atoi(index)
		if err != nil {
			continue
		}

		remaining := strings.Join(items[:len(items)-1], "")
		remainingParts := strings.Split(remaining, ".")

		arg := argsOriginal[i]
		if len(remainingParts) > 0 {
			arg = fmt.Sprintf("%v.%v", arg, strings.Join(remainingParts[1:len(remainingParts)], "."))
		}

		newArg := fmt.Sprintf("%v%v%v", arg, indexSeparator, index)
		if topLevelRule {
			newArg = fmt.Sprintf("%v", arg)
		}

		subWith = strings.Replace(subWith, item, newArg, -1)
	}

	return subWith
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

			if strings.HasPrefix(results[i].TimeComplexity, "^") {
				resultComplexity = fmt.Sprintf("+ [%v]", strings.TrimPrefix(results[i].TimeComplexity, "^"))
			} else {
				resultComplexity = fmt.Sprintf("%v", results[i].TimeComplexity)
			}

		} else {
			if strings.HasPrefix(results[i].TimeComplexity, "^") {
				resultComplexity = fmt.Sprintf("+ [%v] %v", strings.TrimPrefix(results[i].TimeComplexity, "^"), resultComplexity)
			} else {
				if results[i].TimeComplexity != "O(1)" {

					if strings.HasPrefix(resultComplexity, "+") {
						resultComplexity = fmt.Sprintf("[ %v  %v ]", results[i].TimeComplexity, resultComplexity)
					} else {
						if ast.ContainsRefs(results[i].Raw) {
							resultComplexity = fmt.Sprintf("[ %v * %v ]", results[i].TimeComplexity, resultComplexity)
						} else {
							resultComplexity = fmt.Sprintf("[ %v + %v ]", results[i].TimeComplexity, resultComplexity)
						}
					}
				}
			}
		}
	}

	if resultComplexity == "" {
		resultComplexity = "O(1)"
	}

	// remove leading "+" sign
	if len(analyzeRuleResult.Raw.Head.Args) != 0 {
		resultComplexity = strings.TrimPrefix(resultComplexity, "+")
	}

	resultComplexity = strings.TrimSpace(resultComplexity)

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
	} else if expr.IsEquality() || expr.IsAssignment() {
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

		analyzeExprResult.Time = strings.Join(temp, " * ")

		analyzeExprResult.TimeComplexity = analyzeExprResult.Time

	} else if expr.IsCall() {
		operator := expr.Operator().String()

		_, ok := ast.BuiltinMap[operator]
		if ok {
			if operator != ast.WalkBuiltin.Name {
				analyzeExprResult.Count = "O(1)"
				analyzeExprResult.Time = "O(1)"

				if operator == ast.Assign.Name {
					analyzeExprResult.Time = runtimeComplexityTerm(expr.Operands()[1], analyzeRuleResult, all)
				}

				analyzeExprResult.TimeComplexity = analyzeExprResult.Time
			} else {
				analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, expr.String())
			}

		} else {
			opSl := strings.Split(operator, ".")
			op := opSl[len(opSl)-1]

			temp := []string{}
			for i := range expr.Operands() {

				// check the one value and many value bucket to see if the
				// function's argument points to a local variable or a rule/function.
				// Add such an expression to the missing list as the runtime of
				// the function cannot be represented definitively in terms
				// of the original dependancy of the argument
				//
				// Example:
				// foo {
				//    some container
				//    input_container[container]
				//    bar(container)
				// }
				//
				// input_container[c] {
				//    c := input.request.object.spec.containers[_]
				// }
				//
				// bar(item) {
				//    item.spec[_] == "hello"
				// }
				//
				// In rule "foo", the runtime of the function "bar" depends
				// on the argument "container" which is a local
				// variable. The value of the variable "container" itself
				// depends on "input.request.object.spec.containers"
				// as described in the rule "input_container". For such scenarios,
				// where is it not definitive to track the dependancy of the
				// variable in this case "container", add the expression to the
				// missing list

				var value string
				var val ast.Value
				var ok bool

				if val, ok = analyzeRuleResult.OneValue[expr.Operand(i).String()]; ok {
					if val != nil {
						if isLocal(val.String()) || len(isVirtualDoc(val.String(), all)) != 0 {
							analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, expr.String())
						} else {
							temp = append(temp, val.String())
						}
					}
				} else if value, ok = analyzeRuleResult.ManyValue[expr.Operand(i).String()]; ok {
					if value != "" {
						if isLocal(value) || len(isVirtualDoc(value, all)) != 0 {
							analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, expr.String())
						} else {
							temp = append(temp, value)
						}
					}
				} else {
					temp = append(temp, expr.Operand(i).String())
				}
			}
			analyzeExprResult.TimeComplexity = fmt.Sprintf("#%v(%v)#", op, strings.Join(temp, " "))
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

			if isPrefixInManyVal(value, analyzeRuleResult) ||
				isPrefixInOneVal(value, analyzeRuleResult.OneValue) {
				continue
			}

			if value != "" && !isLocal(value) {
				tempRes := fmt.Sprintf("O(%v)", value)

				// if the term is part of an expression in the function body,
				// check if the runtime complexity of the term
				// references any of the function's arguments
				//
				// Example:
				// get_foo(a, b) {
				//	 a[_]
				//   b.foo[_]
				// }
				//
				// Complexity: O(a_0) * O(b.foo_1) where "0" and "1" indicate
				// position of the arguments in the function definition
				if len(analyzeRuleResult.Raw.Head.Args) != 0 {
					args := analyzeRuleResult.Raw.Head.Args
					for i := range args {
						if strings.HasPrefix(value, args[i].String()) {
							tempRes = fmt.Sprintf("O(%v%v%v)", value, indexSeparator, i)
							break
						}
					}
				}

				temp = append(temp, tempRes)
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
	case ast.Null, ast.Boolean, ast.Number, ast.String:
		return "O(1)"
	case ast.Var:
		// if the variable is the return value of the function,
		// check if the variable points to a value that
		// references any of the function's arguments
		if term.Equal(analyzeRuleResult.Raw.Head.Value) {
			var value string
			var val ast.Value
			var ok bool

			if val, ok = analyzeRuleResult.OneValue[term.String()]; ok {
				if val != nil {
					tempRes := fmt.Sprintf("O(%v)", val.String())
					if len(analyzeRuleResult.Raw.Head.Args) != 0 {
						args := analyzeRuleResult.Raw.Head.Args
						for i := range args {
							if strings.HasPrefix(val.String(), args[i].String()) {
								tempRes = fmt.Sprintf("^O(%v%v%v)", val.String(), indexSeparator, i)
								break
							}
						}
					}
					return tempRes
				}
			} else if value, ok = analyzeRuleResult.ManyValue[term.String()]; ok {
				return fmt.Sprintf("O(%v)", value)
			} else {
				return fmt.Sprintf("O(%v)", term.String())
			}
		}
		return "O(1)"
	case ast.Array, ast.Set, ast.Object:
		return "O(1)"
	case ast.Call:
		if !x.IsGround() || len(isVirtualDoc(strings.Split(term.String(), "(")[0], all)) != 0 {

			matches := argRegexp.FindAllString(term.String(), -1)

			for _, match := range matches {
				arg := strings.Trim(match, "()")

				// check if the function's argument or its prefix is a locally
				// generated variable. If yes, it implies that the function was
				// provided a result of a previous rule/function evaluation.
				// Since the runtime of such a function cannot be represented
				// definitively in terms of the original dependancy of the argument,
				// add the function to the missing list

				var value string
				var val ast.Value
				var ok, okOneValue, okManyValue bool

				// use the argument's prefix if it exists in the one value or
				// many value bucket. If it doesn't, use the original argument
				resolvedArg := resolvePrefix(arg, analyzeRuleResult)

				if val, ok = analyzeRuleResult.OneValue[resolvedArg]; ok {
					if val != nil {
						_, okOneValue = analyzeRuleResult.OneValue[val.String()]
						_, okManyValue = analyzeRuleResult.ManyValue[val.String()]
						if okOneValue || isPrefixInOneVal(val.String(), analyzeRuleResult.OneValue) ||
							okManyValue || isPrefixInManyVal(val.String(), analyzeRuleResult) ||
							len(isVirtualDoc(val.String(), all)) != 0 {
							analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, term.String())
						}
					}
				} else if value, ok = analyzeRuleResult.ManyValue[resolvedArg]; ok {
					if value != "" {
						_, okOneValue = analyzeRuleResult.OneValue[value]
						_, okManyValue = analyzeRuleResult.ManyValue[value]
						if okOneValue || isPrefixInOneVal(value, analyzeRuleResult.OneValue) ||
							okManyValue || isPrefixInManyVal(value, analyzeRuleResult) ||
							len(isVirtualDoc(value, all)) != 0 {
							analyzeRuleResult.Missing = append(analyzeRuleResult.Missing, term.String())
						}
					}
				}
			}
			return fmt.Sprintf("#%v#", term.String())
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

	if expr.IsEquality() || expr.IsAssignment() {
		for _, term := range expr.Operands() {
			analyzeTerm(term, seen, result)
		}

		// For expressions that contain references with no variables on the RHS,
		// assign the variable on the LHS to the RHS in the one-value category
		// eg. x := input.request.object.metadata.name
		switch x := expr.Operands()[1].Value.(type) {
		case ast.Ref:
			if _, ok := result.OneValue[x.String()]; !ok {
				result.OneValue[expr.Operands()[0].String()] = x
			}
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

// helper functions

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

func isPrefixInManyVal(item string, analyzeRuleResult *analyzeResult) bool {
	for iter := range analyzeRuleResult.RevManyValue {
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

func resolvePrefix(item string, analyzeRuleResult *analyzeResult) string {
	for iter := range analyzeRuleResult.OneValue {
		if item != iter && strings.HasPrefix(item, iter) {
			return iter
		}
	}

	for iter := range analyzeRuleResult.RevManyValue {
		if item != iter && strings.HasPrefix(item, iter) {
			return iter
		}
	}

	return item
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

func generateLocalVar() ast.Var {
	result := fmt.Sprintf("__local%v__", localVarCount)
	localVarCount++
	return ast.Var(result)
}

func isGenerated(v string) bool {
	return strings.HasPrefix(v, "_$")
}

func isLocal(v string) bool {
	return strings.HasPrefix(v, "__local")
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

func generateGroupedResult(input string) string {
	matches := functionResultGroupingRegexp.FindAllStringIndex(input, -1)

	if matches == nil || len(matches) == 0 {
		return input
	}

	shift := 0
	for _, item := range matches {
		idx := item[0]
		input = input[:idx-shift] + input[idx-shift+1:] + "]"
		shift++
	}
	return input
}
