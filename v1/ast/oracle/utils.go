package oracle

import (
	"errors"

	"github.com/open-policy-agent/opa/v1/ast"
)

func halted(c *ast.Compiler) error {
	if c.Failed() && len(c.Errors) == 1 && c.Errors[0].Code == "halt" {
		return nil
	} else if len(c.Errors) > 0 {
		return c.Errors
	}
	// NOTE(tsandall): this indicate an internal error in the compiler and should
	// not be reachable.
	return errors.New("unreachable: did not halt")
}
