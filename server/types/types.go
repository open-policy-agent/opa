// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package types contains request/response types and codes for the server.
package types

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown"
	v1 "github.com/open-policy-agent/opa/v1/server/types"
)

// Error codes returned by OPA's REST API.
const (
	CodeInternal          = v1.CodeInternal
	CodeEvaluation        = v1.CodeEvaluation
	CodeUnauthorized      = v1.CodeUnauthorized
	CodeInvalidParameter  = v1.CodeInvalidParameter
	CodeInvalidOperation  = v1.CodeInvalidOperation
	CodeResourceNotFound  = v1.CodeResourceNotFound
	CodeResourceConflict  = v1.CodeResourceConflict
	CodeUndefinedDocument = v1.CodeUndefinedDocument
)

// ErrorV1 models an error response sent to the client.
type ErrorV1 = v1.ErrorV1

// NewErrorV1 returns a new ErrorV1 object.
func NewErrorV1(code, f string, a ...interface{}) *ErrorV1 {
	return v1.NewErrorV1(code, f, a...)
}

// Messages included in error responses.
const (
	MsgCompileModuleError         = v1.MsgCompileModuleError
	MsgParseQueryError            = v1.MsgParseQueryError
	MsgCompileQueryError          = v1.MsgCompileQueryError
	MsgEvaluationError            = v1.MsgEvaluationError
	MsgUnauthorizedUndefinedError = v1.MsgUnauthorizedUndefinedError
	MsgUnauthorizedError          = v1.MsgUnauthorizedError
	MsgUndefinedError             = v1.MsgUndefinedError
	MsgMissingError               = v1.MsgMissingError
	MsgFoundUndefinedError        = v1.MsgFoundUndefinedError
	MsgPluginConfigError          = v1.MsgPluginConfigError
	MsgDecodingLimitError         = v1.MsgDecodingLimitError
	MsgDecodingGzipLimitError     = v1.MsgDecodingGzipLimitError
)

// PatchV1 models a single patch operation against a document.
type PatchV1 = v1.PatchV1

// PolicyListResponseV1 models the response message for the Policy API list operation.
type PolicyListResponseV1 = v1.PolicyListResponseV1

// PolicyGetResponseV1 models the response message for the Policy API get operation.
type PolicyGetResponseV1 = v1.PolicyGetResponseV1

// PolicyPutResponseV1 models the response message for the Policy API put operation.
type PolicyPutResponseV1 = v1.PolicyPutResponseV1

// PolicyDeleteResponseV1 models the response message for the Policy API delete operation.
type PolicyDeleteResponseV1 = v1.PolicyDeleteResponseV1

// PolicyV1 models a policy module in OPA.
type PolicyV1 = v1.PolicyV1

// ProvenanceV1 models a collection of build/version information.
type ProvenanceV1 = v1.ProvenanceV1

// ProvenanceBundleV1 models a bundle at some point in time
type ProvenanceBundleV1 = v1.ProvenanceBundleV1

// DataRequestV1 models the request message for Data API POST operations.
type DataRequestV1 = v1.DataRequestV1

// DataResponseV1 models the response message for Data API read operations.
type DataResponseV1 = v1.DataResponseV1

// Warning models DataResponse warnings
type Warning = v1.Warning

// Warning Codes
const CodeAPIUsageWarn = v1.CodeAPIUsageWarn

// Warning Messages
const MsgInputKeyMissing = v1.MsgInputKeyMissing

// NewWarning returns a new Warning object
func NewWarning(code, message string) *Warning {
	return v1.NewWarning(code, message)
}

// MetricsV1 models a collection of performance metrics.
type MetricsV1 = v1.MetricsV1

// QueryResponseV1 models the response message for Query API operations.
type QueryResponseV1 = v1.QueryResponseV1

// AdhocQueryResultSetV1 models the result of a Query API query.
type AdhocQueryResultSetV1 = v1.AdhocQueryResultSetV1

// ExplainModeV1 defines supported values for the "explain" query parameter.
type ExplainModeV1 = v1.ExplainModeV1

// Explanation mode enumeration.
const (
	ExplainOffV1   ExplainModeV1 = v1.ExplainOffV1
	ExplainFullV1  ExplainModeV1 = v1.ExplainFullV1
	ExplainNotesV1 ExplainModeV1 = v1.ExplainNotesV1
	ExplainFailsV1 ExplainModeV1 = v1.ExplainFailsV1
	ExplainDebugV1 ExplainModeV1 = v1.ExplainDebugV1
)

// TraceV1 models the trace result returned for queries that include the
// "explain" parameter.
type TraceV1 = v1.TraceV1

// TraceV1Raw models the trace result returned for queries that include the
// "explain" parameter. The trace is modelled as series of trace events that
// identify the expression, local term bindings, query hierarchy, etc.
type TraceV1Raw = v1.TraceV1Raw

