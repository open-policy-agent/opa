// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package log is a wrapper for logrus Go logging package
package log

import (
	"context"
	"io"

	"github.com/sirupsen/logrus"
)

// Fields aliases logrus.Fields
type Fields = logrus.Fields

// Entry aliases logrus.Entry
type Entry = logrus.Entry

// Logger is the interface for loggers that can be used by applications.
type Logger interface {
	Debug(...interface{})
	Debugf(string, ...interface{})
	Debugln(...interface{})

	Info(...interface{})
	Infof(string, ...interface{})
	Infoln(...interface{})

	Warn(...interface{})
	Warnf(string, ...interface{})
	Warnln(...interface{})

	Error(...interface{})
	Errorf(string, ...interface{})
	Errorln(...interface{})

	Fatal(...interface{})
	Fatalln(...interface{})
	Fatalf(string, ...interface{})

	Panic(...interface{})
	Panicln(...interface{})
	Panicf(string, ...interface{})

	WithField(key string, value interface{}) *Entry
	WithFields(Fields) *Entry

	SetLevel(string) error
	SetOutput(io.Writer)
	SetJSONFormatter()

	WithContext(context.Context) Logger
}

type logger struct {
	entry *logrus.Entry
}

// NewLogger creates a new logger.
func NewLogger() Logger {
	l := logrus.New()
	return logger{entry: logrus.NewEntry(l)}
}

// WithContext adds a context to the Entry.
func (l logger) WithContext(ctx context.Context) Logger {
	return logger{l.entry.WithContext(ctx)}
}

// Debug logs a message at level Debug on the logger.
func (l logger) Debug(args ...interface{}) {
	l.entry.Debug(args...)
}

// Debugf logs a message at level Debug on the logger.
func (l logger) Debugf(format string, args ...interface{}) {
	l.entry.Debugf(format, args...)
}

// Debugln logs a message at level Debug on the logger.
func (l logger) Debugln(args ...interface{}) {
	l.entry.Debugln(args...)
}

// Info logs a message at level Info on the logger.
func (l logger) Info(args ...interface{}) {
	l.entry.Info(args...)
}

// Infof logs a message at level Info on the logger.
func (l logger) Infof(format string, args ...interface{}) {
	l.entry.Infof(format, args...)
}

// Infoln logs a message at level Info on the logger.
func (l logger) Infoln(args ...interface{}) {
	l.entry.Infoln(args...)
}

// Warn logs a message at level Warn on the logger.
func (l logger) Warn(args ...interface{}) {
	l.entry.Warn(args...)
}

// Warnf logs a message at level Warn on the logger.
func (l logger) Warnf(format string, args ...interface{}) {
	l.entry.Warnf(format, args...)
}

// Warnln logs a message at level Warn on the logger.
func (l logger) Warnln(args ...interface{}) {
	l.entry.Warnln(args...)
}

// Error logs a message at level Error on the logger.
func (l logger) Error(args ...interface{}) {
	l.entry.Error(args...)
}

// Errorf logs a message at level Error on the logger.
func (l logger) Errorf(format string, args ...interface{}) {
	l.entry.Errorf(format, args...)
}

// Errorln logs a message at level Error on the logger.
func (l logger) Errorln(args ...interface{}) {
	l.entry.Errorln(args...)
}

// Fatal logs a message at level Fatal on the logger.
func (l logger) Fatal(args ...interface{}) {
	l.entry.Fatal(args...)
}

// Fatalf logs a message at level Fatal on the logger.
func (l logger) Fatalf(format string, args ...interface{}) {
	l.entry.Fatalf(format, args...)
}

// Fatalln logs a message at level Fatal on the logger.
func (l logger) Fatalln(args ...interface{}) {
	l.entry.Fatalln(args...)
}

// Panic logs a message at level Panic on the logger.
func (l logger) Panic(args ...interface{}) {
	l.entry.Panic(args...)
}

// Panicf logs a message at level Panic on the logger.
func (l logger) Panicf(format string, args ...interface{}) {
	l.entry.Panicf(format, args...)
}

// Panicln logs a message at level Panic on the logger.
func (l logger) Panicln(args ...interface{}) {
	l.entry.Panicln(args...)
}

// WithField adds a field to the logger.
func (l logger) WithField(key string, value interface{}) *Entry {
	return l.entry.WithField(key, value)
}

