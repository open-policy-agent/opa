// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
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
	"github.com/open-policy-agent/opa/version"
	"github.com/pkg/errors"
)

// apiErrorV1 models an error response sent to the client.
type apiErrorV1 struct {
	Code    int
	Message string
}

func (err *apiErrorV1) Bytes() []byte {
	if bs, err := json.MarshalIndent(err, "", "  "); err == nil {
		return bs
	}
	return nil
}

// WriteConflictError represents an error condition raised if the caller
// attempts to modify a virtual document.
type WriteConflictError struct {
	path ast.Ref
}

func (err WriteConflictError) Error() string {
	return fmt.Sprintf("write conflict: %v", err.path)
}

// IsWriteConflict returns true if the error indicates write conflict.
func IsWriteConflict(err error) bool {
	_, ok := err.(WriteConflictError)
	return ok
}

// undefinedV1 models the an undefined query result.
type undefinedV1 struct {
	IsUndefined bool
}

// patchV1 models a single patch operation against a document.
type patchV1 struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

// policyV1 models a policy module in OPA.
type policyV1 struct {
	ID     string
	Module *ast.Module
}

func (p *policyV1) Equal(other *policyV1) bool {
	return p.ID == other.ID && p.Module.Equal(other.Module)
}

// resultSetV1 models the result of an ad-hoc query.
type resultSetV1 []map[string]interface{}

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
func New(store *storage.Storage, addr string, persist bool) *Server {

	s := &Server{
		addr:     addr,
		persist:  persist,
		compiler: ast.NewCompiler(),
		store:    store,
	}

	router := mux.NewRouter()

	s.registerHandlerV1(router, "/data/{path:.+}", "GET", s.v1DataGet)
	s.registerHandlerV1(router, "/data", "GET", s.v1DataGet)
	s.registerHandlerV1(router, "/data/{path:.+}", "PATCH", s.v1DataPatch)
	s.registerHandlerV1(router, "/data", "PATCH", s.v1DataPatch)
	s.registerHandlerV1(router, "/policies", "GET", s.v1PoliciesList)
	s.registerHandlerV1(router, "/policies/{id}", "DELETE", s.v1PoliciesDelete)
	s.registerHandlerV1(router, "/policies/{id}", "GET", s.v1PoliciesGet)
	s.registerHandlerV1(router, "/policies/{id}/raw", "GET", s.v1PoliciesRawGet)
	s.registerHandlerV1(router, "/policies/{id}", "PUT", s.v1PoliciesPut)
	s.registerHandlerV1(router, "/query", "GET", s.v1QueryGet)

	router.HandleFunc("/", s.indexGet).Methods("GET")

	s.Handler = router

	return s
}

// Loop starts the server. This function does not return.
func (s *Server) Loop() error {

	txn, err := s.store.NewTransaction()
	if err != nil {
		return err
	}

	mods := s.store.ListPolicies(txn)
	s.store.Close(txn)

	c := ast.NewCompiler()
	if c.Compile(mods); c.Failed() {
		return c.Errors[0]
	}

	s.setCompiler(c)

	return http.ListenAndServe(s.addr, s.Handler)
}

func (s *Server) compileQuery(compiler *ast.Compiler, qStr string) (ast.Body, error) {

	body, err := ast.ParseBody(qStr)
	if err != nil {
		return nil, errors.Wrapf(err, "parse error")
	}

	return compiler.CompileOne(body)
}

func (s *Server) execQuery(qStr string) (resultSetV1, error) {

	compiler := s.getCompiler()

	query, err := s.compileQuery(compiler, qStr)
	if err != nil {
		return nil, err
	}

	txn, err := s.store.NewTransaction()
	if err != nil {
		return nil, err
	}

	defer s.store.Close(txn)

	ctx := topdown.NewContext(query, compiler, s.store, txn)

	results := resultSetV1{}

	err = topdown.Eval(ctx, func(ctx *topdown.Context) error {
		result := map[string]interface{}{}
		var err error
		ctx.Locals.Iter(func(k, v ast.Value) bool {
			kv, ok := k.(ast.Var)
			if !ok {
				return false
			}
			if kv.IsWildcard() {
				return false
			}
			vv, e := topdown.ValueToInterface(v, ctx)
			if err != nil {
				err = e
				return true
			}
			result[string(kv)] = vv
			return false
		})
		if err != nil {
			return err
		}
		if len(result) > 0 {
			results = append(results, result)
		}
		return nil
	})

	return results, err
}

