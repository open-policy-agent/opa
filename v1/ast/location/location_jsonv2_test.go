//go:build go1.27

package location_test

import (
	"encoding/json/jsontext"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast/location"
)

func TestUnmarshalFromFn(t *testing.T) {
	txt := `{"file":"p.rego","row":1,"col":2,"text":"dGVzdA=="}`
	dec := jsontext.NewDecoder(strings.NewReader(txt), location.UnmarshalOpts)
	loc := &location.Location{}

	if err := location.UnmarshalFromFn(dec, loc); err != nil {
		t.Fatal(err)
	}
	if loc.File != "p.rego" {
		t.Errorf("Expected file to be 'p.rego', got: %v", loc.File)
	}
	if loc.Row != 1 {
		t.Errorf("Expected row to be 1, got: %v", loc.Row)
	}
	if loc.Col != 2 {
		t.Errorf("Expected col to be 2, got: %v", loc.Col)
	}
	if loc.Text != nil {
		t.Errorf("Expected text to be empty, got: %v", string(loc.Text))
	}
}