// WithFields adds a map of fields to the logger.
func (l logger) WithFields(fields Fields) *Entry {
	return l.entry.WithFields(fields)
}

// SetLevel sets the logger level.
func (l logger) SetLevel(level string) error {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}

	l.entry.Logger.SetLevel(lvl)
	return nil
}

// SetOutput sets the logger output.
func (l logger) SetOutput(w io.Writer) {
	l.entry.Logger.SetOutput(w)
}

// SetJSONFormatter sets the logger formatter to JSONFormatter.
func (l logger) SetJSONFormatter() {
	l.entry.Logger.SetFormatter(&logrus.JSONFormatter{})
}

var origLogger = logrus.New()
var globalLogger = logger{entry: logrus.NewEntry(origLogger)}

// Global returns the default logger.
func Global() Logger {
	return globalLogger
}

// WithContext adds a context to the Entry.
func WithContext(ctx context.Context) Logger {
	return logger{globalLogger.entry.WithContext(ctx)}
}

// Debug logs a message at level Debug on the logger.
func Debug(args ...interface{}) {
	globalLogger.entry.Debug(args...)
}

// Debugf logs a message at level Debug on the logger.
func Debugf(format string, args ...interface{}) {
	globalLogger.entry.Debugf(format, args...)
}

// Debugln logs a message at level Debug on the logger.
func Debugln(args ...interface{}) {
	globalLogger.entry.Debugln(args...)
}

// Info logs a message at level Info on the logger.
func Info(args ...interface{}) {
	globalLogger.entry.Info(args...)
}

// Infof logs a message at level Info on the logger.
func Infof(format string, args ...interface{}) {
	globalLogger.entry.Infof(format, args...)
}

// Infoln logs a message at level Info on the logger.
func Infoln(args ...interface{}) {
	globalLogger.entry.Infoln(args...)
}

// Warn logs a message at level Warn on the logger.
func Warn(args ...interface{}) {
	globalLogger.entry.Warn(args...)
}

// Warnf logs a message at level Warn on the logger.
func Warnf(format string, args ...interface{}) {
	globalLogger.entry.Warnf(format, args...)
}

// Warnln logs a message at level Warn on the logger.
func Warnln(args ...interface{}) {
	globalLogger.entry.Warnln(args...)
}

// Error logs a message at level Error on the logger.
func Error(args ...interface{}) {
	globalLogger.entry.Error(args...)
}

// Errorf logs a message at level Error on the logger.
func Errorf(format string, args ...interface{}) {
	globalLogger.entry.Errorf(format, args...)
}

// Errorln logs a message at level Error on the logger.
func Errorln(args ...interface{}) {
	globalLogger.entry.Errorln(args...)
}

// Fatal logs a message at level Fatal on the logger.
func Fatal(args ...interface{}) {
	globalLogger.entry.Fatal(args...)
}

// Fatalf logs a message at level Fatal on the logger.
func Fatalf(format string, args ...interface{}) {
	globalLogger.entry.Fatalf(format, args...)
}

// Fatalln logs a message at level Fatal on the logger.
func Fatalln(args ...interface{}) {
	globalLogger.entry.Fatalln(args...)
}

// Panic logs a message at level Panic on the logger.
func Panic(args ...interface{}) {
	globalLogger.entry.Panic(args...)
}

// Panicf logs a message at level Panic on the logger.
func Panicf(format string, args ...interface{}) {
	globalLogger.entry.Panicf(format, args...)
}

// Panicln logs a message at level Panic on the logger.
func Panicln(args ...interface{}) {
	globalLogger.entry.Panicln(args...)
}

// WithField adds a field to the logger.
func WithField(key string, value interface{}) *Entry {
	return globalLogger.entry.WithField(key, value)
}

// WithFields adds a map of fields to the logger.
func WithFields(fields Fields) *Entry {
	return globalLogger.entry.WithFields(fields)
}

// SetLevel sets the logger level.
func SetLevel(level string) error {
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}

	origLogger.SetLevel(lvl)
	return nil
}

// SetOutput sets the logger output.
func SetOutput(w io.Writer) {
	origLogger.SetOutput(w)
}

// SetJSONFormatter sets the logger formatter to JSONFormatter.
func SetJSONFormatter() {
	origLogger.SetFormatter(&logrus.JSONFormatter{})
}
