package test

import rego.v2

p if {
	input.a and input.b
}

q if not { input.a; input.b }

r if input.a or input.b

# Keyword ref terms written bracketed keep their brackets.
s if {
	x := input["and"]
	y := input.or
	x or y
}