func (s *Server) indexGet(w http.ResponseWriter, r *http.Request) {

	renderHeader(w)
	renderBanner(w)
	renderVersion(w)

	values := r.URL.Query()
	qStrs := values["q"]

	renderQueryForm(w, qStrs)

	if len(qStrs) > 0 {
		qStr := qStrs[len(qStrs)-1]
		t0 := time.Now()
		results, err := s.execQuery(qStr)
		dt := time.Since(t0)
		renderQueryResult(w, results, err, dt)
	}

	renderFooter(w)
}

func (s *Server) registerHandlerV1(router *mux.Router, path string, method string, h func(http.ResponseWriter, *http.Request)) {
	router.HandleFunc("/v1"+path, h).Methods(method)
}

func (s *Server) v1DataGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := stringPathToInterface(vars["path"])
	globals, err := parseGlobals(r.URL.Query()["global"])
	if err != nil {
		handleError(w, 400, err)
		return
	}

	txn, err := s.store.NewTransaction()
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(txn)

	params := topdown.NewQueryParams(s.getCompiler(), s.store, txn, globals, path)

	result, err := topdown.Query(params)

	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	pretty := getPretty(r.URL.Query()["pretty"])

	if _, ok := result.(topdown.Undefined); ok {
		handleResponseJSON(w, 404, undefinedV1{true}, pretty)
		return
	}

	handleResponseJSON(w, 200, result, pretty)
}

func (s *Server) v1DataPatch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	root := ast.Ref{ast.DefaultRootDocument}
	root = append(root, stringPathToRef(vars["path"])...)

	ops := []patchV1{}
	if err := json.NewDecoder(r.Body).Decode(&ops); err != nil {
		handleError(w, 400, err)
		return
	}

	txn, err := s.store.NewTransaction()
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(txn)

	for i := range ops {

		var op storage.PatchOp

		// TODO this could be refactored for failure handling
		switch ops[i].Op {
		case "add":
			op = storage.AddOp
		case "remove":
			op = storage.RemoveOp
		case "replace":
			op = storage.ReplaceOp
		default:
			handleErrorf(w, 400, "bad patch operation: %v", ops[i].Op)
			return
		}

		path := root
		path = append(path, stringPathToRef(ops[i].Path)...)

		if err := s.writeConflict(op, path); err != nil {
			handleErrorAuto(w, err)
			return
		}

		if err := s.store.Write(txn, op, path, ops[i].Value); err != nil {
			handleErrorAuto(w, err)
			return
		}
	}

	handleResponse(w, 204, nil)
}

func (s *Server) writeConflict(op storage.PatchOp, path ast.Ref) error {

	if op == storage.AddOp && path[len(path)-1].Value.Equal(ast.String("-")) {
		path = path[:len(path)-1]
	}

	if rs := s.getCompiler().GetRulesForVirtualDocument(path); rs != nil {
		return WriteConflictError{path}
	}

	return nil
}

func (s *Server) v1PoliciesDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	txn, err := s.store.NewTransaction()
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(txn)

	_, _, err = s.store.GetPolicy(txn, id)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	mods := s.store.ListPolicies(txn)
	delete(mods, id)

	c := ast.NewCompiler()

	if c.Compile(mods); c.Failed() {
		handleErrorf(w, 400, c.FlattenErrors())
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
	vars := mux.Vars(r)
	id := vars["id"]

	txn, err := s.store.NewTransaction()
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(txn)

	mod, _, err := s.store.GetPolicy(txn, id)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	policy := &policyV1{
		ID:     id,
		Module: mod,
	}

	handleResponseJSON(w, 200, policy, true)
}

func (s *Server) v1PoliciesRawGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	txn, err := s.store.NewTransaction()
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(txn)

	_, bs, err := s.store.GetPolicy(txn, id)

	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	handleResponse(w, 200, bs)
}

func (s *Server) v1PoliciesList(w http.ResponseWriter, r *http.Request) {

	policies := []*policyV1{}

	txn, err := s.store.NewTransaction()
	if err != nil {
		handleErrorAuto(w, err)
		return
	}
	defer s.store.Close(txn)

	for id, mod := range s.store.ListPolicies(txn) {
		policy := &policyV1{
			ID:     id,
			Module: mod,
		}
		policies = append(policies, policy)
	}

	handleResponseJSON(w, 200, policies, true)
}

