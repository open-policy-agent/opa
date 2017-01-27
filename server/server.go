// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/explain"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/version"
	"github.com/pkg/errors"
)

// Server represents an instance of OPA running in server mode.
type Server struct {
	Handler http.Handler

	addr    string
	persist bool

	// access to the compiler is guarded by mtx
	mtx      sync.RWMutex
	compiler *ast.Compiler

	store *storage.Storage
}

// New returns a new Server.
func New(ctx context.Context, store *storage.Storage, addr string, persist bool) (*Server, error) {

	s := &Server{
		addr:    addr,
		persist: persist,
		store:   store,
	}

	// Initialize HTTP handlers.
	router := mux.NewRouter()
	s.registerHandlerV1(router, "/data/{path:.+}", "PUT", s.v1DataPut)
	s.registerHandlerV1(router, "/data", "PUT", s.v1DataPut)
	s.registerHandlerV1(router, "/data/{path:.+}", "GET", s.v1DataGet)
	s.registerHandlerV1(router, "/data", "GET", s.v1DataGet)
	s.registerHandlerV1(router, "/data/{path:.+}", "PATCH", s.v1DataPatch)
	s.registerHandlerV1(router, "/data", "PATCH", s.v1DataPatch)
	s.registerHandlerV1(router, "/data/{path:.+}", "POST", s.v1DataPost)
	s.registerHandlerV1(router, "/data", "POST", s.v1DataPost)
	s.registerHandlerV1(router, "/policies", "GET", s.v1PoliciesList)
	s.registerHandlerV1(router, "/policies/{id}", "DELETE", s.v1PoliciesDelete)
	s.registerHandlerV1(router, "/policies/{id}", "GET", s.v1PoliciesGet)
	s.registerHandlerV1(router, "/policies/{id}/raw", "GET", s.v1PoliciesRawGet)
	s.registerHandlerV1(router, "/policies/{id}", "PUT", s.v1PoliciesPut)
	s.registerHandlerV1(router, "/query", "GET", s.v1QueryGet)
	router.HandleFunc("/", s.indexGet).Methods("GET")
	s.Handler = router

	// Initialize compiler with policies found in storage.
	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		return nil, err
	}

	defer s.store.Close(ctx, txn)

	modules := s.store.ListPolicies(txn)
	compiler := ast.NewCompiler()

	if compiler.Compile(modules); compiler.Failed() {
		return nil, compiler.Errors
	}

	s.setCompiler(compiler)

	return s, nil
}

// Compiler returns the server's compiler.
//
// The server's compiler contains the compiled versions of all modules added to
// the server as well as data structures for performing query analysis. This is
// intended to allow services to embed the OPA server while still relying on the
// topdown package for query evaluation.
func (s *Server) Compiler() *ast.Compiler {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.compiler
}

// Loop starts the server. This function does not return.
func (s *Server) Loop() error {
	return http.ListenAndServe(s.addr, s.Handler)
}

func (s *Server) execQuery(ctx context.Context, compiler *ast.Compiler, txn storage.Transaction, query ast.Body, explainMode explainModeV1) (queryResponseV1, error) {

	t := topdown.New(ctx, query, s.Compiler(), s.store, txn)

	var buf *topdown.BufferTracer

	if explainMode != explainOffV1 {
		buf = topdown.NewBufferTracer()
		t.Tracer = buf
	}

	qrs := adhocQueryResultSetV1{}

	err := topdown.Eval(t, func(t *topdown.Topdown) error {
		result := map[string]interface{}{}
		for k, v := range t.Vars() {
			if !k.IsWildcard() {
				x, err := topdown.ValueToInterface(v, t)
				if err != nil {
					return err
				}
				result[k.String()] = x
			}
		}
		if len(result) > 0 {
			qrs = append(qrs, result)
		}
		return nil
	})

	if err != nil {
		return queryResponseV1{}, err
	}

	result := queryResponseV1{
		Result: qrs,
	}

	switch explainMode {
	case explainFullV1:
		result.Explanation = newTraceV1(*buf)
	case explainTruthV1:
		answer, err := explain.Truth(compiler, *buf)
		if err != nil {
			return queryResponseV1{}, err
		}
		result.Explanation = newTraceV1(answer)
	}

	return result, nil
}

