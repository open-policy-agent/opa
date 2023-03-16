package ast

// customJSON is an interface that can be implemented by AST nodes that
// allows the parser to set options for JSON operations on that node.
type customJSON interface {
	setJSONOptions(JSONOptions)
}
