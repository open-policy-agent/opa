package validator_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-policy-agent/opa/internal/gqlparser"
	"github.com/open-policy-agent/opa/internal/gqlparser/ast"
	"github.com/open-policy-agent/opa/internal/gqlparser/validator"
)

func TestValidateVars(t *testing.T) {
	schema := gqlparser.MustLoadSchema(&ast.Source{
		Name:  "vars.graphql",
		Input: mustReadFile("./testdata/vars.graphql"),
	})

	t.Run("undefined variable", func(t *testing.T) {
		t.Run("without default", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query($id: Int!) { intArg(i: $id) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), nil)
			require.EqualError(t, gerr, "input: variable.id must be defined")
		})

		t.Run("nil in required value", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query($id: Int!) { intArg(i: $id) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"id": nil,
			})
			require.EqualError(t, gerr, "input: variable.id cannot be null")
		})

		t.Run("with default", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query($id: Int! = 1) { intArg(i: $id) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), nil)
			require.Nil(t, gerr)
			require.EqualValues(t, 1, vars["id"])
		})

		t.Run("with union", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query($id: Int! = 1) { intArg(i: $id) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), nil)
			require.Nil(t, gerr)
			require.EqualValues(t, 1, vars["id"])
		})
	})

	t.Run("input object", func(t *testing.T) {
		t.Run("non object", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType!) { structArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": "hello",
			})
			require.EqualError(t, gerr, "input: variable.var must be a InputType")
		})

		t.Run("defaults", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType! = {name: "foo"}) { structArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), nil)
			require.Nil(t, gerr)
			require.EqualValues(t, map[string]interface{}{"name": "foo"}, vars["var"])
		})

		t.Run("valid value", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType!) { structArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": map[string]interface{}{
					"name": "foobar",
				},
			})
			require.Nil(t, gerr)
			require.EqualValues(t, map[string]interface{}{"name": "foobar"}, vars["var"])
		})

		t.Run("null object field", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType!) { structArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": map[string]interface{}{
					"name":     "foobar",
					"nullName": nil,
				},
			})
			require.Nil(t, gerr)
			require.EqualValues(t, map[string]interface{}{"name": "foobar", "nullName": nil}, vars["var"])
		})

		t.Run("missing required values", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType!) { structArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": map[string]interface{}{},
			})
			require.EqualError(t, gerr, "input: variable.var.name must be defined")
		})

		t.Run("null required field", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType!) { structArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": map[string]interface{}{
					"name": nil,
				},
			})
			require.EqualError(t, gerr, "input: variable.var.name cannot be null")
		})

		t.Run("null embedded input object", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType!) { structArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": map[string]interface{}{
					"name":         "foo",
					"nullEmbedded": nil,
				},
			})
			require.Nil(t, gerr)
		})

		t.Run("unknown field", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType!) { structArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": map[string]interface{}{
					"name":    "foobar",
					"foobard": true,
				},
			})
			require.EqualError(t, gerr, "input: variable.var.foobard unknown field")
		})

		t.Run("unknown __typefield", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType!) { structArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": map[string]interface{}{
					"name":       "foobar",
					"__typename": "InputType",
				},
			})
			require.Nil(t, gerr)
			require.EqualValues(t, map[string]interface{}{"__typename": "InputType", "name": "foobar"}, vars["var"])

		})

		t.Run("enum input object", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType!) { structArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": map[string]interface{}{
					"name": "foobar",
					"enum": "A",
				},
			})
			require.Nil(t, gerr)
		})

		t.Run("unknown enum value input object", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: InputType!) { structArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": map[string]interface{}{
					"name": "foobar",
					"enum": "B",
				},
			})
			require.EqualError(t, gerr, "input: variable.var.enum B is not a valid Enum")
		})
	})

	t.Run("array", func(t *testing.T) {
		t.Run("non-null object value should be coerced to an array", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: [InputType!]) { arrayArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": map[string]interface{}{"name": "hello"},
			})
			require.Nil(t, gerr)
			require.EqualValues(t, []map[string]interface{}{{"name": "hello"}}, vars["var"])
		})

		t.Run("non-null int value should be coerced to an array", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: [Int!]) { intArrayArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": 5,
			})
			require.Nil(t, gerr)
			expected := []int{5}
			require.EqualValues(t, expected, vars["var"])
		})

		t.Run("non-null int deep value should be coerced to an array", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: [CustomType]) { typeArrayArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": []map[string]interface{}{{"and": 5}},
			})
			require.Nil(t, gerr)
			expected := []map[string]interface{}{{"and": []int{5}}}
			require.EqualValues(t, expected, vars["var"])
		})

		t.Run("defaults", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: [InputType!] = [{name: "foo"}]) { arrayArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), nil)
			require.Nil(t, gerr)
			require.EqualValues(t, []interface{}{map[string]interface{}{
				"name": "foo",
			}}, vars["var"])
		})

		t.Run("valid value", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: [InputType!]) { arrayArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": []interface{}{map[string]interface{}{
					"name": "foo",
				}},
			})
			require.Nil(t, gerr)
			require.EqualValues(t, []interface{}{map[string]interface{}{
				"name": "foo",
			}}, vars["var"])
		})

		t.Run("null element value", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: [InputType!]) { arrayArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": []interface{}{nil},
			})
			require.EqualError(t, gerr, "input: variable.var[0] cannot be null")
		})

		t.Run("missing required values", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: [InputType!]) { arrayArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": []interface{}{map[string]interface{}{}},
			})
			require.EqualError(t, gerr, "input: variable.var[0].name must be defined")
		})
		t.Run("invalid variable paths", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var1: InputType!, $var2: InputType!) { a:structArg(i: $var1) b:structArg(i: $var2) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var1": map[string]interface{}{
					"name": "foobar",
				},
				"var2": map[string]interface{}{
					"nullName": "foobar",
				},
			})
			require.EqualError(t, gerr, "input: variable.var2.name must be defined")
		})
	})

	t.Run("Scalars", func(t *testing.T) {
		t.Run("String -> String", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: String!) { stringArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": "asdf",
			})
			require.Nil(t, gerr)
			require.EqualValues(t, "asdf", vars["var"])
		})

		t.Run("Int -> String", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: String!) { stringArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": 1,
			})
			require.EqualError(t, gerr, "input: variable.var cannot use int as String")
		})

		t.Run("Nil -> String", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: String!) { stringArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": nil,
			})
			require.EqualError(t, gerr, "input: variable.var cannot be null")
		})

		t.Run("Undefined -> String!", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: String!) { stringArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), nil)
			require.EqualError(t, gerr, "input: variable.var must be defined")
		})

		t.Run("Undefined -> Int", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: Int) { optionalIntArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), nil)
			require.Nil(t, gerr)
		})

		t.Run("Json Number -> Int", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: Int) { optionalIntArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": 10,
			})
			require.Nil(t, gerr)
			require.Equal(t, 10, vars["var"])
		})

		t.Run("Json Number -> Float", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: Float!) { floatArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": 10.2,
			})
			require.Nil(t, gerr)
			require.Equal(t, 10.2, vars["var"])
		})

		t.Run("Nil -> Int", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: Int) { optionalIntArg(i: $var) }`)
			vars, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": nil,
			})
			require.Nil(t, gerr)
			require.Equal(t, nil, vars["var"])
		})

		t.Run("Bool -> Int", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: Int!) { intArg(i: $var) }`)
			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": true,
			})
			require.EqualError(t, gerr, "input: variable.var cannot use bool as Int")
		})
	})

	t.Run("Int Array", func(t *testing.T) {
		t.Run("Array with null", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: [Int]) { intArrayArg(i: $var) }`)
			a := 1
			b := 2

			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": []*int{&a, &b, nil},
			})
			require.Nil(t, gerr)
		})
	})

	t.Run("String Array", func(t *testing.T) {
		t.Run("Array with null", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: [String]) { stringArrayArg(i: $var) }`)
			a := "1"
			b := "2"

			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": []*string{&a, &b, nil},
			})
			require.Nil(t, gerr)
		})
	})

	t.Run("Boolean Array", func(t *testing.T) {
		t.Run("Array with null", func(t *testing.T) {
			q := gqlparser.MustLoadQuery(schema, `query foo($var: [Boolean]) { boolArrayArg(i: $var) }`)
			a := true
			b := false

			_, gerr := validator.VariableValues(schema, q.Operations.ForName(""), map[string]interface{}{
				"var": []*bool{&a, &b, nil},
			})
			require.Nil(t, gerr)
		})
	})
}

func mustReadFile(name string) string {
	src, err := os.ReadFile(name)
	if err != nil {
		panic(err)
	}

	return string(src)
}
