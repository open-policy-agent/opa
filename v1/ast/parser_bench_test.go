// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/util"
)

// BenchmarkParseModuleRulesBase gives a baseline for parsing modules with
// what are extremely simple rules.
func BenchmarkParseModuleRulesBase(b *testing.B) {
	sizes := []int{1, 10, 100, 1000}
	for _, size := range sizes {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			mod := generateModule(size)
			runParseModuleBenchmark(b, mod)
		})
	}
}

// BenchmarkParseStatementBasic gives a baseline for parsing a simple
// statement with a single call and two variables
func BenchmarkParseStatementBasicCall(b *testing.B) {
	runParseStatementBenchmark(b, `a + b`)
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
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			stmt := generateArrayStatement(size)
			runParseStatementBenchmark(b, stmt)
		})
	}
}

func TestParseStatementSimpleArray(b *testing.T) {
	sizes := []int{10} // , 10, 100, 1000}
	for _, size := range sizes {
		b.Run(strconv.Itoa(size), func(b *testing.T) {
			stmt := generateArrayStatement(size)
			_, err := ParseStatement(stmt)
			if err != nil {
				b.Fatalf("Unexpected error: %s", err)
			}
		})
	}
}

// BenchmarkParseStatementNestedObjects gives a baseline for parsing objects.
// This includes "flat" ones and more deeply nested varieties.
func BenchmarkParseStatementNestedObjects(b *testing.B) {
	sizes := [][]int{{1, 1}, {5, 1}, {10, 1}, {1, 5}, {1, 10}, {5, 5}} // Note: 10x10 will essentially hang while parsing
	for _, size := range sizes {
		b.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(b *testing.B) {
			stmt := string(util.MustMarshalJSON(generateObject(size[0], size[1])))
			runParseStatementBenchmark(b, stmt)
		})
	}
}

func BenchmarkParseSome(b *testing.B) {
	for b.Loop() {
		_ = MustParseStatement("some foo")
	}
}

func BenchmarkParseEvery(b *testing.B) {
	for b.Loop() {
		_ = MustParseStatement("every x, y in input { x == y}")
	}
}

// BenchmarkParseDeepNesting tests the impact of recursion depth tracking
// on parsing performance with deeply nested structures (arrays and objects).
// Different depths are used to measure the overhead at various nesting levels.
func BenchmarkParseDeepNesting(b *testing.B) {
	depths := []int{10, 50, 100, 500, 2500, 12500}

	b.Run("NestedArrays", func(b *testing.B) {
		for _, depth := range depths {
			b.Run(fmt.Sprintf("depth-%d", depth), func(b *testing.B) {
				stmt := generateDeeplyNestedArray(depth)
				runParseStatementBenchmark(b, stmt)
			})
		}
	})

	b.Run("NestedObjects", func(b *testing.B) {
		for _, depth := range depths {
			b.Run(fmt.Sprintf("depth-%d", depth), func(b *testing.B) {
				stmt := generateDeeplyNestedObject(depth)
				runParseStatementBenchmark(b, stmt)
			})
		}
	})
}

func BenchmarkParseStatementNestedObjectsOrSets(b *testing.B) {
	sizes := []int{1, 5, 10, 15, 20}
	for _, size := range sizes {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			stmt := generateObjectOrSetStatement(size)
			for b.Loop() {
				if _, err := ParseStatement(stmt); err == nil {
					b.Fatalf("Expected error: %s", err)
				}
			}
		})
	}
}

