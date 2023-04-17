package logging

import (
	"bytes"
	"context"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/uuid"
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

func TestRequestContextFields(t *testing.T) {
	fields := RequestContext{
		ClientAddr: "127.0.0.1",
		ReqID:      1,
		ReqMethod:  "GET",
		ReqPath:    "/test",
	}.Fields()

	var fieldvalue interface{}
	var ok bool

	if fieldvalue, ok = fields["client_addr"]; !ok {
		t.Fatal("Fields did not contain the client_addr field")
	}

	if fieldvalue.(string) != "127.0.0.1" {
		t.Fatal("Fields did not contain the configured client_addr value")
	}

	if fieldvalue, ok = fields["req_id"]; !ok {
		t.Fatal("Fields did not contain the req_id field")
	}

	if fieldvalue.(uint64) != 1 {
		t.Fatal("Fields did not contain the configured req_id value")
	}

	if fieldvalue, ok = fields["req_method"]; !ok {
		t.Fatal("Fields did not contain the req_method field")
	}

	if fieldvalue.(string) != "GET" {
		t.Fatal("Fields did not contain the configured req_method value")
	}

	if fieldvalue, ok = fields["req_path"]; !ok {
		t.Fatal("Fields did not contain the req_path field")
	}

	if fieldvalue.(string) != "/test" {
		t.Fatal("Fields did not contain the configured req_path value")
	}
}

func TestDecsionIDFromContext(t *testing.T) {
	id, err := uuid.New(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithDecisionID(context.Background(), id)

	act, ok := DecisionIDFromContext(ctx)
	if !ok {
		t.Fatalf("expected 'ok' to be true")
	}
	if exp := id; act != exp {
		t.Errorf("Expected %q to be %q", act, exp)
	}
}
