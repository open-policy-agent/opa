// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"net/http"
	"slices"

	serverDecodingPlugin "github.com/open-policy-agent/opa/v1/plugins/server/decoding"
	serverEncodingPlugin "github.com/open-policy-agent/opa/v1/plugins/server/encoding"
	"github.com/open-policy-agent/opa/v1/server/authorizer"
	"github.com/open-policy-agent/opa/v1/server/handlers"
	"github.com/open-policy-agent/opa/v1/server/identifier"
	"github.com/open-policy-agent/opa/v1/tracing"
)

// middleware is an alias so that the ones plugins supply are interchangeable
// with ours.
type middleware = func(http.Handler) http.Handler

// chain applies mw outermost first, skipping nil entries.
func chain(mw ...middleware) middleware {
	return func(handler http.Handler) http.Handler {
		for _, m := range slices.Backward(mw) {
			if m != nil {
				handler = m(handler)
			}
		}
		return handler
	}
}

// authnMiddleware is nil when authentication is disabled.
func (s *Server) authnMiddleware() middleware {
	switch s.authentication {
	case AuthenticationToken:
		return func(next http.Handler) http.Handler { return identifier.NewTokenBased(next) }
	case AuthenticationTLS:
		return func(next http.Handler) http.Handler { return identifier.NewTLSBased(next) }
	}

	return nil
}

// authzMiddleware is nil when authorization is disabled. It reads the
// inter-query caches off the server, so it must not be built before they exist.
func (s *Server) authzMiddleware() middleware {
	if s.authorization != AuthorizationBasic {
		return nil
	}

	return func(next http.Handler) http.Handler {
		handler := authorizer.NewBasic(
			next,
			s.getCompiler,
			s.store,
			authorizer.Runtime(s.runtime),
			authorizer.Decision(s.manager.GetConfig().DefaultAuthorizationDecisionRef),
			authorizer.PrintHook(s.manager.PrintHook()),
			authorizer.EnablePrintStatements(s.manager.EnablePrintStatements()),
			authorizer.InterQueryCache(s.interQueryBuiltinCache),
			authorizer.InterQueryValueCache(s.interQueryBuiltinValueCache),
			authorizer.URLPathExpectsBodyFunc(s.manager.ExtraAuthorizerRoutes()),
		)
		if s.metrics != nil {
			return s.instrumentHandler(handler.ServeHTTP, PromHandlerAPIAuthz)
		}
		return handler
	}
}

func (s *Server) compressionMiddleware(ctx context.Context) (middleware, error) {
	var raw []byte
	if cfg := s.manager.GetConfig(); cfg.Server != nil {
		raw = []byte(cfg.Server.Encoding)
	}

	config, err := serverEncodingPlugin.NewConfigBuilder().WithBytes(raw).ParseWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return handlers.CompressHandler(next, *config.Gzip.MinLength, *config.Gzip.CompressionLevel)
	}, nil
}

// decodingLimitsMiddleware passes the gzip size limit down to the body-reading
// method via the request context.
func (s *Server) decodingLimitsMiddleware(ctx context.Context) (middleware, error) {
	var raw []byte
	if cfg := s.manager.GetConfig(); cfg.Server != nil {
		raw = []byte(cfg.Server.Decoding)
	}

	config, err := serverDecodingPlugin.NewConfigBuilder().WithBytes(raw).ParseWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return func(next http.Handler) http.Handler {
		return handlers.DecodingLimitsHandler(next, *config.MaxLength, *config.Gzip.MaxLength)
	}, nil
}

func (s *Server) instrumentHandler(handler http.HandlerFunc, label string) http.Handler {
	var metricsMW, tracingMW middleware
	if s.metrics != nil {
		metricsMW = func(next http.Handler) http.Handler {
			return s.metrics.InstrumentHandler(next, label)
		}
	}
	if len(s.distributedTracingOpts) > 0 {
		tracingMW = func(next http.Handler) http.Handler {
			return tracing.NewHandler(next, label, s.distributedTracingOpts)
		}
	}

	extras := s.manager.ExtraMiddlewares()
	mw := make([]middleware, 0, len(extras)+3)
	mw = append(mw, metricsMW, tracingMW, handlers.DefaultHandler)
	mw = append(mw, extras...)

	return chain(mw...)(handler)
}