// 7391 ns/op	    8136 B/op	      68 allocs/op // default, before
// 7085 ns/op	    7117 B/op	      58 allocs/op // default, now
// 1990 ns/op	    5129 B/op	      53 allocs/op // with caps, notice the big ns/op difference!
func BenchmarkParseVars(b *testing.B) {
	b.Run("with default options", func(b *testing.B) {
		for b.Loop() {
			if _, err := ParseExpr(`data[i][_][j]`); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with capabilities provided", func(b *testing.B) {
		opts := ParserOptions{RegoVersion: RegoV1, SkipRules: true, Capabilities: CapabilitiesForThisVersion()}
		for b.Loop() {
			if _, err := ParseExprWithOpts(`data[i][_][j]`, opts); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkParseBasicABACModule(b *testing.B) {
	mod := `
	package app.abac

	default allow = false

	allow if {
		user_is_owner
	}

	allow if {
		user_is_employee
		action_is_read
	}

	allow if {
		user_is_employee
		user_is_senior
		action_is_update
	}

	allow if {
		user_is_customer
		action_is_read
		not pet_is_adopted
	}

	user_is_owner if {
		data.user_attributes[input.user].title == "owner"
	}

	user_is_employee if {
		data.user_attributes[input.user].title == "employee"
	}

	user_is_customer if {
		data.user_attributes[input.user].title == "customer"
	}

	user_is_senior if {
		data.user_attributes[input.user].tenure > 8
	}

	action_is_read if {
		input.action == "read"
	}

	action_is_update if {
		input.action == "update"
	}

	pet_is_adopted if {
		data.pet_attributes[input.resource].adopted == true
	}
	`
	runParseModuleBenchmark(b, mod)
}

func runParseModuleBenchmark(b *testing.B, mod string) {
	opts := ParserOptions{Capabilities: CapabilitiesForThisVersion()}
	for b.Loop() {
		MustParseModuleWithOpts(mod, opts)
	}
}

func runParseStatementBenchmark(b *testing.B, stmt string) {
	for b.Loop() {
		if _, err := ParseStatement(stmt); err != nil {
			b.Fatalf("Unexpected error: %s", err)
		}
	}
}

func generateModule(numRules int) string {
	mod := bytes.NewBufferString("package bench\n")
	for i := range numRules {
		is := strconv.Itoa(i)
		mod.WriteByte('p')
		mod.WriteString(is)
		mod.WriteString(" if { input.x")
		mod.WriteString(is)
		mod.WriteString(" = ")
		mod.WriteString(is)
		mod.WriteString(" }\n")
	}
	return mod.String()
}

func generateArrayStatement(size int) string {
	a := make([]string, size)
	for i := range size {
		a[i] = fmt.Sprintf("entry-%d", i)
	}
	return string(util.MustMarshalJSON(a))
}

func generateObject(width, depth int) map[string]any {
	o := map[string]any{}
	for i := range width {
		key := fmt.Sprintf("entry-%d", i)
		if depth <= 1 {
			o[key] = "value"
		} else {
			o[key] = generateObject(width, depth-1)
		}
	}
	return o
}

func generateObjectOrSetStatement(depth int) string {
	buf := make([]byte, 0, depth*11)
	for i := range depth {
		buf = append(buf, '{', 'a')
		buf = util.AppendInt(buf, i)
		buf = append(buf, ':', 'a')
		buf = util.AppendInt(buf, i)
		buf = append(buf, '|')
	}
	return util.ByteSliceToString(buf)
}

// 112270 ns/op	  191153 B/op	    5056 allocs/op
// 87285 ns/op	  179144 B/op	    4056 allocs/op // With text reused from location
func BenchmarkParseComments(b *testing.B) {
	policy := strings.Repeat("# comment\n", 1000) + "package p\n"
	for b.Loop() {
		MustParseModule(policy)
	}
}

// _7136 ns/op     5744 B/op       40 allocs/op // parsing only "package p"
// 34255 ns/op    40760 B/op      506 allocs/op // with annotations
// 33261 ns/op    38841 B/op      499 allocs/op // pre-alloc location text buffer
// 32817 ns/op    37319 B/op      487 allocs/op // use single metadataParser instance with reset
// 31932 ns/op	  35730 B/op	  457 allocs/op // cheaper comment parsing
// 26720 ns/op	  33742 B/op	  452 allocs/op // cheaper comment parsing + caps provided
func BenchmarkParseAnnotations(b *testing.B) {
	policy := `
# METADATA
# title: Example Policy
# description: Annotations are fun
# organizations:
#   - Open Policy Agent
#   - Cloud Native Computing Foundation
# related_resources:
#   - https://www.openpolicyagent.org
#   - https://www.cncf.io
# authors:
#   - Alice
#   - Bob
# custom:
#   tags:
#     - example
#     - demo
# scope: subpackages
# schemas:
#   - input: {"type": "object", "properties": {"user": {"type": "string"}}}
package p
`
	opts := ParserOptions{ProcessAnnotation: true, Capabilities: CapabilitiesForThisVersion()}
	for b.Loop() {
		MustParseModuleWithOpts(policy, opts)
	}
}
