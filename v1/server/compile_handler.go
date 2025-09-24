// Copyright 2025 The OPA Authors
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/open-policy-agent/opa/internal/compile"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/rego"
	rego_compile "github.com/open-policy-agent/opa/v1/rego/compile"
	"github.com/open-policy-agent/opa/v1/server/failtracer"
	"github.com/open-policy-agent/opa/v1/server/types"
	"github.com/open-policy-agent/opa/v1/server/writer"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/util"
)

const (
	// decisionLogType is injected under custom.type in the DL entry
	decisionLogType = "open-policy-agent/compile"

	invalidUnknownCode = "invalid_unknown"

	// Timer names
	timerPrepPartial                = metrics.CompilePrepPartial
	timerExtractAnnotationsUnknowns = metrics.CompileExtractAnnotationsUnknowns
	timerExtractAnnotationsMask     = metrics.CompileExtractAnnotationsMask

	unknownsCacheSize    = 500
	maskingRuleCacheSize = 500

	// These need to be kept up to date with `CompileApiKnownHeaders()` below
	multiTargetJSON  = "application/vnd.opa.multitarget+json"
	ucastAllJSON     = "application/vnd.opa.ucast.all+json"
	ucastMinimalJSON = "application/vnd.opa.ucast.minimal+json"
	ucastPrismaJSON  = "application/vnd.opa.ucast.prisma+json"
	ucastLINQJSON    = "application/vnd.opa.ucast.linq+json"
	sqlPostgresJSON  = "application/vnd.opa.sql.postgresql+json"
	sqlMySQLJSON     = "application/vnd.opa.sql.mysql+json"
	sqlSQLServerJSON = "application/vnd.opa.sql.sqlserver+json"
	sqliteJSON       = "application/vnd.opa.sql.sqlite+json"

	// back-compat
	applicationJSON = "application/json"
)

func CompileAPIKnownHeaders() []string {
	return []string{
		multiTargetJSON,
		ucastAllJSON,
		ucastMinimalJSON,
		ucastPrismaJSON,
		ucastLINQJSON,
		sqlPostgresJSON,
		sqlMySQLJSON,
		sqlSQLServerJSON,
		sqliteJSON,
	}
}

var allKnownHeaders = append(CompileAPIKnownHeaders(), applicationJSON)

type CompileResult struct {
	Query any            `json:"query"`
	Masks map[string]any `json:"masks,omitempty"`
}

// Incoming JSON request body structure.
type CompileFiltersRequestV1 struct {
	Input    *any      `json:"input"`
	Query    string    `json:"query"`
	Unknowns *[]string `json:"unknowns"`
	Options  struct {
		DisableInlining []string       `json:"disableInlining,omitempty"`
		Mappings        map[string]any `json:"targetSQLTableMappings,omitempty"`
		TargetDialects  []string       `json:"targetDialects,omitempty"`
		MaskRule        string         `json:"maskRule,omitempty"`
	} `json:"options"`
}

type compileFiltersRequest struct {
	Query    ast.Body
	Input    ast.Value
	Unknowns []ast.Ref
	Options  compileFiltersRequestOptions
}

type compileFiltersRequestOptions struct {
	MaskRule  ast.Ref   `json:"maskRule,omitempty"`
	MaskInput ast.Value `json:"maskInput,omitempty"`
}

type CompileResponseV1 struct {
	Result      *any              `json:"result,omitempty"`
	Explanation types.TraceV1     `json:"explanation,omitempty"`
	Metrics     types.MetricsV1   `json:"metrics,omitempty"`
	Hints       []failtracer.Hint `json:"hints,omitempty"`
}

