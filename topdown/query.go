package topdown

import (
	"github.com/open-policy-agent/opa/v1/ast"
	v1 "github.com/open-policy-agent/opa/v1/topdown"
)

// QueryResultSet represents a collection of results returned by a query.
type QueryResultSet = v1.QueryResultSet

// QueryResult represents a single result returned by a query. The result
// contains bindings for all variables that appear in the query.
type QueryResult = v1.QueryResult

// Query provides a configurable interface for performing query evaluation.
type Query = v1.Query

// Builtin represents a built-in function that queries can call.
type Builtin = v1.Builtin

// NewQuery returns a new Query object that can be run.
func NewQuery(query ast.Body) *Query {
	return v1.NewQuery(query)
}
