// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

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

// Server contains runtime state specific to the server-mode persona, e.g.,
// HTTP router.
//
// Notes:
//
// - In the future, the HTTP routing could be factored into a separate module
// relying on the server (which would remain RPC-based). For now, it's simpler
// to keep the HTTP routing and backend implementation in one place.
type Server struct {
	Addr    string
	Persist bool
	Runtime *Runtime
	Router  *mux.Router

	mtx sync.RWMutex
}

// NewServer returns a new Server.
func NewServer(rt *Runtime, addr string, persist bool) *Server {

	s := &Server{
		Addr:    addr,
		Persist: persist,
		Runtime: rt,
		Router:  mux.NewRouter(),
	}

	s.registerHandlerV1("/data/{path:.+}", "GET", s.v1DataGet)
	s.registerHandlerV1("/data/{path:.+}", "PATCH", s.v1DataPatch)
	s.registerHandlerV1("/policies", "GET", s.v1PoliciesList)
	s.registerHandlerV1("/policies/{id}", "DELETE", s.v1PoliciesDelete)
	s.registerHandlerV1("/policies/{id}", "GET", s.v1PoliciesGet)
	s.registerHandlerV1("/policies/{id}/raw", "GET", s.v1PoliciesRawGet)
	s.registerHandlerV1("/policies/{id}", "PUT", s.v1PoliciesPut)
	s.registerHandlerV1("/query", "GET", s.v1QueryGet)

	s.Router.HandleFunc("/", s.indexGet).Methods("GET")

	return s
}

// Loop starts the server. This function does not return.
func (s *Server) Loop() {
	wrapped := NewLoggingHandler(s.Router)
	http.ListenAndServe(s.Addr, wrapped)
}

func (s *Server) execQuery(qStr string) (resultSetV1, error) {

	query, err := ast.ParseBody(qStr)

	if err != nil {
		return nil, err
	}

	path := ast.Ref{ast.DefaultRootDocument}

	rule := &ast.Rule{
		Body: query,
	}

	mod := &ast.Module{
		Package: &ast.Package{
			Path: path,
		},
		Rules: []*ast.Rule{rule},
	}

	c := ast.NewCompiler()

	if c.Compile(map[string]*ast.Module{"": mod}); c.Failed() {
		return nil, c.Errors[0]
	}

	compiled := c.Modules[""].Rules[0].Body

	ctx := topdown.NewContext(compiled, s.Runtime.DataStore)

	results := resultSetV1{}

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	err = topdown.Eval(ctx, func(ctx *topdown.Context) error {
		result := map[string]interface{}{}
		var err error
		ctx.Locals.Iter(func(k, v ast.Value) bool {
			kv, ok := k.(ast.Var)
			if !ok {
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

func (s *Server) registerHandlerV1(path string, method string, h func(http.ResponseWriter, *http.Request)) {
	s.Router.HandleFunc("/v1"+path, h).Methods(method)
}

func (s *Server) v1DataGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := splitPath(vars["path"])
	globals, err := parseGlobals(r.URL.Query()["global"])
	if err != nil {
		handleError(w, 400, err)
		return
	}
	params := topdown.NewQueryParams(s.Runtime.DataStore, globals, path)

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	result, err := topdown.Query(params)

	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	handleResponseJSON(w, 200, result)
}

func (s *Server) v1DataPatch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	root := splitPath(vars["path"])

	ops := []patchV1{}
	if err := json.NewDecoder(r.Body).Decode(&ops); err != nil {
		handleError(w, 400, err)
		return
	}

	path := []interface{}{}
	for _, x := range root {
		path = append(path, x)
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

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

		parts := splitPath(ops[i].Path)
		for _, x := range parts {
			if x == "" {
				continue
			}
			path = append(path, x)
		}

		if err := s.Runtime.DataStore.Patch(op, path, ops[i].Value); err != nil {
			handleErrorAuto(w, err)
			return
		}

		path = path[:len(root)]
	}

	handleResponse(w, 204, nil)
}

func (s *Server) v1PoliciesDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	_, err := s.Runtime.PolicyStore.Get(id)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	mods := s.Runtime.PolicyStore.List()
	delete(mods, id)

	c := ast.NewCompiler()
	if c.Compile(mods); c.Failed() {
		handleErrorf(w, 400, c.FlattenErrors())
		return
	}

	if err := s.Runtime.PolicyStore.Remove(id); err != nil {
		handleErrorAuto(w, err)
		return
	}

	handleResponse(w, 204, nil)
}

func (s *Server) v1PoliciesGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	mod, err := s.Runtime.PolicyStore.Get(id)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	policy := &policyV1{
		ID:     id,
		Module: mod,
	}

	handleResponseJSONPretty(w, 200, policy)
}

func (s *Server) v1PoliciesRawGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	bs, err := s.Runtime.PolicyStore.GetRaw(id)

	if err != nil {
		handleErrorAuto(w, err)
		return
	}

	handleResponse(w, 200, bs)
}

func (s *Server) v1PoliciesList(w http.ResponseWriter, r *http.Request) {

	policies := []*policyV1{}

	s.mtx.RLock()
	defer s.mtx.RUnlock()

	for id, mod := range s.Runtime.PolicyStore.List() {
		policy := &policyV1{
			ID:     id,
			Module: mod,
		}
		policies = append(policies, policy)
	}

	handleResponseJSONPretty(w, 200, policies)
}

func (s *Server) v1PoliciesPut(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	id := vars["id"]

	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleError(w, 500, err)
		return
	}

	mod, err := ast.ParseModule(string(buf))
	if err != nil {
		handleError(w, 400, err)
		return
	}
	if mod == nil {
		handleErrorf(w, 400, "refusing to add empty module")
		return
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	mods := s.Runtime.PolicyStore.List()
	mods[id] = mod

	c := ast.NewCompiler()

	if c.Compile(mods); c.Failed() {
		handleErrorf(w, 400, c.FlattenErrors())
		return
	}

	mod = c.Modules[id]

	if err := s.Runtime.PolicyStore.Add(id, mod, buf, s.Persist); err != nil {
		handleErrorAuto(w, err)
		return
	}

	policy := &policyV1{
		ID:     id,
		Module: mod,
	}

	handleResponseJSONPretty(w, 200, policy)
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

	handleResponseJSON(w, 200, results)
}

func splitPath(s string) []interface{} {
	p := strings.Split(s, "/")
	r := []interface{}{}
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

func handleResponseJSON(w http.ResponseWriter, code int, v interface{}) {
	bs, err := json.Marshal(v)
	if err != nil {
		handleErrorAuto(w, err)
		return
	}
	headers := w.Header()
	headers.Add("Content-Type", "application/json")
	handleResponse(w, code, bs)
}

func handleResponseJSONPretty(w http.ResponseWriter, code int, v interface{}) {
	bs, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		handleErrorAuto(w, err)
		return
	}
	headers := w.Header()
	headers.Add("Content-Type", "application/json")
	handleResponse(w, code, bs)
}

func parseGlobals(globals []string) (*storage.Bindings, error) {
	r := storage.NewBindings()
	for _, g := range globals {
		vs := strings.SplitN(g, ":", 2)
		k, err := ast.ParseTerm(vs[0])
		if err != nil {
			return nil, err
		}
		v, err := ast.ParseTerm(vs[1])
		if err != nil {
			return nil, err
		}
		r.Put(k.Value, v.Value)
	}
	return r, nil
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
	fmt.Fprintln(w, "Open Policy Agent - An open source project to policy enable any application.<br>")
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
