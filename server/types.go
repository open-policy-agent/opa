// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"encoding/json"

	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
)

// apiErrorV1 models an error response sent to the client.
type apiErrorV1 struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (err *apiErrorV1) Bytes() []byte {
	if bs, err := json.MarshalIndent(err, "", "  "); err == nil {
		return bs
	}
	return nil
}

// astErrorV1 models the error response sent to the client when a parse or
// compile error occurs.
type astErrorV1 struct {
	Code    int          `json:"code"`
	Message string       `json:"message"`
	Errors  []*ast.Error `json:"errors"`
}

func (err *astErrorV1) Bytes() []byte {
	if bs, err := json.MarshalIndent(err, "", "  "); err == nil {
		return bs
	}
	return nil
}

const compileModErrMsg = "error(s) occurred while compiling module(s)"
const compileQueryErrMsg = "error(s) occurred while compiling query"

// patchV1 models a single patch operation against a document.
type patchV1 struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// policyListResponseV1 models the response mesasge for the Policy API list operation.
type policyListResponseV1 struct {
	Result []policyV1 `json:"result"`
}

// policyGetResponseV1 models the response message for the Policy API get operation.
type policyGetResponseV1 struct {
	Result policyV1 `json:"result"`
}

// policyPutResponseV1 models the response message for the Policy API put operation.
type policyPutResponseV1 struct {
	Result policyV1 `json:"result"`
}

// policyV1 models a policy module in OPA.
type policyV1 struct {
	ID     string      `json:"id"`
	Module *ast.Module `json:"module"`
}

func (p policyV1) Equal(other policyV1) bool {
	return p.ID == other.ID && p.Module.Equal(other.Module)
}

// dataRequestV1 models the request message for Data API POST operations.
type dataRequestV1 struct {
	Input *interface{} `json:"input"`
}

// dataResponseV1 models the response message for Data API read operations.
type dataResponseV1 struct {
	Explanation traceV1      `json:"explanation,omitempty"`
	Result      *interface{} `json:"result,omitempty"`
}

// queryResponseV1 models the response message for Query API operations.
type queryResponseV1 struct {
	Explanation traceV1               `json:"explanation,omitempty"`
	Result      adhocQueryResultSetV1 `json:"result"`
}

// adhocQueryResultSet models the result of a Query API query.
type adhocQueryResultSetV1 []map[string]interface{}

// queryResultSetV1 models the result of a Data API query when the query would
// return multiple values for the document.
type queryResultSetV1 []*queryResultV1

func newQueryResultSetV1(qrs topdown.QueryResultSet) *queryResultSetV1 {
	result := make(queryResultSetV1, len(qrs))
	for i := range qrs {
		result[i] = &queryResultV1{qrs[i].Result, qrs[i].Bindings}
	}
	return &result
}

// queryResultV1 models a single result of a Data API query that would return
// multiple values for the document. The bindings can be used to differentiate
// between results.
type queryResultV1 struct {
	result   interface{}
	bindings map[string]interface{}
}

func (qr *queryResultV1) MarshalJSON() ([]byte, error) {
	return json.Marshal([]interface{}{qr.result, qr.bindings})
}

// explainModeV1 defines supported values for the "explain" query parameter.
type explainModeV1 string

const (
	explainOffV1   explainModeV1 = "off"
	explainFullV1  explainModeV1 = "full"
	explainTruthV1 explainModeV1 = "truth"
)

// traceV1 models the trace result returned for queries that include the
// "explain" parameter. The trace is modelled as series of trace events that
// identify the expression, local term bindings, query hierarchy, etc.
type traceV1 []traceEventV1

func newTraceV1(trace []*topdown.Event) (result traceV1) {
	result = make(traceV1, len(trace))
	for i := range trace {
		result[i] = traceEventV1{
			Op:       strings.ToLower(string(trace[i].Op)),
			QueryID:  trace[i].QueryID,
			ParentID: trace[i].ParentID,
			Type:     ast.TypeName(trace[i].Node),
			Node:     trace[i].Node,
			Locals:   newBindingsV1(trace[i].Locals),
		}
	}
	return result
}

// traceEventV1 represents a step in the query evaluation process.
type traceEventV1 struct {
	Op       string      `json:"op"`
	QueryID  uint64      `json:"query_id"`
	ParentID uint64      `json:"parent_id"`
	Type     string      `json:"type"`
	Node     interface{} `json:"node"`
	Locals   bindingsV1  `json:"locals"`
}

func (te *traceEventV1) UnmarshalJSON(bs []byte) error {

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

// bindingsV1 represents a set of term bindings.
type bindingsV1 []*bindingV1

// bindingV1 represents a single term binding.
type bindingV1 struct {
	Key   *ast.Term `json:"key"`
	Value *ast.Term `json:"value"`
}

func newBindingsV1(locals *ast.ValueMap) (result []*bindingV1) {
	result = make([]*bindingV1, 0, locals.Len())
	locals.Iter(func(key, value ast.Value) bool {
		result = append(result, &bindingV1{
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
)
