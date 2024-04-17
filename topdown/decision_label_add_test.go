package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

func TestBuiltinDecisionLabelAdd(t *testing.T) {

	// inputs
	bctx := BuiltinContext{DecisionLabel: builtins.DecisionLabel{}}
	key := ast.StringTerm("foo")
	value := ast.StringTerm("bar")
	inputs := []*ast.Term{
		key,
		value,
	}

	if err := builtinDecisionLabelAdd(bctx, inputs, nil); err != nil {
		t.Fatalf("Value %s for Key %s was not added to the DecisionLabel Object.", value.Value.String(), key.Value.String())
	}

}

func TestBuiltinDecisionLabelAddInvalidInputs(t *testing.T) {

	// inputs
	bctx := BuiltinContext{DecisionLabel: builtins.DecisionLabel{}}
	key := ast.IntNumberTerm(1337)
	value := ast.IntNumberTerm(1812)
	inputs := []*ast.Term{
		key,
		value,
	}

	if err := builtinDecisionLabelAdd(bctx, inputs, nil); err == nil {
		t.Fatalf("Invalid inputs for Key %s and Value %s were added to the DecisionLabel Object.", key.Value.String(), value.Value.String())
	}

}

func TestBuiltinDecisionLabelAddSameKeySecondEntry(t *testing.T) {

	bctx := BuiltinContext{DecisionLabel: builtins.DecisionLabel{}}
	key := ast.StringTerm("foo")
	value1, value2 := ast.StringTerm("bar"), ast.StringTerm("baz")
	inputs1, inputs2 := []*ast.Term{
		key,
		value1,
	},
		[]*ast.Term{
			key,
			value2,
		}

	if err := builtinDecisionLabelAdd(bctx, inputs1, nil); err != nil {
		t.Fatalf("First pair of Key %s and Value %s was not properly assigned.", key.Value.String(), value1.Value.String())
	}

	if err := builtinDecisionLabelAdd(bctx, inputs2, nil); err != nil {
		t.Fatalf("Second pair of Key %s and Value %s was not properly assigned.", key.Value.String(), value2.Value.String())
	}

	if value, ok := bctx.DecisionLabel.Get(ast.String(key.String())); ok {
		if value == value1.Value {
			t.Fatalf("Original value %s still present for Key %s after secondary assignment.", value1.Value.String(), key.Value.String())
		}
	}

}
