package test

// This file collects some helpers for generating data used in
// benchmarks,
// - topdown/topdown_bench_test.go

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

// ArrayIterationBenchmarkModule returns a module that iterates an array
// with `n` elements
func ArrayIterationBenchmarkModule(n int) string {
	return fmt.Sprintf(`package test

	fixture = [ x | x := numbers.range(1, %d)[_] ]

	main { fixture[i] }`, n)
}

// SetIterationBenchmarkModule returns a module that iterates a set
// with `n` elements
func SetIterationBenchmarkModule(n int) string {
	return fmt.Sprintf(`package test

	fixture = { x | x := numbers.range(1, %d)[_] }

	main { fixture[i] }`, n)
}

// ObjectIterationBenchmarkModule returns a module that iterates an object
// with `n` key/val pairs
func ObjectIterationBenchmarkModule(n int) string {
	return fmt.Sprintf(`package test

	fixture = { x: x | x := numbers.range(1, %d)[_] }

	main { fixture[i] }`, n)
}

// GenerateLargeJSONBenchmarkData returns a map of 100 keys and 100.000 key/value
// pairs.
func GenerateLargeJSONBenchmarkData() map[string]interface{} {
	return GenerateJSONBenchmarkData(100, 100*1000)
}

// GenerateJSONBenchmarkData returns a map of `k` keys and `v` key/value pairs.
func GenerateJSONBenchmarkData(k, v int) map[string]interface{} {

	// create array of null values that can be iterated over
	keys := make([]interface{}, k)
	for i := range keys {
		keys[i] = nil
	}

	// create large JSON object value (100,000 entries is about 2MB on disk)
	values := map[string]interface{}{}
	for i := 0; i < v; i++ {
		values[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	return map[string]interface{}{
		"keys":   keys,
		"values": values,
	}
}

// GenerateConcurrencyBenchmarkData returns a module and data; the module
// checks some input parameters against that data in a simple API authz
// scheme.
func GenerateConcurrencyBenchmarkData() (string, map[string]interface{}) {
	obj := []byte(`
		{
			"objs": [
				{
					"attr1": "get",
					"path": "/foo/bar",
					"user": "bob"
				},
				{
					"attr1": "set",
					"path": "/foo/bar/baz",
					"user": "alice"
				},
				{
					"attr1": "get",
					"path": "/foo",
					"groups": [
						"admin",
						"eng"
					]
				},
				{
					"path": "/foo/bar",
					"user": "alice"
				}
			]
		}
		`)

	var data map[string]interface{}
	if err := json.Unmarshal(obj, &data); err != nil {
		panic(err)
	}
	mod := `package test

	import data.objs

	p {
		objs[i].attr1 = "get"
		objs[i].groups[j] = "eng"
	}

	p {
		objs[i].user = "alice"
	}
	`

	return mod, data
}

// GenerateVirtualDocsBenchmarkData generates a module and input; the
// numTotalRules and numHitRules create as many rules in the module to
// match/miss the returned input.
func GenerateVirtualDocsBenchmarkData(numTotalRules, numHitRules int) (string, map[string]interface{}) {

	hitRule := `
	allow {
		input.method = "POST"
		input.path = ["accounts", account_id]
		input.user_id = account_id
	}
	`

	missRule := `
	allow {
		input.method = "GET"
		input.path = ["salaries", account_id]
		input.user_id = account_id
	}
	`

	testModuleTmpl := `package a.b.c

	{{range .MissRules }}
		{{ . }}
	{{end}}

	{{range .HitRules }}
		{{ . }}
	{{end}}
	`

	tmpl, err := template.New("Test").Parse(testModuleTmpl)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer

	var missRules []string

	if numTotalRules > numHitRules {
		missRules = make([]string, numTotalRules-numHitRules)
		for i := range missRules {
			missRules[i] = missRule
		}
	}

	hitRules := make([]string, numHitRules)
	for i := range hitRules {
		hitRules[i] = hitRule
	}

	params := struct {
		MissRules []string
		HitRules  []string
	}{
		MissRules: missRules,
		HitRules:  hitRules,
	}

	err = tmpl.Execute(&buf, params)
	if err != nil {
		panic(err)
	}

	input := map[string]interface{}{
		"path":    []interface{}{"accounts", "alice"},
		"method":  "POST",
		"user_id": "alice",
	}

	return buf.String(), input
}