// TraceV1Pretty models the trace result returned for queries that include the "explain"
// parameter. The trace is modelled as a human readable array of strings representing the
// evaluation of the query.
type TraceV1Pretty = v1.TraceV1Pretty

// NewTraceV1 returns a new TraceV1 object.
func NewTraceV1(trace []*topdown.Event, pretty bool) (result TraceV1, err error) {
	return v1.NewTraceV1(trace, pretty)
}

// TraceEventV1 represents a step in the query evaluation process.
type TraceEventV1 = v1.TraceEventV1

// BindingsV1 represents a set of term bindings.
type BindingsV1 = v1.BindingsV1

// BindingV1 represents a single term binding.
type BindingV1 = v1.BindingV1

// NewBindingsV1 returns a new BindingsV1 object.
func NewBindingsV1(locals *ast.ValueMap) (result []*BindingV1) {
	return v1.NewBindingsV1(locals)
}

// CompileRequestV1 models the request message for Compile API operations.
type CompileRequestV1 = v1.CompileRequestV1

// CompileResponseV1 models the response message for Compile API operations.
type CompileResponseV1 = v1.CompileResponseV1

// PartialEvaluationResultV1 represents the output of partial evaluation and is
// included in Compile API responses.
type PartialEvaluationResultV1 = v1.PartialEvaluationResultV1

// QueryRequestV1 models the request message for Query API operations.
type QueryRequestV1 = v1.QueryRequestV1

// ConfigResponseV1 models the response message for Config API operations.
type ConfigResponseV1 = v1.ConfigResponseV1

// StatusResponseV1 models the response message for Status API (pull) operations.
type StatusResponseV1 = v1.StatusResponseV1

// HealthResponseV1 models the response message for Health API operations.
type HealthResponseV1 = v1.HealthResponseV1

const (
	// ParamQueryV1 defines the name of the HTTP URL parameter that specifies
	// values for the request query.
	ParamQueryV1 = v1.ParamQueryV1

	// ParamInputV1 defines the name of the HTTP URL parameter that specifies
	// values for the "input" document.
	ParamInputV1 = v1.ParamInputV1

	// ParamPrettyV1 defines the name of the HTTP URL parameter that indicates
	// the client wants to receive a pretty-printed version of the response.
	ParamPrettyV1 = v1.ParamPrettyV1

	// ParamExplainV1 defines the name of the HTTP URL parameter that indicates the
	// client wants to receive explanations in addition to the result.
	ParamExplainV1 = v1.ParamExplainV1

	// ParamMetricsV1 defines the name of the HTTP URL parameter that indicates
	// the client wants to receive performance metrics in addition to the
	// result.
	ParamMetricsV1 = v1.ParamMetricsV1

	// ParamInstrumentV1 defines the name of the HTTP URL parameter that
	// indicates the client wants to receive instrumentation data for
	// diagnosing performance issues.
	ParamInstrumentV1 = v1.ParamInstrumentV1

	// ParamProvenanceV1 defines the name of the HTTP URL parameter that indicates
	// the client wants build and version information in addition to the result.
	ParamProvenanceV1 = v1.ParamProvenanceV1

	// ParamBundleActivationV1 defines the name of the HTTP URL parameter that
	// indicates the client wants to include bundle activation in the results
	// of the health API.
	// Deprecated: Use ParamBundlesActivationV1 instead.
	ParamBundleActivationV1 = v1.ParamBundleActivationV1

	// ParamBundlesActivationV1 defines the name of the HTTP URL parameter that
	// indicates the client wants to include bundle activation in the results
	// of the health API.
	ParamBundlesActivationV1 = v1.ParamBundlesActivationV1

	// ParamPluginsV1 defines the name of the HTTP URL parameter that
	// indicates the client wants to include bundle status in the results
	// of the health API.
	ParamPluginsV1 = v1.ParamPluginsV1

	// ParamExcludePluginV1 defines the name of the HTTP URL parameter that
	// indicates the client wants to exclude plugin status in the results
	// of the health API for the specified plugin(s)
	ParamExcludePluginV1 = v1.ParamExcludePluginV1

	// ParamStrictBuiltinErrors names the HTTP URL parameter that indicates the client
	// wants built-in function errors to be treated as fatal.
	ParamStrictBuiltinErrors = v1.ParamStrictBuiltinErrors
)

// BadRequestErr represents an error condition raised if the caller passes
// invalid parameters.
type BadRequestErr = v1.BadRequestErr

// BadPatchOperationErr returns BadRequestErr indicating the patch operation was
// invalid.
func BadPatchOperationErr(op string) error {
	return v1.BadPatchOperationErr(op)
}

// BadPatchPathErr returns BadRequestErr indicating the patch path was invalid.
func BadPatchPathErr(path string) error {
	return v1.BadPatchPathErr(path)
}

// IsBadRequest returns true if err is a BadRequestErr.
func IsBadRequest(err error) bool {
	return v1.IsBadRequest(err)
}