func (s *Server) indexGet(w http.ResponseWriter, r *http.Request) {

	renderHeader(w)
	renderBanner(w)
	renderVersion(w)

	values := r.URL.Query()
	qStrs := values["q"]
	explainMode := getExplain(r.URL.Query()["explain"])
	ctx := r.Context()

	renderQueryForm(w, qStrs, explainMode)

	if len(qStrs) > 0 {
		qStr := qStrs[len(qStrs)-1]
		t0 := time.Now()

		var results interface{}
		txn, err := s.store.NewTransaction(ctx)

		if err == nil {
			var query ast.Body
			query, err = ast.ParseBody(qStr)
			if err == nil {
				compiler := s.Compiler()
				query, err = compiler.QueryCompiler().Compile(query)
				if err == nil {
					results, err = s.execQuery(ctx, compiler, txn, query, explainMode)
				}
			}
			s.store.Close(ctx, txn)
		}

		dt := time.Since(t0)
		renderQueryResult(w, results, err, dt)
	}

	renderFooter(w)
}

func (s *Server) registerHandlerV1(router *mux.Router, path string, method string, h func(http.ResponseWriter, *http.Request)) {
	router.HandleFunc("/v1"+path, h).Methods(method)
}

func (s *Server) v1DataGet(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	vars := mux.Vars(r)
	path := stringPathToDataRef(vars["path"])
	pretty := getPretty(r.URL.Query()["pretty"])
	explainMode := getExplain(r.URL.Query()["explain"])
	input, nonGround, err := parseInput(r.URL.Query()[ParamInputV1])

	if err != nil {
		handleError(w, 400, err)
		return
	}

	if nonGround && explainMode != explainOffV1 {
		handleError(w, 400, fmt.Errorf("explanations with non-ground input values not supported"))
		return
	}

	// Prepare for query.
	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(ctx, txn)

	compiler := s.Compiler()
	params := topdown.NewQueryParams(ctx, compiler, s.store, txn, input, path)

	var buf *topdown.BufferTracer
	if explainMode != explainOffV1 {
		buf = topdown.NewBufferTracer()
		params.Tracer = buf
	}

	// Execute query.
	qrs, err := topdown.Query(params)

	// Handle results.
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	result := dataResponseV1{}

	if qrs.Undefined() {
		if explainMode == explainFullV1 {
			result.Explanation = newTraceV1(*buf)
			handleResponseJSON(w, 404, result, pretty)
		} else {
			handleResponse(w, 404, nil)
		}
		return
	}

	if nonGround {
		result.Result = newQueryResultSetV1(qrs)
	} else {
		result.Result = qrs[0].Result
	}

	switch explainMode {
	case explainFullV1:
		result.Explanation = newTraceV1(*buf)
	case explainTruthV1:
		answer, err := explain.Truth(compiler, *buf)
		if err != nil {
			handleErrorAuto(w, err)
			return
		}
		result.Explanation = newTraceV1(answer)
	}

	handleResponseJSON(w, 200, result, pretty)
}

func (s *Server) v1DataPatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	ops := []patchV1{}

	if err := util.NewJSONDecoder(r.Body).Decode(&ops); err != nil {
		handleError(w, 400, err)
		return
	}

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(ctx, txn)

	patches, err := s.prepareV1PatchSlice(vars["path"], ops)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	for _, patch := range patches {
		if err := s.store.Write(ctx, txn, patch.op, patch.path, patch.value); err != nil {
			handleErrorAuto(w, err)
			return
		}
	}

	handleResponse(w, 204, nil)
}

func (s *Server) v1DataPost(w http.ResponseWriter, r *http.Request) {

	ctx := r.Context()
	vars := mux.Vars(r)
	path := stringPathToDataRef(vars["path"])
	pretty := getPretty(r.URL.Query()["pretty"])
	explainMode := getExplain(r.URL.Query()["explain"])

	input, err := readInput(r.Body)
	if err != nil {
		handleError(w, 400, err)
		return
	}

	// Prepare for query.
	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(ctx, txn)

	compiler := s.Compiler()
	params := topdown.NewQueryParams(ctx, compiler, s.store, txn, input, path)

	var buf *topdown.BufferTracer
	if explainMode != explainOffV1 {
		buf = topdown.NewBufferTracer()
		params.Tracer = buf
	}

	// Execute query.
	qrs, err := topdown.Query(params)

	// Handle results.
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	result := dataResponseV1{}

	if qrs.Undefined() {
		if explainMode == explainFullV1 {
			result.Explanation = newTraceV1(*buf)
			handleResponseJSON(w, 404, result, pretty)
		} else {
			handleResponse(w, 404, nil)
		}
		return
	}

	result.Result = qrs[0].Result

	switch explainMode {
	case explainFullV1:
		result.Explanation = newTraceV1(*buf)
	case explainTruthV1:
		answer, err := explain.Truth(compiler, *buf)
		if err != nil {
			handleErrorAuto(w, err)
			return
		}
		result.Explanation = newTraceV1(answer)
	}

	handleResponseJSON(w, 200, result, pretty)
}

