package ast

import "fmt"

// CheckRegoV2 checks the given module or rule for errors that are specific to Rego v2,
// including those reported by CheckRegoV1.
// Passing something other than an *ast.Rule or *ast.Module is considered a programming error, and will cause a panic.
func CheckRegoV2(x any) Errors {
	return CheckRegoV2WithOptions(x, NewRegoCheckOptions())
}

// CheckRegoV2WithOptions is CheckRegoV2 with the CheckRegoV1 checks selected by opts.
// Keywords introduced by Rego v2 are rejected as rule names if opts.NoKeywordsAsRuleNames is set,
// and as variable names if opts.NoKeywordsAsVarNames is set.
func CheckRegoV2WithOptions(x any, opts RegoCheckOptions) Errors {
	errs := CheckRegoV1WithOptions(x, opts)

	switch x := x.(type) {
	case *Module:
		for _, rule := range x.Rules {
			errs = append(errs, checkRegoV2Rule(rule, opts)...)
		}
	case *Rule:
		errs = append(errs, checkRegoV2Rule(x, opts)...)
	default:
		panic(fmt.Sprintf("cannot check rego-v2 compatibility on type %T", x))
	}

	return errs
}

func checkRegoV2Rule(rule *Rule, opts RegoCheckOptions) Errors {
	var errs Errors

	if opts.NoKeywordsAsRuleNames && len(rule.Head.Reference) < 2 && isRegoV2OnlyKeyword(rule.Head.Name.String()) {
		errs = append(errs, NewError(ParseErr, rule.Location, "%s keyword cannot be used for rule name", rule.Head.Name.String()))
	}

	if opts.NoKeywordsAsVarNames {
		errs = append(errs, checkVarNamesShadowsKeyword(rule)...)
	}

	return errs
}

func checkVarNamesShadowsKeyword(rule *Rule) Errors {
	var errs Errors

	seen := map[*Term]struct{}{}
	var vis *GenericVisitor
	vis = NewGenericVisitor(func(x any) bool {
		switch x := x.(type) {
		case *Expr:
			// Call operators are refs to built-ins, and the `and`/`or` built-ins
			// are written infix (`&`, `|`).
			if x.IsCall() {
				for _, op := range x.Operands() {
					vis.Walk(op)
				}
				for _, w := range x.With {
					vis.Walk(w)
				}
				return true
			}
		case Call:
			for _, op := range x[1:] {
				vis.Walk(op)
			}
			return true
		case *Term:
			if v, ok := x.Value.(Var); ok && isRegoV2OnlyKeyword(string(v)) {
				// A ref head's last term may also be its key.
				if _, ok := seen[x]; !ok {
					seen[x] = struct{}{}
					errs = append(errs, NewError(ParseErr, x.Location, "%s keyword cannot be used for variable name", v))
				}
			}
		}
		return false
	})

	for r := rule; r != nil; r = r.Else {
		if len(r.Head.Reference) > 1 {
			vis.Walk(r.Head.Reference[1:])
		}
		vis.Walk(r.Head.Args)
		if r.Head.Key != nil {
			vis.Walk(r.Head.Key)
		}
		if r.Head.Value != nil {
			vis.Walk(r.Head.Value)
		}
		vis.Walk(r.Body)
	}

	return errs
}

func isRegoV2OnlyKeyword(s string) bool {
	return IsKeywordInRegoVersion(s, RegoV2) && !IsKeywordInRegoVersion(s, RegoV1)
}
