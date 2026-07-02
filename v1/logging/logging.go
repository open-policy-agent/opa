package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// Level log level for Logger
type Level uint8

const (
	// Error error log level
	Error Level = iota
	// Warn warn log level
	Warn
	// Info info log level
	Info
	// Debug debug log level
	Debug
)

// Logger provides interface for OPA logger implementations
type Logger interface {
	Debug(fmt string, a ...any)
	Info(fmt string, a ...any)
	Error(fmt string, a ...any)
	Warn(fmt string, a ...any)

	WithFields(map[string]any) Logger

	GetLevel() Level
	SetLevel(Level)
}

// LoggerWithContext is an optional interface that Logger implementations
// can implement to support extracting trace information from a context.
// Use WithContext to call this method on a Logger if it is supported.
type LoggerWithContext interface {
	WithContext(context.Context) Logger
}

// WithContext returns a logger with context information if the logger
// supports it (i.e., implements LoggerWithContext). Otherwise, the
// logger is returned unchanged.
func WithContext(logger Logger, ctx context.Context) Logger {
	if lc, ok := logger.(LoggerWithContext); ok {
		return lc.WithContext(ctx)
	}
	return logger
}

// StandardLogger is the default OPA logger implementation.
type StandardLogger struct {
	logger *logrus.Logger
	fields map[string]any
}

// New returns a new standard logger.
func New() *StandardLogger {
	return &StandardLogger{
		logger: logrus.New(),
	}
}

// Get returns the standard logger used throughout OPA.
//
// Deprecated. Do not rely on the global logger.
func Get() *StandardLogger {
	return &StandardLogger{
		logger: logrus.StandardLogger(),
	}
}

// SetOutput sets the underlying logrus output.
func (l *StandardLogger) SetOutput(w io.Writer) {
	l.logger.SetOutput(w)
}

// SetFormatter sets the underlying logrus formatter.
func (l *StandardLogger) SetFormatter(formatter logrus.Formatter) {
	l.logger.SetFormatter(formatter)
}

// WithFields provides additional fields to include in log output
func (l *StandardLogger) WithFields(fields map[string]any) Logger {
	cp := *l
	cp.fields = make(map[string]any)
	maps.Copy(cp.fields, l.fields)
	maps.Copy(cp.fields, fields)
	return &cp
}

// getFields returns additional fields of this logger
func (l *StandardLogger) getFields() map[string]any {
	return l.fields
}

// SetLevel sets the standard logger level.
func (l *StandardLogger) SetLevel(level Level) {
	var logrusLevel logrus.Level
	switch level {
	case Error: // set logging level report Warn or higher (includes Error)
		logrusLevel = logrus.WarnLevel
	case Warn:
		logrusLevel = logrus.WarnLevel
	case Info:
		logrusLevel = logrus.InfoLevel
	case Debug:
		logrusLevel = logrus.DebugLevel
	default:
		l.Warn("unknown log level %v", level)
		logrusLevel = logrus.InfoLevel
	}

	l.logger.SetLevel(logrusLevel)
}

// GetLevel returns the standard logger level.
func (l *StandardLogger) GetLevel() Level {
	logrusLevel := l.logger.GetLevel()

	var level Level
	switch logrusLevel {
	case logrus.WarnLevel:
		level = Error
	case logrus.InfoLevel:
		level = Info
	case logrus.DebugLevel:
		level = Debug
	default:
		l.Warn("unknown log level %v", logrusLevel)
		level = Info
	}

	return level
}

// Debug logs at debug level
func (l *StandardLogger) Debug(fmt string, a ...any) {
	if len(a) == 0 {
		l.logger.WithFields(l.getFields()).Debug(fmt)
		return
	}
	l.logger.WithFields(l.getFields()).Debugf(fmt, a...)
}

// Info logs at info level
func (l *StandardLogger) Info(fmt string, a ...any) {
	if len(a) == 0 {
		l.logger.WithFields(l.getFields()).Info(fmt)
		return
	}
	l.logger.WithFields(l.getFields()).Infof(fmt, a...)
}