func (s *Server) v1DataPut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	var value interface{}
	if err := util.NewJSONDecoder(r.Body).Decode(&value); err != nil {
		handleError(w, 400, err)
		return
	}

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(ctx, txn)

	path, ok := storage.ParsePath("/" + strings.Trim(vars["path"], "/"))
	if !ok {
		handleErrorf(w, 400, "bad path format %v", vars["path"])
		return
	}

	_, err = s.store.Read(ctx, txn, path)

	if err != nil {
		if !storage.IsNotFound(err) {
			handleErrorAuto(w, err)
			return
		}
		if err := s.makeDir(ctx, txn, path[:len(path)-1]); err != nil {
			handleErrorAuto(w, err)
			return
		}
	} else if r.Header.Get("If-None-Match") == "*" {
		handleResponse(w, 304, nil)
		return
	}

	if err := s.store.Write(ctx, txn, storage.AddOp, path, value); err != nil {
		handleErrorAuto(w, err)
		return
	}

	handleResponse(w, 204, nil)
}

func (s *Server) v1PoliciesDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(ctx, txn)

	_, _, err = s.store.GetPolicy(txn, id)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	mods := s.store.ListPolicies(txn)
	delete(mods, id)

	c := ast.NewCompiler()

	if c.Compile(mods); c.Failed() {
		handleErrorAST(w, 400, compileModErrMsg, c.Errors)
		return
	}

	if err := s.store.DeletePolicy(txn, id); err != nil {
		handleErrorAuto(w, err)
		return
	}

	s.setCompiler(c)

	handleResponse(w, 204, nil)
}

func (s *Server) v1PoliciesGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(ctx, txn)

	_, _, err = s.store.GetPolicy(txn, id)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	c := s.Compiler()

	response := policyGetResponseV1{
		Result: policyV1{
			ID:     id,
			Module: c.Modules[id],
		},
	}

	handleResponseJSON(w, 200, response, true)
}

func (s *Server) v1PoliciesRawGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(ctx, txn)

	_, bs, err := s.store.GetPolicy(txn, id)

	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	handleResponse(w, 200, bs)
}

func (s *Server) v1PoliciesList(w http.ResponseWriter, r *http.Request) {

	policies := []policyV1{}

	c := s.Compiler()

	for id, mod := range c.Modules {
		policy := policyV1{
			ID:     id,
			Module: mod,
		}
		policies = append(policies, policy)
	}

	response := policyListResponseV1{
		Result: policies,
	}

	handleResponseJSON(w, 200, response, true)
}

func (s *Server) v1PoliciesPut(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleError(w, 500, err)
		return
	}

	parsedMod, err := ast.ParseModule(id, string(buf))

	if err != nil {
		switch err := err.(type) {
		case ast.Errors:
			handleErrorAST(w, 400, compileModErrMsg, err)
		default:
			handleError(w, 400, err)
		}
		return
	}

	if parsedMod == nil {
		handleErrorf(w, 400, "refusing to add empty module")
		return
	}

	txn, err := s.store.NewTransaction(ctx)

	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(ctx, txn)

	mods := s.store.ListPolicies(txn)
	mods[id] = parsedMod

	c := ast.NewCompiler()

	if c.Compile(mods); c.Failed() {
		handleErrorAST(w, 400, compileModErrMsg, c.Errors)
		return
	}

	if err := s.store.InsertPolicy(txn, id, parsedMod, buf, s.persist); err != nil {
		handleErrorAuto(w, err)
		return
	}

	s.setCompiler(c)

	response := policyPutResponseV1{
		Result: policyV1{
			ID:     id,
			Module: c.Modules[id],
		},
	}

	handleResponseJSON(w, 200, response, true)
}

