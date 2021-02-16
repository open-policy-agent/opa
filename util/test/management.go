package test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Server management API server for testing
type Server struct {
	HTTPServer *httptest.Server

	t       *testing.T
	rootDir string
	fsClean func()
	expCode int
}

// NewTestManagementServer instantiates a new management API server backend for testing
func NewTestManagementServer(t *testing.T) *Server {
	fs, fsCleanup, err := MakeTempFS("", "opa_management", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	server := &Server{
		t:       t,
		rootDir: fs,
		fsClean: fsCleanup,
		expCode: 200,
	}

	return server
}

// WithBundleAtEndpoint take files at path and package as bundle, served at endpoint
func (t *Server) WithBundleAtEndpoint(buf *bytes.Buffer, endpoint string) *Server {
	target := filepath.Join(t.rootDir, endpoint)

	err := os.MkdirAll(filepath.Dir(target), os.ModePerm)
	if err != nil {
		t.t.Fatalf("unexpected error %v", err)
	}
	out, err := os.Create(target)
	if err != nil {
		t.t.Fatalf("unexpected error %v", err)
	}
	defer out.Close()

	_, err = io.Copy(out, buf)
	if err != nil {
		t.t.Fatalf("unexpected error %v", err)
	}

	return t
}

func (t *Server) handle(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/bundles") {
		t.bundleHandler(w, r)
	} else {
		t.t.Logf("no handle registered for path %v", r.URL.Path)
		w.WriteHeader(t.expCode)
	}
}

func (t *Server) bundleHandler(w http.ResponseWriter, r *http.Request) {
	http.FileServer(http.Dir(t.rootDir)).ServeHTTP(w, r)
}

// Start the TestServer.
func (t *Server) Start() *Server {
	t.HTTPServer = httptest.NewServer(http.HandlerFunc(t.handle))

	return t
}

// Stop the TestServer.
func (t *Server) Stop() *Server {
	if t.HTTPServer != nil {
		t.HTTPServer.Close()
	}

	return t
}

// Cleanup runs cleanup procedure
func (t *Server) Cleanup() {
	t.fsClean()
}