func (s *Server) v1PoliciesPut(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleError(w, 500, err)
		return
	}

	mod, err := ast.ParseModule(id, string(buf))

	if err != nil {
		handleError(w, 400, err)
		return
	}

	if mod == nil {
		handleErrorf(w, 400, "refusing to add empty module")
		return
	}

	txn, err := s.store.NewTransaction()

	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	defer s.store.Close(txn)

	mods := s.store.ListPolicies(txn)
	mods[id] = mod

	c := ast.NewCompiler()

	if c.Compile(mods); c.Failed() {
		handleErrorf(w, 400, c.FlattenErrors())
		return
	}

	mod = c.Modules[id]

	if err := s.store.InsertPolicy(txn, id, mod, buf, s.persist); err != nil {
		handleErrorAuto(w, err)
		return
	}

	s.setCompiler(c)

	policy := &policyV1{
		ID:     id,
		Module: mod,
	}

	handleResponseJSON(w, 200, policy, true)
}

func (s *Server) v1QueryGet(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	qStrs := values["q"]
	if len(qStrs) == 0 {
		handleErrorf(w, 400, "missing query parameter 'q'")
		return
	}

	qStr := qStrs[len(qStrs)-1]
	results, err := s.execQuery(qStr)

	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	pretty := getPretty(r.URL.Query()["pretty"])

	handleResponseJSON(w, 200, results, pretty)
}

func (s *Server) getCompiler() *ast.Compiler {
	s.mtx.RLock()
	defer s.mtx.RUnlock()
	return s.compiler
}

func (s *Server) setCompiler(compiler *ast.Compiler) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.compiler = compiler
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
			r = append(r, ast.NumberTerm(float64(i)))
		}
	}
	return r
}

func stringPathToInterface(s string) (r []interface{}) {
	if len(s) == 0 {
		return r
	}
	p := strings.Split(s, "/")
	for _, x := range p {
		i, err := strconv.Atoi(x)
		if err != nil {
			r = append(r, x)
		} else {
			r = append(r, float64(i))
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
		if storage.IsNotFound(curr) {
			handleError(w, 404, err)
			return
		}
		if topdown.IsUnboundGlobal(curr) {
			handleError(w, 400, err)
			return
		}
		if IsWriteConflict(curr) {
			handleError(w, 404, err)
			return
		}
		if storage.IsInvalidPatch(curr) {
			handleError(w, 400, err)
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

func globalConflictErr(k ast.Value) error {
	return fmt.Errorf("conflicting global: %v: check global arguments", k)
}

func getPretty(p []string) bool {
	for _, x := range p {
		if strings.ToLower(x) == "true" {
			return true
		}
	}
	return false
}

func parseGlobals(g []string) (*storage.Bindings, error) {
	globals := storage.NewBindings()
	for _, g := range g {
		vs := strings.SplitN(g, ":", 2)
		k, err := ast.ParseTerm(vs[0])
		if err != nil {
			return nil, err
		}
		v, err := ast.ParseTerm(vs[1])
		if err != nil {
			return nil, err
		}
		switch k := k.Value.(type) {
		case ast.Ref:
			obj := makeTree(k[1:], v)
			switch b := globals.Get(k[0].Value).(type) {
			case nil:
				globals.Put(k[0].Value, obj)
			case ast.Object:
				m, ok := b.Merge(obj)
				if !ok {
					return nil, globalConflictErr(k)
				}
				globals.Put(k[0].Value, m)
			default:
				return nil, globalConflictErr(k)
			}
		case ast.Var:
			if globals.Get(k) != nil {
				return nil, globalConflictErr(k)
			}
			globals.Put(k, v.Value)
		default:
			return nil, fmt.Errorf("invalid global: %v: path must be a variable or a reference", k)
		}
	}
	return globals, nil
}

// makeTree returns an object that represents a document where the value v is the
// leaf and elements in k represent intermediate objects.
func makeTree(k ast.Ref, v *ast.Term) ast.Object {
	var obj ast.Object
	for i := len(k) - 1; i >= 1; i-- {
		obj = ast.Object{ast.Item(k[i], v)}
		v = &ast.Term{Value: obj}
		obj = ast.Object{}
	}
	obj = ast.Object{ast.Item(k[0], v)}
	return obj
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

func renderQueryForm(w http.ResponseWriter, qStrs []string) {

	input := ""

	if len(qStrs) > 0 {
		input = qStrs[len(qStrs)-1]
	}

	fmt.Fprintf(w, `
	<form>
  	Query:<br>
	<textarea rows="10" cols="50" name="q">%s</textarea><br>
	<input type="submit" value="Submit">
	</form>`, input)
}

func renderQueryResult(w io.Writer, results resultSetV1, err error, d time.Duration) {

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
