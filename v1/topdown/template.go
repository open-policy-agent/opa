package topdown

import (
	"bytes"
	"strings"

	// A method-less copy of text/template (see internal/methodlesstemplate). Rego values
	// decode to map[string]any/[]any/scalars, which have no methods, so eliding
	// method calls is a no-op here; it keeps text/template's evalField
	// MethodByName off the reachable graph, which otherwise disables the Go
	// linker's method-level dead-code elimination binary-wide (golang/go#72895,
	// #7903).
	template "github.com/open-policy-agent/opa/internal/methodlesstemplate"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func renderTemplate(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	preContentTerm, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	templateVariablesTerm, err := builtins.ObjectOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	var templateVariables map[string]any

	if err := ast.As(templateVariablesTerm, &templateVariables); err != nil {
		return err
	}

	tmpl, err := template.New("template").Parse(string(preContentTerm))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateVariables); err != nil {
		return err
	}

	res := strings.ReplaceAll(buf.String(), "<no value>", "<undefined>")
	return iter(ast.StringTerm(res))
}

func init() {
	RegisterBuiltinFunc(ast.RenderTemplate.Name, renderTemplate)
}
