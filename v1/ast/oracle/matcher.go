// Copyright 2025 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.
package oracle

import (
	"github.com/open-policy-agent/opa/v1/ast"
)

// matchResult holds the best match found during searching
type matchResult struct {
	loc   *ast.Location // location of the match
	isVar bool          // true if the match is a variable (not a ref)
}

// createMatcher returns a function that searches for definitions of the target.
// The target can be either a variable or a reference.
func createMatcher(t *target) func(ast.Node, *ast.Compiler, *ast.Module, *matchResult) {
	return func(node ast.Node, compiler *ast.Compiler, parsed *ast.Module, result *matchResult) {
		if t.isRef {
			if targetRef, ok := t.term.Value.(ast.Ref); ok {
				matchRef(targetRef, node, compiler, parsed, result)
			}
		} else if t.isVar {
			if targetVar, ok := t.term.Value.(ast.Var); ok {
				matchVar(targetVar, t.term, node, compiler, parsed, result)
			}
		}
	}
}

// matchRef searches for reference definitions in a node.
// Ref usage sites are never definition sites (definitions come from rules/imports).
func matchRef(targetRef ast.Ref, node ast.Node, compiler *ast.Compiler, parsed *ast.Module, result *matchResult) {
	term, ok := node.(*ast.Term)
	if !ok {
		return
	}

	nodeRef, ok := term.Value.(ast.Ref)
	if !ok {
		return
	}

	// no self defn
	if !nodeRef.Equal(targetRef) {
		return
	}

	// check compiler for rule definitions
	if compiler != nil {
		if loc := findRulesDefinition(compiler, nodeRef); loc != nil {
			result.addRef(loc)
		}
	}

	// check imports for prefix matches
	if parsed != nil {
		prefix := nodeRef.ConstantPrefix()
		for _, imp := range parsed.Imports {
			if path, ok := imp.Path.Value.(ast.Ref); ok {
				if prefix.HasPrefix(path) {
					result.addRef(imp.Path.Location)
				}
			}
		}
	}
}

// matchVar searches for variable definitions in a node.
// Variables can be declared in-place (e.g., function args, iteration vars),
// so targetLocation is used to skip self-definition at the declaration site.
func matchVar(v ast.Var, targetTerm *ast.Term, node ast.Node, compiler *ast.Compiler, parsed *ast.Module, result *matchResult) {
	targetLocation := targetTerm.Location

	switch n := node.(type) {
	case *ast.Expr:
		// Check for iteration variables in Every/SomeDecl
		if every, ok := n.Terms.(*ast.Every); ok {
			// For domains that are refs, look them up in the compiler
			if domain, ok := every.Domain.Value.(ast.Ref); ok && compiler != nil {
				if loc := findRulesDefinition(compiler, domain); loc != nil {
					result.addRef(loc)
				}
			}
			// Check iteration variables
			result.addVarIfNotSelf(every.Key, v, targetLocation)
			result.addVarIfNotSelf(every.Value, v, targetLocation)
		} else if someDecl, ok := n.Terms.(*ast.SomeDecl); ok {
			if len(someDecl.Symbols) > 0 {
				if call, ok := someDecl.Symbols[0].Value.(ast.Call); ok && len(call) >= 2 {
					// For domains that are refs, look them up in the compiler
					domain := call[len(call)-1]
					if domainRef, ok := domain.Value.(ast.Ref); ok && compiler != nil {
						if loc := findRulesDefinition(compiler, domainRef); loc != nil {
							result.addRef(loc)
						}
					}
					// Check iteration variables
					iterVars := call[1 : len(call)-1]
					for _, iterVar := range iterVars {
						result.addVarIfNotSelf(iterVar, v, targetLocation)
					}
				}
			}
		}
	case *ast.Rule: // check function arguments
		for _, arg := range n.Head.Args {
			result.addVarIfNotSelf(arg, v, targetLocation)
		}
	case ast.Body: // check body assignments
		var firstOccurrence *ast.Term
		var assignment *ast.Term

		ast.WalkNodes(n, func(x ast.Node) bool {
			if assignment != nil {
				return true
			}

			switch node := x.(type) {
			case *ast.Expr:
				if !node.IsAssignment() {
					break
				}
				if term := node.Operand(0); term != nil {
					if termMatchesVar(term, v) {
						assignment = term
						return true
					}
				}
			case *ast.Term:
				// implicit definitions look like occurrences rather than assignments
				if firstOccurrence == nil && termMatchesVar(node, v) {
					firstOccurrence = node
				}
			}
			return assignment != nil
		})

		// prefer explicit assignment over implicit definition
		if assignment != nil {
			result.addVarIfNotSelf(assignment, v, targetLocation)
		} else if firstOccurrence != nil {
			result.addVarIfNotSelf(firstOccurrence, v, targetLocation)
		}
	}
}

// addVarIfNotSelf updates the match if the term contains the target variable
// and is not at the target location (avoiding self-definition).
// Only keeps the innermost (first encountered) variable.
func (r *matchResult) addVarIfNotSelf(
	term *ast.Term,
	targetVar ast.Var,
	targetLocation *ast.Location,
) {
	// Skip if we already have a match (either var or ref)
	if term == nil || r.loc != nil {
		return
	}

	v, ok := term.Value.(ast.Var)
	if !ok || v.Compare(targetVar) != 0 {
		return
	}

	// Skip self-definition
	if targetLocation != nil && term.Location != nil && term.Location.Equal(targetLocation) {
		return
	}

	r.loc = term.Location
	r.isVar = true
}

// addRef updates the match with a ref (refs always win over vars)
func (r *matchResult) addRef(loc *ast.Location) {
	// Only set if we have a var or no match yet
	if r.isVar || r.loc == nil {
		r.loc = loc
		r.isVar = false
	}
}

// termMatchesVar checks if a term contains a variable matching the given name.
func termMatchesVar(t *ast.Term, name ast.Var) bool {
	if t == nil {
		return false
	}

	v, ok := t.Value.(ast.Var)

	return ok && v.Compare(name) == 0
}

// findRulesDefinition looks up rules for a given ref. Rules appear in various
// other scenarios and this shares the rule look up logic.
func findRulesDefinition(compiler *ast.Compiler, ref ast.Ref) *ast.Location {
	if rules := compiler.GetRules(ref); len(rules) > 0 {
		return rules[0].Location
	}

	return nil
}
