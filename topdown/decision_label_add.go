package topdown

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

// add init() in here
// In init() register the function
// http.go Line 201 as example
func init() {
	RegisterBuiltinFunc(ast.DecisionLabelAdd.Name, builtinDecisionLabelAdd)
}

// Operands (http.go Line 128) reference the Decl field from the Builtin Struct and are insanely useful in here
func builtinDecisionLabelAdd(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {

	// TODO: Make sure all of this is aligned to the ast definition of the Built-in
	// both operands need to be pulled in
	obj, err := builtins.ObjectOperand(operands[0].Value, 1)
	if err != nil {
		return handleBuiltinErr(ast.DecisionLabelAdd.Name, bctx.Location, err)
	}

	// From here down is HTTPSend specific (Working on changing that)
	// validate both operands
	// operand [0] should be a string (key)
	// operand [1] should be a string (value) as well
	// make a local function for validation
	req, err := validateHTTPRequestOperand(operands[0], 1)
	if err != nil {
		return handleBuiltinErr(ast.DecisionLabelAdd.Name, bctx.Location, err)
	}

	result, err := getHTTPResponse(bctx, req)
	if err != nil {
		return handleBuiltinErr(ast.DecisionLabelAdd.Name, bctx.Location, err)
	}
	return iter(result)
}

// Need to ensure proper state is accessed, related to ast, no further information right now.
