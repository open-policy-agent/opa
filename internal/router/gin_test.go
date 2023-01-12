package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/open-policy-agent/opa/router"
	"github.com/stretchr/testify/assert"
)

func TestGinEngineWrapper_HandleMethodNotAllowed(t *testing.T) {
	g := New()
	g.Handle(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))

	r, _ := http.NewRequest(http.MethodHead, "/", nil)
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)

	result := w.Result()
	assert := assert.New(t)
	assert.Equal(http.StatusNotFound, result.StatusCode)

	g.HandleMethodNotAllowed(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not found"))
	}))

	r, _ = http.NewRequest(http.MethodHead, "/", nil)
	w = httptest.NewRecorder()
	g.ServeHTTP(w, r)

	result = w.Result()
	assert.Equal(http.StatusMethodNotAllowed, result.StatusCode)
	body, err := io.ReadAll(result.Body)
	assert.NoErrorf(err, "error reading result body")
	assert.Equal(string(body), "not found")
}

type fixture struct {
	g *GinEngineWrapper
}

func (f *fixture) doReq(t *testing.T, method string, path string, status int) {
	r, _ := http.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	f.g.ServeHTTP(w, r)

	result := w.Result()
	assert := assert.New(t)
	assert.Equal(status, result.StatusCode)
}
func TestGinEngineWrapper_StrictSlash(t *testing.T) {
	g := New()
	g.Handle(http.MethodGet, "/without", okHandler())
	g.Handle(http.MethodGet, "/with/", okHandler())

	f := fixture{g}

	g.RedirectTrailingSlash(false)

	type req []struct {
		name   string
		path   string
		status int
	}

	reqs := req{
		{
			name:   "/without",
			path:   "/without",
			status: http.StatusOK,
		},
		{
			name:   "/without/",
			path:   "/without/",
			status: http.StatusNotFound,
		},
		{
			name:   "/with/",
			path:   "/with/",
			status: http.StatusOK,
		},
		{
			name:   "/with",
			path:   "/with",
			status: http.StatusNotFound,
		},
	}
	for _, req := range reqs {
		t.Run("disabled_strict_flag-"+req.name, func(t *testing.T) {
			f.doReq(t, http.MethodGet, req.path, req.status)
		})
	}

	g.RedirectTrailingSlash(true)

	reqs = req{
		{
			name:   "/without",
			path:   "/without",
			status: http.StatusOK,
		},
		{
			name:   "/without/",
			path:   "/without/",
			status: http.StatusMovedPermanently,
		},
		{
			name:   "/with/",
			path:   "/with/",
			status: http.StatusOK,
		},
		{
			name:   "/with",
			path:   "/with",
			status: http.StatusMovedPermanently,
		},
	}
	for _, req := range reqs {
		t.Run("enabled_strict_flag_-"+req.name, func(t *testing.T) {
			f.doReq(t, http.MethodGet, req.path, req.status)
		})
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
}

func TestGinEngineWrapper_EscapePath(t *testing.T) {
	type req []struct {
		name       string
		path       string
		status     int
		EscapePath bool
		expected   string
	}

	reqs := req{
		{
			name:       "/a%2Fb/c",
			path:       "/a%2Fb/c",
			status:     http.StatusOK,
			EscapePath: true,
			expected:   "/a%2Fb/c",
		},
		{
			name:       "/a%2Fb/c-unescape",
			path:       "/a%2Fb/c",
			status:     http.StatusOK,
			EscapePath: false,
			expected:   "/a/b/c",
		},
	}
	for _, req := range reqs {
		t.Run(req.name, func(t *testing.T) {
			var pth string
			g := New()
			g.Handle(http.MethodGet, "/*path", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				pth = router.GetParams(r.Context())["path"]
			}))
			g.EscapePath(req.EscapePath)

			f := fixture{g}
			f.doReq(t, http.MethodGet, req.path, req.status)

			assert.Equal(t, req.expected, pth)
		})
	}
}

func TestGinEngineWrapper_Any(t *testing.T) {
	g := New()
	g.Any("/", okHandler())
	f := fixture{g}

	methods := []string{
		http.MethodConnect,
		http.MethodDelete,
		http.MethodGet,
		http.MethodHead,
		http.MethodOptions,
		http.MethodPatch,
		http.MethodPost,
		http.MethodPut,
		http.MethodTrace,
	}

	for _, m := range methods {
		t.Run(m, func(t *testing.T) {
			f.doReq(t, m, "/", 200)
		})
	}
}
