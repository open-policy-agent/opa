package validator

import (
	"testing"

	"github.com/open-policy-agent/opa/internal/gqlparser/ast"
)

func Test_sameArguments(t *testing.T) {
	tests := map[string]struct {
		args   func() (args1, args2 []*ast.Argument)
		result bool
	}{
		"both argument lists empty": {
			args: func() (args1 []*ast.Argument, args2 []*ast.Argument) {
				return nil, nil
			},
			result: true,
		},
		"args 1 empty, args 2 not": {
			args: func() (args1 []*ast.Argument, args2 []*ast.Argument) {
				return nil, []*ast.Argument{
					{
						Name:     "thing",
						Value:    &ast.Value{Raw: "a thing"},
						Position: &ast.Position{},
					},
				}
			},
			result: false,
		},
		"args 2 empty, args 1 not": {
			args: func() (args1 []*ast.Argument, args2 []*ast.Argument) {
				return []*ast.Argument{
					{
						Name:     "thing",
						Value:    &ast.Value{Raw: "a thing"},
						Position: &ast.Position{},
					},
				}, nil
			},
			result: false,
		},
		"args 1 mismatches args 2 names": {
			args: func() (args1 []*ast.Argument, args2 []*ast.Argument) {
				return []*ast.Argument{
						{
							Name:     "thing1",
							Value:    &ast.Value{Raw: "1 thing"},
							Position: &ast.Position{},
						},
					},
					[]*ast.Argument{
						{
							Name:     "thing2",
							Value:    &ast.Value{Raw: "2 thing"},
							Position: &ast.Position{},
						},
					}
			},
			result: false,
		},
		"args 1 mismatches args 2 values": {
			args: func() (args1 []*ast.Argument, args2 []*ast.Argument) {
				return []*ast.Argument{
						{
							Name:     "thing1",
							Value:    &ast.Value{Raw: "1 thing"},
							Position: &ast.Position{},
						},
					},
					[]*ast.Argument{
						{
							Name:     "thing1",
							Value:    &ast.Value{Raw: "2 thing"},
							Position: &ast.Position{},
						},
					}
			},
			result: false,
		},
		"args 1 matches args 2 names and values": {
			args: func() (args1 []*ast.Argument, args2 []*ast.Argument) {
				return []*ast.Argument{
						{
							Name:     "thing1",
							Value:    &ast.Value{Raw: "1 thing"},
							Position: &ast.Position{},
						},
					},
					[]*ast.Argument{
						{
							Name:     "thing1",
							Value:    &ast.Value{Raw: "1 thing"},
							Position: &ast.Position{},
						},
					}
			},
			result: true,
		},
		"args 1 matches args 2 names and values where multiple exist in various orders": {
			args: func() (args1 []*ast.Argument, args2 []*ast.Argument) {
				return []*ast.Argument{
						{
							Name:     "thing1",
							Value:    &ast.Value{Raw: "1 thing"},
							Position: &ast.Position{},
						},
						{
							Name:     "thing2",
							Value:    &ast.Value{Raw: "2 thing"},
							Position: &ast.Position{},
						},
					},
					[]*ast.Argument{
						{
							Name:     "thing1",
							Value:    &ast.Value{Raw: "1 thing"},
							Position: &ast.Position{},
						},
						{
							Name:     "thing2",
							Value:    &ast.Value{Raw: "2 thing"},
							Position: &ast.Position{},
						},
					}
			},
			result: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			args1, args2 := tc.args()

			resp := sameArguments(args1, args2)

			if resp != tc.result {
				t.Fatalf("Expected %t got %t", tc.result, resp)
			}
		})
	}
}
