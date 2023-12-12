package ast

func checkDuplicateImports(modules []*Module) (errors Errors) {
	for _, module := range modules {
		processedImports := map[Var]*Import{}

		for _, imp := range module.Imports {
			name := imp.Name()

			if processed, conflict := processedImports[name]; conflict {
				errors = append(errors, NewError(CompileErr, imp.Location, "import must not shadow %v", processed))
			} else {
				processedImports[name] = imp
			}
		}
	}
	return
}

func checkRootDocumentOverrides(node interface{}) Errors {
	errors := Errors{}

	WalkRules(node, func(rule *Rule) bool {
		var name string
		if len(rule.Head.Reference) > 0 {
			name = rule.Head.Reference[0].Value.(Var).String()
		} else {
			name = rule.Head.Name.String()
		}
		if RootDocumentRefs.Contains(RefTerm(VarTerm(name))) {
			errors = append(errors, NewError(CompileErr, rule.Location, "rules must not shadow %v (use a different rule name)", name))
		}

		for _, arg := range rule.Head.Args {
			if _, ok := arg.Value.(Ref); ok {
				if RootDocumentRefs.Contains(arg) {
					errors = append(errors, NewError(CompileErr, arg.Location, "args must not shadow %v (use a different variable name)", arg))
				}
			}
		}

		return true
	})

	WalkExprs(node, func(expr *Expr) bool {
		if expr.IsAssignment() {
			name := expr.Operand(0).String()
			if RootDocumentRefs.Contains(RefTerm(VarTerm(name))) {
				errors = append(errors, NewError(CompileErr, expr.Location, "variables must not shadow %v (use a different variable name)", name))
			}
		}
		return false
	})

	return errors
}

func walkCalls(node interface{}, f func(interface{}) bool) {
	vis := &GenericVisitor{func(x interface{}) bool {
		switch x := x.(type) {
		case Call:
			return f(x)
		case *Expr:
			if x.IsCall() {
				return f(x)
			}
		case *Head:
			// GenericVisitor doesn't walk the rule head ref
			walkCalls(x.Reference, f)
		}
		return false
	}}
	vis.Walk(node)
}

func checkDeprecatedBuiltins(deprecatedBuiltinsMap map[string]struct{}, node interface{}) Errors {
	errs := make(Errors, 0)

	walkCalls(node, func(x interface{}) bool {
		var operator string
		var loc *Location

		switch x := x.(type) {
		case *Expr:
			operator = x.Operator().String()
			loc = x.Loc()
		case Call:
			terms := []*Term(x)
			if len(terms) > 0 {
				operator = terms[0].Value.String()
				loc = terms[0].Loc()
			}
		}

		if operator != "" {
			if _, ok := deprecatedBuiltinsMap[operator]; ok {
				errs = append(errs, NewError(TypeErr, loc, "deprecated built-in function calls in expression: %v", operator))
			}
		}

		return false
	})

	return errs
}

func checkDeprecatedBuiltinsForCurrentVersion(node interface{}) Errors {
	deprecatedBuiltins := make(map[string]struct{})
	capabilities := CapabilitiesForThisVersion()
	for _, bi := range capabilities.Builtins {
		if bi.IsDeprecated() {
			deprecatedBuiltins[bi.Name] = struct{}{}
		}
	}

	return checkDeprecatedBuiltins(deprecatedBuiltins, node)
}

// CheckRegoV1 checks the given module for errors that are specific to Rego v1
func CheckRegoV1(module *Module) Errors {
	var errors Errors
	errors = append(errors, checkDuplicateImports([]*Module{module})...)
	errors = append(errors, checkRootDocumentOverrides(module)...)
	errors = append(errors, checkDeprecatedBuiltinsForCurrentVersion(module)...)
	return errors
}
