package ast

import "github.com/open-policy-agent/opa/ast/marshal"

// customJSON is an interface that can be implemented by AST nodes that
// allows the parser to set options for JSON operations on that node.
type customJSON interface {
	setJSONOptions(marshal.JSONOptions)
}
