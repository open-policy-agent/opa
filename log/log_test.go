// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package log

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestInfo(t *testing.T) {

	var buffer bytes.Buffer
	var fields Fields

	logger := getLogger(&buffer)

	logger.Info("Hello")

	err := json.Unmarshal(buffer.Bytes(), &fields)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertResult(t, fields["level"], "info")
	assertResult(t, fields["msg"], "Hello")
}

func TestDebug(t *testing.T) {

	var buffer bytes.Buffer
	var fields Fields

	logger := getLogger(&buffer)
	logger.SetLevel("debug")

	logger.Debugf("Hello %v", "World")

	err := json.Unmarshal(buffer.Bytes(), &fields)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertResult(t, fields["level"], "debug")
	assertResult(t, fields["msg"], "Hello World")
}

func TestError(t *testing.T) {

	var buffer bytes.Buffer
	var fields Fields

	logger := getLogger(&buffer)
	logger.SetLevel("error")

	logger.Errorln("Bad Error")

	err := json.Unmarshal(buffer.Bytes(), &fields)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertResult(t, fields["level"], "error")
	assertResult(t, fields["msg"], "Bad Error")
}

func TestWarn(t *testing.T) {

	var buffer bytes.Buffer
	var fields Fields

	logger := getLogger(&buffer)
	logger.SetLevel("Warn")

	logger.Warn("Bad Warning")

	err := json.Unmarshal(buffer.Bytes(), &fields)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertResult(t, fields["level"], "warning")
	assertResult(t, fields["msg"], "Bad Warning")
}

func TestWithField(t *testing.T) {

	var buffer bytes.Buffer
	var fields Fields

	logger := getLogger(&buffer)

	entry := logger.WithField("foo", "bar")

	entry.Info("Hello")

	err := json.Unmarshal(buffer.Bytes(), &fields)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertResult(t, fields["foo"], "bar")
}

func TestWithFields(t *testing.T) {

	var buffer bytes.Buffer
	var fields Fields

	logger := getLogger(&buffer)

	entry := logger.WithFields(Fields{
		"foo": "bar",
	})

	entry.Info("Hello")

	err := json.Unmarshal(buffer.Bytes(), &fields)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertResult(t, fields["foo"], "bar")
}

func TestWithContext(t *testing.T) {

	var buffer bytes.Buffer
	var fields Fields

	type ctxKey struct{}

	ctx := context.WithValue(context.Background(), ctxKey{}, "bar")

	logger := getLogger(&buffer)
	logger = logger.WithContext(ctx)

	entry := logger.WithFields(Fields{
		"baz": "test",
	})

	entry.Info("Hello")

	err := json.Unmarshal(buffer.Bytes(), &fields)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertResult(t, entry.Context, ctx)
}

func TestGlobalInfo(t *testing.T) {

	var buffer bytes.Buffer
	var fields Fields

	SetOutput(&buffer)
	SetJSONFormatter()

	Info("Hello Global")

	err := json.Unmarshal(buffer.Bytes(), &fields)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertResult(t, fields["level"], "info")
	assertResult(t, fields["msg"], "Hello Global")
}

func assertResult(t *testing.T, actual, expected interface{}) {
	if actual != expected {
		t.Fatalf("Expected result %v but got %v", expected, actual)
	}
}

func getLogger(w io.Writer) Logger {
	logger := NewLogger()
	logger.SetOutput(w)
	logger.SetJSONFormatter()
	return logger
}