func (s *Server) v1QueryGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	values := r.URL.Query()
	pretty := getPretty(r.URL.Query()["pretty"])
	explainMode := getExplain(r.URL.Query()["explain"])
	qStrs := values["q"]
	if len(qStrs) == 0 {
		handleErrorf(w, 400, "missing query parameter 'q'")
		return
	}

	qStr := qStrs[len(qStrs)-1]

	txn, err := s.store.NewTransaction(ctx)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(ctx, txn)

	compiler := s.Compiler()

	query, err := ast.ParseBody(qStr)
	if err != nil {
		handleCompileError(w, err)
		return
	}

	compiled, err := compiler.QueryCompiler().Compile(query)
	if err != nil {
		handleCompileError(w, err)
		return
	}

	results, err := s.execQuery(ctx, compiler, txn, compiled, explainMode)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	handleResponseJSON(w, 200, results, pretty)
}

func handleCompileError(w http.ResponseWriter, err error) {
	switch err := err.(type) {
	case ast.Errors:
		handleErrorAST(w, 400, compileQueryErrMsg, err)
	default:
		handleError(w, 400, err)
	}
}

func (s *Server) setCompiler(compiler *ast.Compiler) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.compiler = compiler
}

func (s *Server) makeDir(ctx context.Context, txn storage.Transaction, path storage.Path) error {

	node, err := s.store.Read(ctx, txn, path)
	if err == nil {
		if _, ok := node.(map[string]interface{}); ok {
			return nil
		}
		return writeConflictErr{path}
	}

	if !storage.IsNotFound(err) {
		return err
	}

	if err := s.makeDir(ctx, txn, path[:len(path)-1]); err != nil {
		return err
	}

	if err := s.writeConflict(storage.AddOp, path); err != nil {
		return err
	}

	return s.store.Write(ctx, txn, storage.AddOp, path, map[string]interface{}{})
}

func (s *Server) prepareV1PatchSlice(root string, ops []patchV1) (result []patchImpl, err error) {

	root = "/" + strings.Trim(root, "/")

	for _, op := range ops {

		impl := patchImpl{
			value: op.Value,
		}

		// Map patch operation.
		switch op.Op {
		case "add":
			impl.op = storage.AddOp
		case "remove":
			impl.op = storage.RemoveOp
		case "replace":
			impl.op = storage.ReplaceOp
		default:
			return nil, badPatchOperationError(op.Op)
		}

		// Construct patch path.
		path := strings.Trim(op.Path, "/")
		if len(path) > 0 {
			path = root + "/" + path
		} else {
			path = root
		}

		var ok bool
		impl.path, ok = storage.ParsePath(path)
		if !ok {
			return nil, badPatchPathError(op.Path)
		}

		if err := s.writeConflict(impl.op, impl.path); err != nil {
			return nil, err
		}

		result = append(result, impl)
	}

	return result, nil
}

// TODO(tsandall): this ought to be enforced by the storage layer.
func (s *Server) writeConflict(op storage.PatchOp, path storage.Path) error {

	if op == storage.AddOp && len(path) > 0 && path[len(path)-1] == "-" {
		path = path[:len(path)-1]
	}

	ref := path.Ref(ast.DefaultRootDocument)

	if rs := s.Compiler().GetRulesForVirtualDocument(ref); rs != nil {
		return writeConflictErr{path}
	}

	return nil
}

func stringPathToDataRef(s string) (r ast.Ref) {
	result := ast.Ref{ast.DefaultRootDocument}
	result = append(result, stringPathToRef(s)...)
	return result
}

func stringPathToRef(s string) (r ast.Ref) {
	if len(s) == 0 {
		return r
	}
	p := strings.Split(s, "/")
	for _, x := range p {
		if x == "" {
			continue
		}
		i, err := strconv.Atoi(x)
		if err != nil {
			r = append(r, ast.StringTerm(x))
		} else {
			r = append(r, ast.IntNumberTerm(i))
		}
	}
	return r
}

func handleError(w http.ResponseWriter, code int, err error) {
	handleErrorf(w, code, err.Error())
}

