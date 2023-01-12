package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/open-policy-agent/opa/router"
)

var _ router.RouterI = (*GinEngineWrapper)(nil)

type GinEngineWrapper struct {
	engine *gin.Engine
}

func (g *GinEngineWrapper) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.engine.ServeHTTP(w, r)
}

func New() *GinEngineWrapper {
	return &GinEngineWrapper{engine: gin.New()}
}

func wrap(h http.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		params := make(map[string]string, len(c.Params))
		for _, param := range c.Params {
			params[param.Key] = param.Value
		}
		c.Request = c.Request.Clone(router.SetParams(c.Request.Context(), params))

		h.ServeHTTP(c.Writer, c.Request)
	}
}

func (g *GinEngineWrapper) Handle(method, path string, handler http.Handler) {
	g.engine.Handle(method, path, wrap(handler))
}

func (g *GinEngineWrapper) Any(path string, handler http.Handler) {
	g.engine.Any(path, wrap(handler))
}

func (g *GinEngineWrapper) EscapePath(value bool) {
	g.engine.UnescapePathValues = !value
	g.engine.UseRawPath = value
}

func (g *GinEngineWrapper) RedirectTrailingSlash(value bool) {
	g.engine.RedirectTrailingSlash = value
}

func (g *GinEngineWrapper) HandleMethodNotAllowed(h http.Handler) {
	g.engine.HandleMethodNotAllowed = true
	g.engine.NoMethod(gin.WrapH(h))
}
