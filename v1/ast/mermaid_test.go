// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMermaidGraph(t *testing.T) {
	tests := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "package, import, default rule, rule and function",
			module: `package example
				import rego.v1
				
				default allow := false
				
				allow if {
					input.user == "admin"
				}
				
				greet(name) := msg if {
					msg := concat(" ", ["hello", name])
				}
			`,
			exp: `flowchart TD
  n1["Module"]
  n2["Package: data.example"]
  n1 -->|package| n2
  n3["Import: rego.v1"]
  n1 -->|import| n3
  n4["Rule: default allow"]
  n5["Head"]
  n6("ref: allow")
  n5 -->|ref| n6
  n7("false")
  n5 -->|value| n7
  n4 --> n5
  n8["Body"]
  n4 --> n8
  n9{{"true"}}
  n10("true")
  n9 --> n10
  n8 -->|0| n9
  n1 -->|rule| n4
  n11["Rule: allow"]
  n12["Head"]
  n13("ref: allow")
  n12 -->|ref| n13
  n14("true")
  n12 -->|value| n14
  n11 --> n12
  n15["Body"]
  n11 --> n15
  n16{{"equal(input.user, #quot;admin#quot;)"}}
  n17("ref: equal")
  n16 -->|op| n17
  n18("ref: input.user")
  n16 -->|"arg[0]"| n18
  n19("#quot;admin#quot;")
  n16 -->|"arg[1]"| n19
  n15 -->|0| n16
  n1 -->|rule| n11
  n20["Rule: greet"]
  n21["Head"]
  n22("ref: greet")
  n21 -->|ref| n22
  n23("name")
  n21 -->|"arg[0]"| n23
  n24("msg")
  n21 -->|value| n24
  n20 --> n21
  n25["Body"]
  n20 --> n25
  n26{{"assign(msg, concat(#quot; #quot;, [#quot;hello#quot;, name]))"}}
  n27("ref: assign")
  n26 -->|op| n27
  n28("msg")
  n26 -->|"arg[0]"| n28
  n29[/"call: concat"/]
  n30("#quot; #quot;")
  n29 -->|"arg[0]"| n30
  n31[("array[2]")]
  n32("#quot;hello#quot;")
  n31 --> n32
  n33("name")
  n31 --> n33
  n29 -->|"arg[1]"| n31
  n26 -->|"arg[1]"| n29
  n25 -->|0| n26
  n1 -->|rule| n20
`,
		},
		{
			note: "and/or expressions, with operand bodies as Lhs/Rhs subtrees",
			module: `package example
				import future.keywords.and
				import future.keywords.or
				
				allow if {
					input.user == "alice" or (input.role == "admin" and input.verified)
				}
			`,
			exp: `flowchart TD
  n1["Module"]
  n2["Package: data.example"]
  n1 -->|package| n2
  n3["Import: future.keywords.and"]
  n1 -->|import| n3
  n4["Import: future.keywords.or"]
  n1 -->|import| n4
  n5["Rule: allow"]
  n6["Head"]
  n7("ref: allow")
  n6 -->|ref| n7
  n8("true")
  n6 -->|value| n8
  n5 --> n6
  n9["Body"]
  n5 --> n9
  n10{{"equal(input.user, #quot;alice#quot;) or equal(input.role, #quot;admin#quot;) and input.verified"}}
  n11["or"]
  n12["Lhs"]
  n11 --> n12
  n13{{"equal(input.user, #quot;alice#quot;)"}}
  n14("ref: equal")
  n13 -->|op| n14
  n15("ref: input.user")
  n13 -->|"arg[0]"| n15
  n16("#quot;alice#quot;")
  n13 -->|"arg[1]"| n16
  n12 --> n13
  n17["Rhs"]
  n11 --> n17
  n18{{"equal(input.role, #quot;admin#quot;) and input.verified"}}
  n19["and"]
  n20["Lhs"]
  n19 --> n20
  n21{{"equal(input.role, #quot;admin#quot;)"}}
  n22("ref: equal")
  n21 -->|op| n22
  n23("ref: input.role")
  n21 -->|"arg[0]"| n23
  n24("#quot;admin#quot;")
  n21 -->|"arg[1]"| n24
  n20 --> n21
  n25["Rhs"]
  n19 --> n25
  n26{{"input.verified"}}
  n27("ref: input.verified")
  n26 --> n27
  n25 --> n26
  n18 --> n19
  n17 --> n18
  n10 --> n11
  n9 -->|0| n10
  n1 -->|rule| n5
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			module, err := ParseModuleWithOpts("test.rego", tc.module, ParserOptions{})
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tc.exp, mermaidGraph(module)); diff != "" {
				t.Errorf("unexpected graph (-want, +got):\n%s", diff)
			}
		})
	}
}
