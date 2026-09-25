package test

import future.keywords.and
import future.keywords.in
import rego.v1
import rego.v2

p if {
	some x in input.xs
	x and input.b
}
