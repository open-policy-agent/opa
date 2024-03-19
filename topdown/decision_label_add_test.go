package topdown

import (
	"github.com/open-policy-agent/opa/ast"
	"testing"
)

func TestValidateKeyStringOperand(t *testing.T) {

	input := ast.StringTerm("foo")

	if _, err := validateKeyStringOperand(input, 1); err != nil {
		t.Fatalf("String Key %s was not properly validated.", input.String())
	}

}

func TestValidateKeyStringOperandInvalidKeyType(t *testing.T) {

	input := ast.IntNumberTerm(1337)

	if _, err := validateKeyStringOperand(input, 1); err == nil {
		t.Fatalf("Non-String Key %s passed validation.", input.Value)
	}

}

func TestValidateValueStringOperand(t *testing.T) {

	input := ast.StringTerm("bar")

	if _, err := validateValueStringOperand(input, 2); err != nil {
		t.Fatalf("String Value %s was not properly validated.", input.String())
	}

}

func TestValidateValueStringOperandInvalidValueType(t *testing.T) {

	input := ast.IntNumberTerm(1338)

	if _, err := validateValueStringOperand(input, 2); err == nil {
		t.Fatalf("Non-String Value %s passed validation.", input.Value)
	}

}

func TestAssignOperandsToDecisionLabel(t *testing.T) {

	//ctx := context.Background()
	//key := ast.StringTerm("foo")
	//value := ast.StringTerm("bar")

}

func TestAssignOperandsToDecisionLabelSameKeySecondEntry(t *testing.T) {

}
