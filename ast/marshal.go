package ast

// customJSON is an interface that can be implemented by AST nodes that
// allows the parser to set a list of fields and if the field is to be
// included in the JSON output.
type customJSON interface {
	exposeJSONFields(map[string]bool)
}
