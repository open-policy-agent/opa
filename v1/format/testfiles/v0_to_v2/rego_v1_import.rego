package test

import rego.v1

p contains x if {
	some x in input.xs
}
