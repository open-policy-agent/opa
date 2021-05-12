// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/util"
)

// BenchmarkParseModuleRulesBase gives a baseline for parsing modules with
// what are extremely simple rules.
func BenchmarkParseModuleRulesBase(b *testing.B) {
	sizes := []int{1, 10, 100, 1000}
	for _, size := range sizes {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			mod := generateModule(size)
			runParseModuleBenchmark(b, mod)
		})
	}
}

// BenchmarkParseStatementBasic gives a baseline for parsing a simple
// statement with a single call and two variables
func BenchmarkParseStatementBasicCall(b *testing.B) {
	runParseStatementBenchmark(b, `a+b`)
}

func BenchmarkParseStatementMixedJSON(b *testing.B) {
	// While nothing in OPA is Kubernetes specific, the webhook admission
	// request payload makes for an interesting parse test being a moderately
	// deep nested object with several different types of values.
	stmt := `{"uid":"d8fdc6db-44e1-11e9-a10f-021ca99d149a","kind":{"group":"apps","version":"v1beta1","kind":"Deployment"},"resource":{"group":"apps","version":"v1beta1","resource":"deployments"},"namespace":"opa-test","operation":"CREATE","userInfo":{"username":"user@acme.com","groups":["system:authenticated"]},"object":{"metadata":{"name":"nginx","namespace":"torin-opa-test","uid":"d8fdc047-44e1-11e9-a10f-021ca99d149a","generation":1,"creationTimestamp":"2019-03-12T16:14:01Z","labels":{"run":"nginx"}},"spec":{"replicas":1,"selector":{"matchLabels":{"run":"nginx"}},"template":{"metadata":{"creationTimestamp":null,"labels":{"run":"nginx"}},"spec":{"containers":[{"name":"nginx","image":"nginx","resources":{},"terminationMessagePath":"/dev/termination-log","terminationMessagePolicy":"File","imagePullPolicy":"Always"}],"restartPolicy":"Always","terminationGracePeriodSeconds":30,"dnsPolicy":"ClusterFirst","securityContext":{},"schedulerName":"default-scheduler"}},"strategy":{"type":"RollingUpdate","rollingUpdate":{"maxUnavailable":"25%","maxSurge":"25%"}},"revisionHistoryLimit":2,"progressDeadlineSeconds":600},"status":{}},"oldObject":null}`
	runParseStatementBenchmark(b, stmt)
}

// BenchmarkParseStatementSimpleArray gives a baseline for parsing arrays of strings.
// There is no nesting, so all test cases are flat array structures.
func BenchmarkParseStatementSimpleArray(b *testing.B) {
	sizes := []int{1, 10, 100, 1000}
	for _, size := range sizes {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			stmt := generateArrayStatement(size)
			runParseStatementBenchmark(b, stmt)
		})
	}
}

// BenchmarkParseStatementNestedObjects gives a baseline for parsing objects.
// This includes "flat" ones and more deeply nested varieties.
func BenchmarkParseStatementNestedObjects(b *testing.B) {
	sizes := [][]int{{1, 1}, {5, 1}, {10, 1}, {1, 5}, {1, 10}, {5, 5}} // Note: 10x10 will essentially hang while parsing
	for _, size := range sizes {
		b.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(b *testing.B) {
			stmt := generateObjectStatement(size[0], size[1])
			runParseStatementBenchmark(b, stmt)
		})
	}
}

func BenchmarkParseBasicABACModule(b *testing.B) {
	mod := `
	package app.abac

	default allow = false
	
	allow {
		user_is_owner
	}
	
	allow {
		user_is_employee
		action_is_read
	}
	
	allow {
		user_is_employee
		user_is_senior
		action_is_update
	}
	
	allow {
		user_is_customer
		action_is_read
		not pet_is_adopted
	}
	
	user_is_owner {
		data.user_attributes[input.user].title == "owner"
	}
	
	user_is_employee {
		data.user_attributes[input.user].title == "employee"
	}
	
	user_is_customer {
		data.user_attributes[input.user].title == "customer"
	}
	
	user_is_senior {
		data.user_attributes[input.user].tenure > 8
	}
	
	action_is_read {
		input.action == "read"
	}
	
	action_is_update {
		input.action == "update"
	}
	
	pet_is_adopted {
		data.pet_attributes[input.resource].adopted == true
	}
	`
	runParseModuleBenchmark(b, mod)
}

func runParseModuleBenchmark(b *testing.B, mod string) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseModule("", mod)
		if err != nil {
			b.Fatalf("Unexpected error: %s", err)
		}
	}
}

func runParseStatementBenchmark(b *testing.B, stmt string) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseStatement(stmt)
		if err != nil {
			b.Fatalf("Unexpected error: %s", err)
		}
	}
}

func generateModule(numRules int) string {
	mod := "package bench\n"
	for i := 0; i < numRules; i++ {
		mod += fmt.Sprintf("p%d { input.x%d = %d }\n", i, i, i)
	}
	return mod
}

func generateArrayStatement(size int) string {
	a := make([]string, size)
	for i := 0; i < size; i++ {
		a[i] = fmt.Sprintf("entry-%d", i)
	}
	return string(util.MustMarshalJSON(a))
}

func generateObjectStatement(width, depth int) string {
	o := generateObject(width, depth)
	return string(util.MustMarshalJSON(o))
}

func generateObject(width, depth int) map[string]interface{} {
	o := map[string]interface{}{}
	for i := 0; i < width; i++ {
		key := fmt.Sprintf("entry-%d", i)
		if depth <= 1 {
			o[key] = "value"
		} else {
			o[key] = generateObject(width, depth-1)
		}
	}
	return o
}
