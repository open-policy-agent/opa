package topdown_test

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown"
)

// Policies that spend their time calling functions and builtins, rather than
// looking up rules or data.
var evalCallsPolicies = []struct{ name, module string }{
	{"FnChain", `package test
f1(x) := x + 1
f2(x) := f1(x) * 2
f3(x) := f2(x) + f1(x)
f4(x) := f3(x) - f2(x)
result := count([i | some i in numbers.range(1, 300); f4(i) > 100])
`},
	{"HelperFilter", `package test
has_role(user, role) if { some r in user.roles; r == role }
is_old(user) if { user.age > 40 }
in_dept(user, d) if { user.dept == d }
result := [u.name | some u in input.users; has_role(u, "admin"); is_old(u); in_dept(u, "eng"); u.active]
`},
	{"Strings", `package test
norm(s) := lower(trim_space(s))
keep(s) if { startswith(norm(s), "a"); not endswith(norm(s), "z") }
result := count([n | some u in input.users; n := norm(u.name); keep(u.name)])
`},
	{"ObjFilter", `package test
result := count([x | some x in input.items; x.kind == "a"; x.size > 10; x.size < 90; x.tags[_] == "t3"])
`},
	{"Nested", `package test
g1(a, b) := a + b
g2(a, b) := g1(a, b) * g1(b, a)
g3(a, b) := g2(a, b) - g1(a, a)
result := [g3(x, y) | some x in numbers.range(1, 30); some y in numbers.range(1, 30); g2(x, y) > 500]
`},
}

func evalCallsInput() *ast.Term {
	users := make([]any, 400)
	for i := range users {
		roles := []any{"user"}
		if i%7 == 0 {
			roles = append(roles, "admin")
		}
		dept := "ops"
		if i%3 == 0 {
			dept = "eng"
		}
		users[i] = map[string]any{"name": fmt.Sprintf(" %cUser%d ", 'A'+rune(i%26), i), "roles": roles, "age": 20 + i%40, "dept": dept, "active": i%5 != 0}
	}
	items := make([]any, 500)
	for i := range items {
		kind := "b"
		if i%2 == 0 {
			kind = "a"
		}
		items[i] = map[string]any{"kind": kind, "size": i % 100, "tags": []any{fmt.Sprintf("t%d", i%5), fmt.Sprintf("t%d", i%7)}}
	}
	return ast.NewTerm(ast.MustInterfaceToValue(map[string]any{"users": users, "items": items}))
}

func BenchmarkEvalCalls(b *testing.B) {
	input := evalCallsInput()
	for _, tc := range evalCallsPolicies {
		b.Run(tc.name, func(b *testing.B) {
			compiler := ast.MustCompileModules(map[string]string{"test.rego": tc.module})
			store := inmem.New()
			query := ast.MustParseBody("data.test.result")
			b.ReportAllocs()
			for b.Loop() {
				q := topdown.NewQuery(query).WithCompiler(compiler).WithStore(store).WithInput(input)
				if _, err := q.Run(b.Context()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
