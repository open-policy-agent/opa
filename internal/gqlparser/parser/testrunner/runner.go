package testrunner

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/andreyvit/diff"
	"github.com/open-policy-agent/opa/internal/gqlparser/gqlerror"
	"gopkg.in/yaml.v2"
)

type Features map[string][]Spec

type Spec struct {
	Name   string
	Input  string
	Error  *gqlerror.Error
	Tokens []Token
	AST    string
}

type Token struct {
	Kind   string
	Value  string
	Start  int
	End    int
	Line   int
	Column int
	Src    string
}

func (t Token) String() string {
	return t.Kind + " " + strconv.Quote(t.Value)
}

func Test(t *testing.T, filename string, f func(t *testing.T, input string) Spec) {
	b, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}
	var tests Features
	err = yaml.Unmarshal(b, &tests)
	if err != nil {
		t.Errorf("unable to load %s: %s", filename, err.Error())
		return
	}

	for name, specs := range tests {
		t.Run(name, func(t *testing.T) {
			for _, spec := range specs {
				t.Run(spec.Name, func(t *testing.T) {
					result := f(t, spec.Input)

					if spec.Error == nil {
						if result.Error != nil {
							gqlErr := err.(*gqlerror.Error)
							t.Errorf("unexpected error %s", gqlErr.Message)
						}
					} else if result.Error == nil {
						t.Errorf("expected error but got none")
					} else {
						if result.Error.Message != spec.Error.Message {
							t.Errorf("wrong error returned\nexpected: %s\ngot:      %s", spec.Error.Message, result.Error.Message)
						}

						if result.Error.Locations[0].Column != spec.Error.Locations[0].Column || result.Error.Locations[0].Line != spec.Error.Locations[0].Line {
							t.Errorf(
								"wrong error location:\nexpected: line %d column %d\ngot:      line %d column %d",
								spec.Error.Locations[0].Line,
								spec.Error.Locations[0].Column,
								result.Error.Locations[0].Line,
								result.Error.Locations[0].Column,
							)
						}
					}

					if len(spec.Tokens) != len(result.Tokens) {
						var tokensStr []string
						for _, t := range result.Tokens {
							tokensStr = append(tokensStr, t.String())
						}
						t.Errorf("token count mismatch, got: \n%s", strings.Join(tokensStr, "\n"))
					} else {
						for i, tok := range result.Tokens {
							expected := spec.Tokens[i]

							if !strings.EqualFold(strings.Replace(expected.Kind, "_", "", -1), tok.Kind) {
								t.Errorf("token[%d].kind should be %s, was %s", i, expected.Kind, tok.Kind)
							}
							if expected.Value != "undefined" && expected.Value != tok.Value {
								t.Errorf("token[%d].value incorrect\nexpected: %s\ngot:      %s", i, strconv.Quote(expected.Value), strconv.Quote(tok.Value))
							}
							if expected.Start != 0 && expected.Start != tok.Start {
								t.Errorf("token[%d].start should be %d, was %d", i, expected.Start, tok.Start)
							}
							if expected.End != 0 && expected.End != tok.End {
								t.Errorf("token[%d].end should be %d, was %d", i, expected.End, tok.End)
							}
							if expected.Line != 0 && expected.Line != tok.Line {
								t.Errorf("token[%d].line should be %d, was %d", i, expected.Line, tok.Line)
							}
							if expected.Column != 0 && expected.Column != tok.Column {
								t.Errorf("token[%d].column should be %d, was %d", i, expected.Column, tok.Column)
							}
							if tok.Src != "spec" {
								t.Errorf("token[%d].source.name should be spec, was %s", i, strconv.Quote(tok.Src))
							}
						}
					}

					spec.AST = strings.TrimSpace(spec.AST)
					result.AST = strings.TrimSpace(result.AST)

					if spec.AST != "" && spec.AST != result.AST {
						diff := diff.LineDiff(spec.AST, result.AST)
						if diff != "" {
							t.Errorf("AST mismatch:\n%s", diff)
						}
					}

					if t.Failed() {
						t.Logf("input: %s", strconv.Quote(spec.Input))
						if result.Error != nil {
							t.Logf("error: %s", result.Error.Message)
						}
						t.Log("tokens: ")
						for _, tok := range result.Tokens {
							t.Logf("  - %s", tok.String())
						}
						t.Logf("  - <EOF>")
					}
				})
			}
		})
	}

}