func handleErrorAuto(w http.ResponseWriter, err error) {
	var prev error
	for curr := err; curr != prev; {
		if isBadRequest(curr) {
			handleError(w, http.StatusBadRequest, err)
			return
		}
		if ast.IsError(ast.MissingInputErr, curr) {
			handleError(w, http.StatusBadRequest, errInputDoc)
			return
		}
		if storage.IsInvalidPatch(curr) {
			handleError(w, http.StatusBadRequest, err)
			return
		}
		if isWriteConflict(curr) {
			handleError(w, http.StatusNotFound, err)
			return
		}
		if storage.IsNotFound(curr) {
			handleError(w, http.StatusNotFound, err)
			return
		}
		prev = curr
		curr = errors.Cause(prev)
	}
	handleError(w, 500, err)
}

func handleErrorf(w http.ResponseWriter, code int, f string, a ...interface{}) {
	headers := w.Header()
	headers.Add("Content-Type", "application/json")
	e := &apiErrorV1{Code: code, Message: fmt.Sprintf(f, a...)}
	w.WriteHeader(code)
	w.Write(e.Bytes())
}

func handleErrorAST(w http.ResponseWriter, code int, msg string, errs ast.Errors) {
	headers := w.Header()
	headers.Add("Content-Type", "application/json")
	e := &astErrorV1{
		Code:    code,
		Message: msg,
		Errors:  errs,
	}
	w.WriteHeader(code)
	w.Write(e.Bytes())
}

func handleResponse(w http.ResponseWriter, code int, bs []byte) {
	w.WriteHeader(code)
	if code == 204 {
		return
	}
	w.Write(bs)
}

func handleResponseJSON(w http.ResponseWriter, code int, v interface{}, pretty bool) {

	var bs []byte
	var err error

	if pretty {
		bs, err = json.MarshalIndent(v, "", "  ")
	} else {
		bs, err = json.Marshal(v)
	}

	if err != nil {
		handleErrorAuto(w, err)
		return
	}
	headers := w.Header()
	headers.Add("Content-Type", "application/json")
	handleResponse(w, code, bs)
}

func getPretty(p []string) bool {
	for _, x := range p {
		if strings.ToLower(x) == "true" {
			return true
		}
	}
	return false
}

func getExplain(p []string) explainModeV1 {
	for _, x := range p {
		switch x {
		case string(explainFullV1):
			return explainFullV1
		case string(explainTruthV1):
			return explainTruthV1
		}
	}
	return explainOffV1
}

var errInputPathFormat = fmt.Errorf("input parameter format is [[<path>]:]<value> where <path> is either var or ref")
var errInputDoc = fmt.Errorf(`query requires input document (hint: POST /data[/path] {"input": value})`)

// readInput reads the query input from r and returns an input value that can be
// used for query evaluation.
func readInput(r io.ReadCloser) (ast.Value, error) {

	bs, err := ioutil.ReadAll(r)

	if err != nil {
		return nil, err
	}

	var input ast.Value

	if len(bs) > 0 {

		var request dataRequestV1

		if err := util.UnmarshalJSON(bs, &request); err != nil {
			return nil, errors.Wrapf(err, "body contains malformed input document")
		}

		if request.Input == nil {
			return nil, errInputDoc
		}

		var err error
		input, err = ast.InterfaceToValue(*request.Input)
		if err != nil {
			return nil, err
		}
	}

	return input, nil
}

// parseInput parses the query parameters contained in s and returns an input
// value that can used for query evaluation.
func parseInput(s []string) (ast.Value, bool, error) {

	pairs := make([][2]*ast.Term, len(s))
	nonGround := false

	for i := range s {

		var k *ast.Term
		var v *ast.Term
		var err error

		if len(s[i]) == 0 {
			return nil, false, errInputPathFormat
		}

		if s[i][0] == ':' {
			k = ast.NewTerm(ast.InputRootRef)
			v, err = ast.ParseTerm(s[i][1:])
		} else {
			v, err = ast.ParseTerm(s[i])
			if err == nil {
				k = ast.NewTerm(ast.InputRootRef)
			} else {
				vs := strings.SplitN(s[i], ":", 2)
				if len(vs) != 2 {
					return nil, false, errInputPathFormat
				}
				k, err = ast.ParseTerm(ast.InputRootDocument.String() + "." + vs[0])
				if err != nil {
					return nil, false, errInputPathFormat
				}
				v, err = ast.ParseTerm(vs[1])
			}
		}

		if err != nil {
			return nil, false, err
		}

		pairs[i] = [...]*ast.Term{k, v}

		if !nonGround {
			ast.WalkVars(v, func(x ast.Var) bool {
				if x.Equal(ast.DefaultRootDocument.Value) {
					return false
				}
				nonGround = true
				return true
			})
		}
	}

	input, err := topdown.MakeInput(pairs)
	if err != nil {
		return nil, false, err
	}

	return input, nonGround, nil
}

