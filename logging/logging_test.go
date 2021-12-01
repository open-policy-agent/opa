package logging

import (
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