func (s *Server) v1CompileFilters(w http.ResponseWriter, r *http.Request) {
	urlPath := r.PathValue("path")
	if urlPath == "" {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "missing required 'path' parameter"))
		return
	}

	ctx := r.Context()
	explainMode := getExplain(r.URL, types.ExplainOffV1)
	includeInstrumentation := getBoolParam(r.URL, types.ParamInstrumentV1, true)

	m := metrics.New()
	m.Timer(metrics.ServerHandler).Start()
	m.Timer(metrics.RegoQueryParse).Start()
	// decompress the input if sent as zip
	body, err := util.ReadMaybeCompressedBody(r)
	if err != nil {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "could not decompress the body: %v", err))
		return
	}

	comp := s.getCompiler() // used for fuzzy rule name hints, and rego evals further down

	// NOTE(sr): We keep some fields twice: from the unparsed and from the transformed
	// request (`orig` and `request` respectively), because otherwise, we'd need to re-
	// transform the values back for including them in the decision logs.
	orig, request, reqErr := readInputCompileFiltersV1(comp, body, urlPath, s.manager.ParserOptions())
	if reqErr != nil {
		writer.Error(w, http.StatusBadRequest, reqErr)
		return
	}
	m.Timer(metrics.RegoQueryParse).Stop()

	c := storage.NewContext().WithMetrics(m)
	txn, err := s.store.NewTransaction(ctx, storage.TransactionParams{Context: c})
	if err != nil {
		writer.ErrorAuto(w, err)
		return
	}

	defer s.store.Abort(ctx, txn)
	var buf *topdown.BufferTracer
	if explainMode != types.ExplainOffV1 {
		buf = topdown.NewBufferTracer()
	}

	unknowns := request.Unknowns            // always wins over annotations if provided
	maskingRule := request.Options.MaskRule // always wins over annotations if provided

	// check annotations if one of these is missing, needs own compiler
	// NB(sr): we throw this compiler away at the end -- what it produced for us is going to be cached
	var annotationsCompiler *ast.Compiler
	if len(unknowns) == 0 || maskingRule == nil {
		var errs ast.Errors
		annotationsCompiler, errs = prepareAnnotations(ctx, comp, s.store, txn, s.manager.ParserOptions())
		if len(errs) > 0 {
			writer.Error(w, http.StatusBadRequest,
				types.NewErrorV1(types.CodeEvaluation, types.MsgEvaluationError).
					WithASTErrors(errs))
			return
		}
	}

	if len(unknowns) == 0 { // check cache for unknowns
		var errs []*ast.Error
		unknowns, errs = s.compileFiltersUnknowns(m, annotationsCompiler, urlPath, request.Query)
		if errs != nil {
			writer.Error(w, http.StatusBadRequest,
				types.NewErrorV1(types.CodeEvaluation, types.MsgEvaluationError).
					WithASTErrors(errs))
			return
		}
	}

	if maskingRule == nil {
		var errs []*ast.Error
		maskingRule, errs = s.compileFiltersMaskRule(m, annotationsCompiler, urlPath, request.Query)
		if errs != nil {
			writer.Error(w, http.StatusBadRequest,
				types.NewErrorV1(types.CodeEvaluation, types.MsgEvaluationError).
					WithASTErrors(errs))
			return
		}
	}

	var ndbCache builtins.NDBCache
	if s.ndbCacheEnabled {
		ndbCache = builtins.NDBCache{}
	}

	contentType, err := sanitizeHeader(r.Header.Get("Accept"))
	if err != nil {
		writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, "Accept header: %s", err.Error()))
		return
	}

	target, dialect := targetDialect(contentType)

	targetOption := []rego_compile.CompileOption{}
	multi := make([][2]string, len(orig.Options.TargetDialects))

	switch target {
	case "multi":
		for i, targetTuple := range orig.Options.TargetDialects {
			s := strings.Split(targetTuple, "+")
			target, dialect := s[0], s[1]
			multi[i] = [2]string{target, dialect}
			targetOption = append(targetOption, rego_compile.Target(target, dialect))
		}
	default:
		targetOption = append(targetOption, rego_compile.Target(target, dialect))
	}

	unks := make([]*ast.Term, len(unknowns))
	for i := range unknowns {
		unks[i] = ast.NewTerm(unknowns[i])
	}
	m.Timer(timerPrepPartial).Start()
	// NB(sr): just cache preparedCompile by path?
	preparedCompile, err := rego_compile.New(
		append(targetOption,
			rego_compile.ParsedUnknowns(unks...),
			rego_compile.ParsedQuery(request.Query),
			rego_compile.Metrics(m),
			rego_compile.Mappings(orig.Options.Mappings),
			rego_compile.MaskRule(maskingRule),
			rego_compile.Rego(
				rego.Compiler(comp),
				rego.Store(s.store),
				rego.Transaction(txn),
				rego.DisableInlining(orig.Options.DisableInlining),
				rego.QueryTracer(buf),
				rego.Instrument(includeInstrumentation),
				rego.NDBuiltinCache(ndbCache),
				rego.Runtime(s.runtime),
				rego.UnsafeBuiltins(unsafeBuiltinsMap),
				rego.InterQueryBuiltinCache(s.interQueryBuiltinCache),
				rego.InterQueryBuiltinValueCache(s.interQueryBuiltinValueCache),
				rego.PrintHook(s.manager.PrintHook()),
				rego.DistributedTracingOpts(s.distributedTracingOpts),
			),
		)...,
	).Prepare(ctx)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeInvalidParameter, types.MsgCompileModuleError).WithASTErrors(err))
		default:
			writer.ErrorAuto(w, err)
		}
		return

	}
	m.Timer(timerPrepPartial).Stop()

	qt := failtracer.New()

	filters, err := preparedCompile.Compile(ctx,
		rego.EvalTransaction(txn),
		rego.EvalParsedInput(request.Input),
		rego.EvalPrintHook(s.manager.PrintHook()),
		rego.EvalNDBuiltinCache(ndbCache),
		rego.EvalInterQueryBuiltinCache(s.interQueryBuiltinCache),
		rego.EvalInterQueryBuiltinValueCache(s.interQueryBuiltinValueCache),
		rego.EvalQueryTracer(qt),
	)
	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			writer.Error(w, http.StatusBadRequest, types.NewErrorV1(types.CodeEvaluation, types.MsgEvaluationError).WithASTErrors(err))
		default:
			writer.ErrorAuto(w, err)
		}
		return
	}

	result := CompileResponseV1{
		Hints: qt.Hints(unknowns),
	}

	switch target {
	case "multi":
		targets := struct {
			UCAST    *CompileResult `json:"ucast,omitempty"`
			Postgres *CompileResult `json:"postgresql,omitempty"`
			MySQL    *CompileResult `json:"mysql,omitempty"`
			MSSQL    *CompileResult `json:"sqlserver,omitempty"`
			SQLite   *CompileResult `json:"sqlite,omitempty"`
		}{}

		for _, targetTuple := range multi {
			target, dialect := targetTuple[0], targetTuple[1]
			f := filters.For(target, dialect)
			var cr *CompileResult
			if f.Query != nil {
				cr = &CompileResult{Query: f.Query, Masks: f.Masks}
			}
			switch target {
			case "ucast":
				if targets.UCAST != nil {
					continue // there's only one UCAST representation, don't translate that twice
				}
				targets.UCAST = cr
			case "sql":
				switch dialect {
				case "postgresql":
					targets.Postgres = cr
				case "mysql":
					targets.MySQL = cr
				case "sqlserver":
					targets.MSSQL = cr
				case "sqlite":
					targets.SQLite = cr
				}
			}
		}

		t0 := any(targets)
		result.Result = &t0

	default:
		f := filters.For(target, dialect)
		if f.Query != nil {
			s0 := any(&CompileResult{Query: f.Query, Masks: f.Masks})
			result.Result = &s0
		}
	}

	m.Timer(metrics.ServerHandler).Stop()
	fin(w, result, contentType, m, includeMetrics(r), includeInstrumentation, pretty(r))

	unk := make([]string, len(unknowns))
	for i := range unknowns {
		unk[i] = unknowns[i].String()
	}

	br, _ := getRevisions(ctx, s.store, txn)
	ctx, logger := s.getDecisionLogger(ctx, br)
	custom := map[string]any{
		"options":   orig.Options,
		"unknowns":  unk,
		"type":      decisionLogType,
		"mask_rule": maskingRule.String(),
	}

	if err := logger.Log(ctx, txn, urlPath, orig.Query, orig.Input, request.Input, result.Result, ndbCache, nil, m, custom); err != nil {
		writer.ErrorAuto(w, err)
		return
	}
}