func renderBanner(w http.ResponseWriter) {
	fmt.Fprintln(w, `<pre>
 ________      ________    ________
|\   __  \    |\   __  \  |\   __  \
\ \  \|\  \   \ \  \|\  \ \ \  \|\  \
 \ \  \\\  \   \ \   ____\ \ \   __  \
  \ \  \\\  \   \ \  \___|  \ \  \ \  \
   \ \_______\   \ \__\      \ \__\ \__\
    \|_______|    \|__|       \|__|\|__|
	</pre>`)
	fmt.Fprintln(w, "Open Policy Agent - An open source project to policy-enable your service.<br>")
	fmt.Fprintln(w, "<br>")
}

func renderFooter(w http.ResponseWriter) {
	fmt.Fprintln(w, "</body>")
	fmt.Fprintln(w, "</html>")
}

func renderHeader(w http.ResponseWriter) {
	fmt.Fprintln(w, "<html>")
	fmt.Fprintln(w, "<body>")
}

func renderQueryForm(w http.ResponseWriter, qStrs []string, explain explainModeV1) {

	input := ""

	if len(qStrs) > 0 {
		input = qStrs[len(qStrs)-1]
	}

	explainRadioCheck := []string{"", "", ""}
	switch explain {
	case explainOffV1:
		explainRadioCheck[0] = "checked"
	case explainFullV1:
		explainRadioCheck[1] = "checked"
	case explainTruthV1:
		explainRadioCheck[2] = "checked"
	}

	fmt.Fprintf(w, `
	<form>
  	Query:<br>
	<textarea rows="10" cols="50" name="q">%s</textarea><br>
	<input type="submit" value="Submit"> Explain:
	<input type="radio" name="explain" value="off" %v>Off
	<input type="radio" name="explain" value="full" %v>Full
	<input type="radio" name="explain" value="truth" %v>Truth
	</form>`, input, explainRadioCheck[0], explainRadioCheck[1], explainRadioCheck[2])
}

func renderQueryResult(w io.Writer, results interface{}, err error, d time.Duration) {

	buf, err2 := json.MarshalIndent(results, "", "  ")

	if err != nil {
		fmt.Fprintf(w, "Query error (took %v): <pre>%v</pre>", d, err)
	} else if err2 != nil {
		fmt.Fprintf(w, "JSON marshal error: <pre>%v</pre>", err2)
	} else {
		fmt.Fprintf(w, "Query results (took %v):<br>", d)
		fmt.Fprintf(w, "<pre>%s</pre>", string(buf))
	}
}

func renderVersion(w http.ResponseWriter) {
	fmt.Fprintln(w, "Version: "+version.Version+"<br>")
	fmt.Fprintln(w, "Build Commit: "+version.Vcs+"<br>")
	fmt.Fprintln(w, "Build Timestamp: "+version.Timestamp+"<br>")
	fmt.Fprintln(w, "Build Hostname: "+version.Hostname+"<br>")
	fmt.Fprintln(w, "<br>")
}

type patchImpl struct {
	path  storage.Path
	op    storage.PatchOp
	value interface{}
}

// writeConflictErr represents an error condition raised if the caller attempts
// to modify a virtual document or create a document at a path that conflicts
// with an existing document.
type writeConflictErr struct {
	path storage.Path
}

func (err writeConflictErr) Error() string {
	return fmt.Sprintf("write conflict: %v", err.path)
}

func isWriteConflict(err error) bool {
	_, ok := err.(writeConflictErr)
	return ok
}

type badRequestError string

func badPatchOperationError(op string) error {
	return badRequestError(fmt.Sprintf("bad patch operation: %v", op))
}

func badPatchPathError(path string) error {
	return badRequestError(fmt.Sprintf("bad patch path: %v", path))
}

func missingInputError(path string) error {
	return badRequestError(fmt.Sprintf("query requires input document (hint: POST /data%v {\"input\": ...})", path))
}

func (err badRequestError) Error() string {
	return string(err)
}

// isBadRequest returns true if the error indicates a badly formatted request.
func isBadRequest(err error) bool {
	_, ok := err.(badRequestError)
	return ok
}
