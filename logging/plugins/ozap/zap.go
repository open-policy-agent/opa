package ozap

import (
	"fmt"

	"github.com/open-policy-agent/opa/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Wrap this zap Logger and AtomicLevel as a logging.Logger.
func Wrap(log *zap.Logger, level *zap.AtomicLevel) logging.Logger {
	return &Wrapper{internal: log, level: level}
}

// Wrapper implements logging.Logger for a zap Logger.
type Wrapper struct {
	internal *zap.Logger
	level    *zap.AtomicLevel
}

// Debug logs at debug level.
func (w *Wrapper) Debug(f string, a ...interface{}) {
	w.internal.Debug(fmt.Sprintf(f, a...))
}

// Info logs at info level.
func (w *Wrapper) Info(f string, a ...interface{}) {
	w.internal.Info(fmt.Sprintf(f, a...))
}

// Error logs at error level.
func (w *Wrapper) Error(f string, a ...interface{}) {
	w.internal.Error(fmt.Sprintf(f, a...))
}

// Warn logs at warn level.
func (w *Wrapper) Warn(f string, a ...interface{}) {
	w.internal.Warn(fmt.Sprintf(f, a...))
}

// WithFields provides additional fields to include in log output.
func (w *Wrapper) WithFields(fields map[string]interface{}) logging.Logger {
	return &Wrapper{
		internal: w.internal.With(toZapFields(fields)...),
		level:    w.level,
	}
}

// toZapFields converts logging format fields to zap format Fields
func toZapFields(fields map[string]interface{}) []zap.Field {
	var zapFields []zap.Field
	for k, v := range fields {
		switch t := v.(type) {
		case error:
			zapFields = append(zapFields, zap.NamedError(k, t))
		case string:
			zapFields = append(zapFields, zap.String(k, t))
		case bool:
			zapFields = append(zapFields, zap.Bool(k, t))
		case int:
			zapFields = append(zapFields, zap.Int(k, t))
		default:
			zapFields = append(zapFields, zap.Any(k, v))
		}
	}
	return zapFields
}

// SetLevel sets the logger level.
func (w *Wrapper) GetLevel() logging.Level {
	switch w.internal.Level() {
	case zap.ErrorLevel:
		return logging.Error
	case zap.WarnLevel:
		return logging.Warn
	case zap.DebugLevel:
		return logging.Debug
	default:
		return logging.Info
	}
}

// SetLevel sets the logger level.
func (w *Wrapper) SetLevel(l logging.Level) {
	var newLevel zapcore.Level
	switch l {
	case logging.Error:
		newLevel = zap.ErrorLevel
	case logging.Warn:
		newLevel = zap.WarnLevel
	case logging.Info:
		newLevel = zap.InfoLevel
	case logging.Debug:
		newLevel = zap.DebugLevel
	default:
		return
	}
	w.level.SetLevel(newLevel)
}
