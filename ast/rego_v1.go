package ast

func CheckDuplicateImports(modules []*Module) (errors Errors) {
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

func CheckRootDocumentOverrides(node interface{}) Errors {
	errors := Errors{}

	WalkRules(node, func(rule *Rule) bool {
		name := rule.Head.Name.String()
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

func CheckDeprecatedBuiltins(deprecatedBuiltinsMap map[string]struct{}, node interface{}) Errors {
	errs := make(Errors, 0)
	WalkExprs(node, func(x *Expr) bool {
		if x.IsCall() {
			operator := x.Operator().String()
			if _, ok := deprecatedBuiltinsMap[operator]; ok {
				errs = append(errs, NewError(TypeErr, x.Loc(), "deprecated built-in function calls in expression: %v", operator))
			}
		}
		return false
	})
	return errs
}

func CheckDeprecatedBuiltinsForCurrentVersion(node interface{}) Errors {
	deprecatedBuiltins := make(map[string]struct{})
	capabilities := CapabilitiesForThisVersion()
	for _, bi := range capabilities.Builtins {
		if bi.IsDeprecated() {
			deprecatedBuiltins[bi.Name] = struct{}{}
		}
	}

	return CheckDeprecatedBuiltins(deprecatedBuiltins, node)
}
