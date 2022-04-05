package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestWithFields(t *testing.T) {
	logger := New().WithFields(map[string]interface{}{"context": "contextvalue"})

	var fieldvalue interface{}
	var ok bool

	if fieldvalue, ok = logger.(*StandardLogger).fields["context"]; !ok {
		t.Fatal("Logger did not contain configured field")
	}

	if fieldvalue.(string) != "contextvalue" {
		t.Fatal("Logger did not contain configured field value")
	}
}

func TestCaptureWarningWithErrorSet(t *testing.T) {
	buf := bytes.Buffer{}
	logger := New()
	logger.SetOutput(&buf)
	logger.SetLevel(Error)

	logger.Warn("This is a warning. Next time, I won't compile.")
	logger.Error("Fix your issues. I'm not compiling.")

	expected := []string{
		`level=warning msg="This is a warning. Next time, I won't compile."`,
		`level=error msg="Fix your issues. I'm not compiling."`,
	}
	for _, exp := range expected {
		if !strings.Contains(buf.String(), exp) {
			t.Errorf("expected string %q not found in logs", exp)
		}
	}
}

func TestWithFieldsOverrides(t *testing.T) {
	logger := New().
		WithFields(map[string]interface{}{"context": "contextvalue"}).
		WithFields(map[string]interface{}{"context": "changedcontextvalue"})

	var fieldvalue interface{}
	var ok bool

	if fieldvalue, ok = logger.(*StandardLogger).fields["context"]; !ok {
		t.Fatal("Logger did not contain configured field")
	}

	if fieldvalue.(string) != "changedcontextvalue" {
		t.Fatal("Logger did not contain configured field value")
	}
}

func TestWithFieldsMerges(t *testing.T) {
	logger := New().
		WithFields(map[string]interface{}{"context": "contextvalue"}).
		WithFields(map[string]interface{}{"anothercontext": "anothercontextvalue"})

	var fieldvalue interface{}
	var ok bool

	if fieldvalue, ok = logger.(*StandardLogger).fields["context"]; !ok {
		t.Fatal("Logger did not contain configured field")
	}

	if fieldvalue.(string) != "contextvalue" {
		t.Fatal("Logger did not contain configured field value")
	}

	if fieldvalue, ok = logger.(*StandardLogger).fields["anothercontext"]; !ok {
		t.Fatal("Logger did not contain configured field")
	}

	if fieldvalue.(string) != "anothercontextvalue" {
		t.Fatal("Logger did not contain configured field value")
	}
}