func (s *Server) compileFiltersUnknowns(m metrics.Metrics, comp *ast.Compiler, path string, query ast.Body) ([]ast.Ref, []*ast.Error) {
	key := path
	unknowns, ok := s.compileUnknownsCache.Get(key)
	if ok {
		return unknowns, nil
	}
	m.Timer(timerExtractAnnotationsUnknowns).Start()
	if len(query) != 1 {
		return nil, nil
	}
	q, ok := query[0].Terms.(*ast.Term)
	if !ok {
		return nil, nil
	}
	queryRef, ok := q.Value.(ast.Ref)
	if !ok {
		return nil, nil
	}
	unknowns, errs := compile.ExtractUnknownsFromAnnotations(comp, queryRef)
	if errs != nil {
		return nil, errs
	}
	m.Timer(timerExtractAnnotationsUnknowns).Stop()

	s.compileUnknownsCache.Add(key, unknowns)
	return unknowns, nil
}

func (s *Server) compileFiltersMaskRule(m metrics.Metrics, comp *ast.Compiler, path string, query ast.Body) (ast.Ref, []*ast.Error) {
	key := path
	mr, ok := s.compileMaskingRulesCache.Get(key)
	if ok {
		return mr, nil
	}

	m.Timer(timerExtractAnnotationsMask).Start()
	if len(query) != 1 {
		return nil, nil
	}
	q, ok := query[0].Terms.(*ast.Term)
	if !ok {
		return nil, nil
	}
	queryRef, ok := q.Value.(ast.Ref)
	if !ok {
		return nil, nil
	}
	parsedMaskingRule, err := compile.ExtractMaskRuleRefFromAnnotations(comp, queryRef)
	if err != nil {
		return parsedMaskingRule, []*ast.Error{err}
	}
	mr = parsedMaskingRule
	m.Timer(timerExtractAnnotationsMask).Stop()

	s.compileMaskingRulesCache.Add(key, parsedMaskingRule)
	return mr, nil
}

