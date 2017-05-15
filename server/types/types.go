// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package types contains request/response types and codes for the server.
package types

import (
	"encoding/json"
	"fmt"

	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
)

// Error codes returned by OPA's REST API.
const (
	CodeInternal          = "internal_error"
	CodeEvaluation        = "evaluation_error"
	CodeUnauthorized      = "unauthorized"
	CodeInvalidParameter  = "invalid_parameter"
	CodeInvalidOperation  = "invalid_operation"
	CodeResourceNotFound  = "resource_not_found"
	CodeResourceConflict  = "resource_conflict"
	CodeUndefinedDocument = "undefined_document"
)

// ErrorV1 models an error response sent to the client.
type ErrorV1 struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	Errors  []error `json:"errors,omitempty"`
}

// NewErrorV1 returns a new ErrorV1 object.
func NewErrorV1(code, f string, a ...interface{}) *ErrorV1 {
	return &ErrorV1{
		Code:    code,
		Message: fmt.Sprintf(f, a...),
	}
}

// WithError updates e to include a detailed error.
func (e *ErrorV1) WithError(err error) *ErrorV1 {
	e.Errors = append(e.Errors, err)
	return e
}

// WithASTErrors updates e to include detailed AST errors.
func (e *ErrorV1) WithASTErrors(errors []*ast.Error) *ErrorV1 {
	e.Errors = make([]error, len(errors))
	for i := range e.Errors {
		e.Errors[i] = errors[i]
	}
	return e
}

// Bytes marshals e with indentation for readability.
func (e *ErrorV1) Bytes() []byte {
	if bs, err := json.MarshalIndent(e, "", "  "); err == nil {
		return bs
	}
	return nil
}

// Messages included in error responses.
const (
	MsgCompileModuleError         = "error(s) occurred while compiling module(s)"
	MsgCompileQueryError          = "error(s) occurred while compiling query"
	MsgEvaluationError            = "error(s) occurred while evaluating query"
	MsgInputDocError              = "input document is missing or conflicts with query"
	MsgUnauthorizedUndefinedError = "authorization policy missing or undefined"
	MsgUnauthorizedError          = "request rejected by administrative policy"
	MsgUndefinedError             = "document missing or undefined"
)

// PatchV1 models a single patch operation against a document.
type PatchV1 struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// PolicyListResponseV1 models the response mesasge for the Policy API list operation.
type PolicyListResponseV1 struct {
	Result []PolicyV1 `json:"result"`
}

// PolicyGetResponseV1 models the response message for the Policy API get operation.
type PolicyGetResponseV1 struct {
	Result PolicyV1 `json:"result"`
}

// PolicyPutResponseV1 models the response message for the Policy API put operation.
type PolicyPutResponseV1 struct {
	Result PolicyV1 `json:"result"`
}

// PolicyV1 models a policy module in OPA.
type PolicyV1 struct {
	ID     string      `json:"id"`
	Module *ast.Module `json:"module"`
}

// Equal returns true if p is equal to other.
func (p PolicyV1) Equal(other PolicyV1) bool {
	return p.ID == other.ID && p.Module.Equal(other.Module)
}

// DataRequestV1 models the request message for Data API POST operations.
type DataRequestV1 struct {
	Input *interface{} `json:"input"`
}

// DataResponseV1 models the response message for Data API read operations.
type DataResponseV1 struct {
	Explanation TraceV1      `json:"explanation,omitempty"`
	Metrics     MetricsV1    `json:"metrics,omitempty"`
	Result      *interface{} `json:"result,omitempty"`
}

// MetricsV1 models a collection of performance metrics.
type MetricsV1 map[string]interface{}

// QueryResponseV1 models the response message for Query API operations.
type QueryResponseV1 struct {
	Explanation TraceV1               `json:"explanation,omitempty"`
	Metrics     MetricsV1             `json:"metrics,omitempty"`
	Result      AdhocQueryResultSetV1 `json:"result"`
}

// AdhocQueryResultSetV1 models the result of a Query API query.
type AdhocQueryResultSetV1 []map[string]interface{}

// QueryResultSetV1 models the result of a Data API query when the query would
// return multiple values for the document.
type QueryResultSetV1 []*QueryResultV1

// NewQueryResultSetV1 returns a new QueryResultSetV1 object.
func NewQueryResultSetV1(qrs topdown.QueryResultSet) *QueryResultSetV1 {
	result := make(QueryResultSetV1, len(qrs))
	for i := range qrs {
		result[i] = &QueryResultV1{qrs[i].Result, qrs[i].Bindings}
	}
	return &result
}

// QueryResultV1 models a single result of a Data API query that would return
// multiple values for the document. The bindings can be used to differentiate
// between results.
type QueryResultV1 struct {
	result   interface{}
	bindings map[string]interface{}
}

// MarshalJSON serializes the QueryResultV1 object as an array.
func (qr *QueryResultV1) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{qr.result, qr.bindings})
}

// ExplainModeV1 defines supported values for the "explain" query parameter.
type ExplainModeV1 string

// Explanation mode enumeration.
const (
	ExplainOffV1   ExplainModeV1 = "off"
	ExplainFullV1  ExplainModeV1 = "full"
	ExplainTruthV1 ExplainModeV1 = "truth"
)