// Error logs at error level
func (l *StandardLogger) Error(fmt string, a ...any) {
	if len(a) == 0 {
		l.logger.WithFields(l.getFields()).Error(fmt)
		return
	}
	l.logger.WithFields(l.getFields()).Errorf(fmt, a...)
}

// Warn logs at warn level
func (l *StandardLogger) Warn(fmt string, a ...any) {
	if len(a) == 0 {
		l.logger.WithFields(l.getFields()).Warn(fmt)
		return
	}
	l.logger.WithFields(l.getFields()).Warnf(fmt, a...)
}

// NoOpLogger logging implementation that does nothing
type NoOpLogger struct {
	level  Level
	fields map[string]any
}

// NewNoOpLogger instantiates new NoOpLogger
func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{
		level: Info,
	}
}

// WithFields provides additional fields to include in log output.
// Implemented here primarily to be able to switch between implementations without loss of data.
func (l *NoOpLogger) WithFields(fields map[string]any) Logger {
	cp := *l
	cp.fields = fields
	return &cp
}

// Debug noop
func (*NoOpLogger) Debug(string, ...any) {}

// Info noop
func (*NoOpLogger) Info(string, ...any) {}

// Error noop
func (*NoOpLogger) Error(string, ...any) {}

// Warn noop
func (*NoOpLogger) Warn(string, ...any) {}

// SetLevel set log level
func (l *NoOpLogger) SetLevel(level Level) {
	l.level = level
}

// GetLevel get log level
func (l *NoOpLogger) GetLevel() Level {
	return l.level
}

type requestContextKey string

const reqCtxKey = requestContextKey("request-context-key")

// RequestContext represents the request context used to store data
// related to the request that could be used on logs.
type RequestContext struct {
	ClientAddr         string
	ReqID              uint64
	ReqMethod          string
	ReqPath            string
	HTTPRequestContext HTTPRequestContext
}

type HTTPRequestContext struct {
	Header http.Header
}

// Fields adapts the RequestContext fields to logrus.Fields.
func (rctx RequestContext) Fields() logrus.Fields {
	return logrus.Fields{
		"client_addr": rctx.ClientAddr,
		"req_id":      rctx.ReqID,
		"req_method":  rctx.ReqMethod,
		"req_path":    rctx.ReqPath,
	}
}

// NewContext returns a copy of parent with an associated RequestContext.
func NewContext(parent context.Context, val *RequestContext) context.Context {
	return context.WithValue(parent, reqCtxKey, val)
}

// FromContext returns the RequestContext associated with ctx, if any.
func FromContext(ctx context.Context) (*RequestContext, bool) {
	requestContext, ok := ctx.Value(reqCtxKey).(*RequestContext)
	return requestContext, ok
}

const httpReqCtxKey = requestContextKey("http-request-context-key")

func WithHTTPRequestContext(parent context.Context, val *HTTPRequestContext) context.Context {
	return context.WithValue(parent, httpReqCtxKey, val)
}

func HTTPRequestContextFromContext(ctx context.Context) (*HTTPRequestContext, bool) {
	requestContext, ok := ctx.Value(httpReqCtxKey).(*HTTPRequestContext)
	return requestContext, ok
}

const decisionCtxKey = requestContextKey("decision_id")

func WithDecisionID(parent context.Context, id string) context.Context {
	return context.WithValue(parent, decisionCtxKey, id)
}

func DecisionIDFromContext(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(decisionCtxKey).(string)
	return s, ok
}

const batchDecisionCtxKey = requestContextKey("batch_decision_id")

func WithBatchDecisionID(parent context.Context, id string) context.Context {
	return context.WithValue(parent, batchDecisionCtxKey, id)
}

func BatchDecisionIDFromContext(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(batchDecisionCtxKey).(string)
	return s, ok
}

// SlogHandler adapts a Logger to slog.Handler interface
type SlogHandler struct {
	logger Logger
}

// NewSlogHandler creates an slog.Handler from a Logger
func NewSlogHandler(logger Logger) slog.Handler {
	return &SlogHandler{logger: logger}
}