func readInputCompileFiltersV1(comp *ast.Compiler, reqBytes []byte, urlPath string, queryParserOptions ast.ParserOptions) (*CompileFiltersRequestV1, *compileFiltersRequest, *types.ErrorV1) {
	var request CompileFiltersRequestV1

	if len(reqBytes) > 0 {
		if err := util.NewJSONDecoder(bytes.NewBuffer(reqBytes)).Decode(&request); err != nil {
			return nil, nil, types.NewErrorV1(types.CodeInvalidParameter, "error(s) occurred while decoding request: %v", err.Error())
		}
	}

	var query ast.Body
	var err error
	if urlPath != "" {
		v, err := stringPathToDataRef(urlPath)
		if err != nil {
			return nil, nil, types.NewErrorV1(types.CodeInvalidParameter, "invalid path: %v", err)
		}
		query = []*ast.Expr{ast.NewExpr(ast.NewTerm(v))}
	} else { // attempt to parse query
		query, err = ast.ParseBodyWithOpts(request.Query, queryParserOptions)
		if err != nil {
			switch err := err.(type) {
			case ast.Errors:
				return nil, nil, types.NewErrorV1(types.CodeInvalidParameter, types.MsgParseQueryError).WithASTErrors(err)
			default:
				return nil, nil, types.NewErrorV1(types.CodeInvalidParameter, "%v: %v", types.MsgParseQueryError, err)
			}
		}
	}

	var input ast.Value
	if request.Input != nil {
		input, err = ast.InterfaceToValue(*request.Input)
		if err != nil {
			return nil, nil, types.NewErrorV1(types.CodeInvalidParameter, "error(s) occurred while converting input: %v", err)
		}
	}

	var unknowns []ast.Ref
	if request.Unknowns != nil {
		unknowns = make([]ast.Ref, len(*request.Unknowns))
		for i, s := range *request.Unknowns {
			unknowns[i], err = ast.ParseRef(s)
			if err != nil {
				return nil, nil, types.NewErrorV1(types.CodeInvalidParameter, "error(s) occurred while parsing unknowns: %v", err)
			}
		}
	}

	var maskRuleRef ast.Ref
	if request.Options.MaskRule != "" {
		maskPath := request.Options.MaskRule
		if !strings.HasPrefix(request.Options.MaskRule, "data.") {
			// If the mask_rule is not a data ref try adding package prefix from URL path.
			dataFiltersRuleRef, _ := stringPathToDataRef(urlPath)
			maskPath = dataFiltersRuleRef[:len(dataFiltersRuleRef)-1].String() + "." + request.Options.MaskRule
		}
		maskRuleRef, err = ast.ParseRef(maskPath)
		if err != nil {
			hint := compile.FuzzyRuleNameMatchHint(comp, request.Options.MaskRule)
			return nil, nil, types.NewErrorV1(types.CodeInvalidParameter, "error(s) occurred while parsing mask_rule name: %s", hint)
		}
	}

	return &request, &compileFiltersRequest{
		Query:    query,
		Input:    input,
		Unknowns: unknowns,
		Options: compileFiltersRequestOptions{
			MaskRule: maskRuleRef,
		},
	}, nil
}

