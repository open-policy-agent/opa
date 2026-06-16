// Copyright 2026 The OPA Authors. All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package logging

import (
	"context"
	"log/slog"
)

// AsSlogLogger returns a *slog.Logger that forwards log records to the given Logger.
// Structured fields are forwarded via WithFields; log levels map to the equivalent
// Logger methods. Groups are not supported and are ignored.
func AsSlogLogger(logger Logger) *slog.Logger {
	return slog.New(slogHandler{logger: logger})
}

// AsSlogLoggerWithPinnedLevel returns a *slog.Logger that forwards all log records
// to the given Logger at the specified fixed level, ignoring the slog record's own level.
// This is useful when bridging a library that uses slog into OPA's logger at a fixed verbosity.
func AsSlogLoggerWithPinnedLevel(logger Logger, level Level) *slog.Logger {
	return slog.New(slogHandler{logger: logger, pinnedLevel: &level})
}

type slogHandler struct {
	logger      Logger
	pinnedLevel *Level
}

func (slogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h slogHandler) Handle(_ context.Context, r slog.Record) error {
	fields := make(map[string]any, r.NumAttrs())

	r.Attrs(func(a slog.Attr) bool {
		fields[a.Key] = a.Value.Any()

		return true
	})

	l := h.logger.WithFields(fields)

	if h.pinnedLevel != nil {
		dispatchAtLevel(l, *h.pinnedLevel, r.Message)
		return nil
	}

	// slog.Level is int; cascade from most-severe so custom levels between named ones route correctly.
	switch {
	case r.Level >= slog.LevelError:
		l.Error(r.Message)
	case r.Level >= slog.LevelWarn:
		l.Warn(r.Message)
	case r.Level >= slog.LevelInfo:
		l.Info(r.Message)
	default:
		l.Debug(r.Message)
	}

	return nil
}

func (h slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields := make(map[string]any, len(attrs))

	for _, a := range attrs {
		fields[a.Key] = a.Value.Any()
	}

	return slogHandler{logger: h.logger.WithFields(fields), pinnedLevel: h.pinnedLevel}
}

func (h slogHandler) WithGroup(_ string) slog.Handler { return h }

func dispatchAtLevel(l Logger, level Level, msg string) {
	switch level {
	case Error:
		l.Error(msg)
	case Warn:
		l.Warn(msg)
	case Info:
		l.Info(msg)
	default:
		l.Debug(msg)
	}
}