func (h *SlogHandler) Enabled(_ context.Context, level slog.Level) bool {
	lvl := h.logger.GetLevel()
	switch level {
	case slog.LevelDebug:
		return lvl >= Debug
	case slog.LevelInfo:
		return lvl >= Info
	case slog.LevelWarn:
		return lvl >= Warn
	case slog.LevelError:
		return lvl >= Error
	}
	return true
}

func (h *SlogHandler) Handle(ctx context.Context, record slog.Record) error {
	attrs := make(map[string]any)
	record.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})

	logger := h.logger
	if len(attrs) > 0 {
		logger = logger.WithFields(attrs)
	}
	if ctx != nil {
		logger = WithContext(logger, ctx)
	}

	msg := record.Message
	switch record.Level {
	case slog.LevelDebug:
		logger.Debug(msg)
	case slog.LevelInfo:
		logger.Info(msg)
	case slog.LevelWarn:
		logger.Warn(msg)
	case slog.LevelError:
		logger.Error(msg)
	default:
		logger.Info(msg)
	}
	return nil
}

func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields := make(map[string]any, len(attrs))
	for _, a := range attrs {
		fields[a.Key] = a.Value.Any()
	}
	return &SlogHandler{
		logger: h.logger.WithFields(fields),
	}
}

func (h *SlogHandler) WithGroup(name string) slog.Handler {
	return h
}

// loggerFromSlogHandler wraps an slog.Handler to implement the Logger interface
type loggerFromSlogHandler struct {
	handler slog.Handler
	level   Level
	fields  map[string]any
	ctx     context.Context
}

var _ LoggerWithContext = (*loggerFromSlogHandler)(nil)

// NewLoggerFromSlogHandler creates a Logger from an slog.Handler
func NewLoggerFromSlogHandler(handler slog.Handler, level Level) Logger {
	return &loggerFromSlogHandler{
		handler: handler,
		level:   level,
		fields:  make(map[string]any),
		ctx:     context.Background(),
	}
}

func (l *loggerFromSlogHandler) log(level slog.Level, format string, args ...any) {
	if !l.handler.Enabled(l.ctx, level) {
		return
	}
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	attrs := make([]slog.Attr, 0, len(l.fields))
	for k, v := range l.fields {
		var attr slog.Attr
		switch val := v.(type) {
		case string:
			attr = slog.String(k, val)
		case int:
			attr = slog.Int(k, val)
		case int64:
			attr = slog.Int64(k, val)
		case uint64:
			attr = slog.Uint64(k, val)
		case float64:
			attr = slog.Float64(k, val)
		case bool:
			attr = slog.Bool(k, val)
		case time.Time:
			attr = slog.Time(k, val)
		case time.Duration:
			attr = slog.Duration(k, val)
		default:
			attr = slog.Any(k, v)
		}
		attrs = append(attrs, attr)
	}

	record := slog.NewRecord(time.Now(), level, msg, 0)
	record.AddAttrs(attrs...)

	_ = l.handler.Handle(l.ctx, record)
}

func (l *loggerFromSlogHandler) Debug(format string, args ...any) {
	l.log(slog.LevelDebug, format, args...)
}

func (l *loggerFromSlogHandler) Info(format string, args ...any) {
	l.log(slog.LevelInfo, format, args...)
}

func (l *loggerFromSlogHandler) Warn(format string, args ...any) {
	l.log(slog.LevelWarn, format, args...)
}

func (l *loggerFromSlogHandler) Error(format string, args ...any) {
	l.log(slog.LevelError, format, args...)
}

func (l *loggerFromSlogHandler) WithFields(fields map[string]any) Logger {
	merged := make(map[string]any, len(l.fields)+len(fields))
	maps.Copy(merged, l.fields)
	maps.Copy(merged, fields)
	return &loggerFromSlogHandler{
		handler: l.handler,
		level:   l.level,
		fields:  merged,
		ctx:     l.ctx,
	}
}

func (l *loggerFromSlogHandler) WithContext(ctx context.Context) Logger {
	return &loggerFromSlogHandler{
		handler: l.handler,
		level:   l.level,
		fields:  l.fields,
		ctx:     ctx,
	}
}

func (l *loggerFromSlogHandler) GetLevel() Level {
	return l.level
}

func (l *loggerFromSlogHandler) SetLevel(level Level) {
	l.level = level
}