func fin(w http.ResponseWriter,
	result CompileResponseV1,
	contentType string,
	metrics metrics.Metrics,
	includeMetrics, includeInstrumentation, pretty bool,
) {
	if includeMetrics || includeInstrumentation {
		result.Metrics = metrics.All()
	}

	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}

	w.Header().Add("Content-Type", contentType)
	// If Encode() calls w.Write() for the first time, it'll set the HTTP status
	// to 200 OK.
	if err := enc.Encode(result); err != nil {
		writer.ErrorAuto(w, err)
		return
	}
}

func sanitizeHeader(accept string) (string, error) {
	if accept == "" {
		return "", errors.New("missing required header")
	}

	if strings.Contains(accept, ",") {
		return "", errors.New("multiple headers not supported")
	}

	if !slices.Contains(allKnownHeaders, accept) {
		return "", fmt.Errorf("unsupported header: %s", accept)
	}

	return accept, nil
}

func targetDialect(accept string) (string, string) {
	switch accept {
	case applicationJSON:
		return "", ""
	case multiTargetJSON:
		return "multi", ""
	case ucastAllJSON:
		return "ucast", "all"
	case ucastMinimalJSON:
		return "ucast", "minimal"
	case ucastPrismaJSON:
		return "ucast", "prisma"
	case ucastLINQJSON:
		return "ucast", "linq"
	case sqlPostgresJSON:
		return "sql", "postgresql"
	case sqlMySQLJSON:
		return "sql", "mysql"
	case sqlSQLServerJSON:
		return "sql", "sqlserver"
	case sqliteJSON:
		return "sql", "sqlite"
	}

	panic("unreachable")
}

func cloneCompiler(c *ast.Compiler) *ast.Compiler {
	return ast.NewCompiler().
		WithDefaultRegoVersion(c.DefaultRegoVersion()).
		WithCapabilities(c.Capabilities())
}

func prepareAnnotations(
	ctx context.Context,
	comp *ast.Compiler,
	store storage.Store,
	txn storage.Transaction,
	po ast.ParserOptions,
) (*ast.Compiler, ast.Errors) {
	var errs []*ast.Error
	mods, err := store.ListPolicies(ctx, txn)
	if err != nil {
		return nil, append(errs, ast.NewError(invalidUnknownCode, nil, "failed to list policies for annotation set: %s", err))
	}
	po.ProcessAnnotation = true
	po.RegoVersion = comp.DefaultRegoVersion()
	modules := make(map[string]*ast.Module, len(mods))
	for _, module := range mods {
		vsn, ok := comp.Modules[module]
		if ok { // NB(sr): I think this should be impossible. Let's try not to panic, and fall back to the default if it _does happen_.
			po.RegoVersion = vsn.RegoVersion()
		}
		rego, err := store.GetPolicy(ctx, txn, module)
		if err != nil {
			return nil, append(errs, ast.NewError(invalidUnknownCode, nil, "failed to read module for annotation set: %s", err))
		}
		m, err := ast.ParseModuleWithOpts(module, string(rego), po)
		if err != nil {
			errs = append(errs, ast.NewError(invalidUnknownCode, nil, "failed to parse module for annotation set: %s", err))
			continue
		}
		modules[module] = m
	}
	if errs != nil {
		return nil, errs
	}

	comp0 := cloneCompiler(comp)
	comp0.WithPathConflictsCheck(storage.NonEmpty(ctx, store, txn)).Compile(modules)
	if len(comp0.Errors) > 0 {
		return nil, comp0.Errors
	}
	return comp0, nil
}