// TraceV1 models the trace result returned for queries that include the
// "explain" parameter. The trace is modelled as series of trace events that
// identify the expression, local term bindings, query hierarchy, etc.
type TraceV1 []TraceEventV1

// NewTraceV1 returns a new TraceV1 object.
func NewTraceV1(trace []*topdown.Event) (result TraceV1) {
	result = make(TraceV1, len(trace))
	for i := range trace {
		result[i] = TraceEventV1{
			Op:       strings.ToLower(string(trace[i].Op)),
			QueryID:  trace[i].QueryID,
			ParentID: trace[i].ParentID,
			Type:     ast.TypeName(trace[i].Node),
			Node:     trace[i].Node,
			Locals:   NewBindingsV1(trace[i].Locals),
		}
	}
	return result
}

// TraceEventV1 represents a step in the query evaluation process.
type TraceEventV1 struct {
	Op       string      `json:"op"`
	QueryID  uint64      `json:"query_id"`
	ParentID uint64      `json:"parent_id"`
	Type     string      `json:"type"`
	Node     interface{} `json:"node"`
	Locals   BindingsV1  `json:"locals"`
}

// UnmarshalJSON deserializes a TraceEventV1 object. The Node field is
// deserialized based on the type hint from the type property in the JSON
// object.
func (te *TraceEventV1) UnmarshalJSON(bs []byte) error {

	keys := map[string]json.RawMessage{}

	if err := util.UnmarshalJSON(bs, &keys); err != nil {
		return err
	}

	if err := util.UnmarshalJSON(keys["type"], &te.Type); err != nil {
		return err
	}

	if err := util.UnmarshalJSON(keys["op"], &te.Op); err != nil {
		return err
	}

	if err := util.UnmarshalJSON(keys["query_id"], &te.QueryID); err != nil {
		return err
	}

	if err := util.UnmarshalJSON(keys["parent_id"], &te.ParentID); err != nil {
		return err
	}

	switch te.Type {
	case ast.BodyTypeName:
		var body ast.Body
		if err := util.UnmarshalJSON(keys["node"], &body); err != nil {
			return err
		}
		te.Node = body
	case ast.ExprTypeName:
		var expr ast.Expr
		if err := util.UnmarshalJSON(keys["node"], &expr); err != nil {
			return err
		}
		te.Node = &expr
	case ast.RuleTypeName:
		var rule ast.Rule
		if err := util.UnmarshalJSON(keys["node"], &rule); err != nil {
			return err
		}
		te.Node = &rule
	}

	if err := util.UnmarshalJSON(keys["locals"], &te.Locals); err != nil {
		return err
	}

	return nil
}

// BindingsV1 represents a set of term bindings.
type BindingsV1 []*BindingV1

// BindingV1 represents a single term binding.
type BindingV1 struct {
	Key   *ast.Term `json:"key"`
	Value *ast.Term `json:"value"`
}

// NewBindingsV1 returns a new BindingsV1 object.
func NewBindingsV1(locals *ast.ValueMap) (result []*BindingV1) {
	result = make([]*BindingV1, 0, locals.Len())
	locals.Iter(func(key, value ast.Value) bool {
		result = append(result, &BindingV1{
			Key:   &ast.Term{Value: key},
			Value: &ast.Term{Value: value},
		})
		return false
	})
	return result
}

const (
	// ParamInputV1 defines the name of the HTTP URL parameter that specifies
	// values for the "input" document.
	ParamInputV1 = "input"

	// ParamSourceV1 defines the name of the HTTP URL parameter that indicates
	// the client wants to receive the raw (uncompiled) version of the module.
	ParamSourceV1 = "source"

	// ParamPrettyV1 defines the name of the HTTP URL parameter that indicates
	// the client wants to receive a pretty-printed version of the response.
	ParamPrettyV1 = "pretty"

	// ParamMetricsV1 defines the name of the HTTP URL parameter that indicates
	// the client wants to receive performance metrics in addition to the
	// result.
	ParamMetricsV1 = "metrics"
)

// WriteConflictErr represents an error condition raised if the caller attempts
// to modify a virtual document or create a document at a path that conflicts
// with an existing document.
type WriteConflictErr struct {
	Path storage.Path
}

func (err WriteConflictErr) Error() string {
	return fmt.Sprintf("write conflict: %v", err.Path)
}

// IsWriteConflict returns true if err is a WriteConflictErr.
func IsWriteConflict(err error) bool {
	_, ok := err.(WriteConflictErr)
	return ok
}

// BadRequestErr represents an error condition raised if the caller passes
// invalid parameters.
type BadRequestErr string

// BadPatchOperationErr returns BadRequestErr indicating the patch operation was
// invalid.
func BadPatchOperationErr(op string) error {
	return BadRequestErr(fmt.Sprintf("bad patch operation: %v", op))
}

// BadPatchPathErr returns BadRequestErr indicating the patch path was invalid.
func BadPatchPathErr(path string) error {
	return BadRequestErr(fmt.Sprintf("bad patch path: %v", path))
}

func (err BadRequestErr) Error() string {
	return string(err)
}

// IsBadRequest returns true if err is a BadRequestErr.
func IsBadRequest(err error) bool {
	_, ok := err.(BadRequestErr)
	return ok
}
